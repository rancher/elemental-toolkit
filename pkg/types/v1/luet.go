/*
Copyright © 2022 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"runtime"
	"strings"

	dockTypes "github.com/docker/docker/api/types"
	"github.com/docker/go-units"
	"github.com/mudler/luet/pkg/api/core/bus"
	"github.com/mudler/luet/pkg/api/core/context"
	luetTypes "github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/database"
	"github.com/mudler/luet/pkg/helpers/docker"
	"github.com/mudler/luet/pkg/installer"
	"github.com/twpayne/go-vfs"
	"gopkg.in/yaml.v3"
)

type LuetInterface interface {
	Unpack(string, string, bool) error
	UnpackFromChannel(string, string) error
}

type Luet struct {
	log               Logger
	context           *context.Context
	auth              *dockTypes.AuthConfig
	fs                FS
	plugins           []string
	VerifyImageUnpack bool
}

type LuetOptions func(l *Luet) error

func WithLuetPlugins(plugins ...string) func(r *Luet) error {
	return func(l *Luet) error {
		if len(plugins) != 0 {
			l.plugins = plugins
		}
		return nil
	}
}

func WithLuetConfig(cfg *luetTypes.LuetConfig) func(r *Luet) error {
	return func(l *Luet) error {
		ctx := context.NewContext(
			context.WithConfig(cfg),
		)
		l.context = ctx
		return nil
	}
}

func WithLuetAuth(auth *dockTypes.AuthConfig) func(r *Luet) error {
	return func(l *Luet) error {
		l.auth = auth
		return nil
	}
}

func WithLuetLogger(log Logger) func(r *Luet) error {
	return func(l *Luet) error {
		l.log = log
		return nil
	}
}

func WithLuetFs(fs FS) func(r *Luet) error {
	return func(l *Luet) error {
		l.fs = fs
		return nil
	}
}

func NewLuet(opts ...LuetOptions) *Luet {

	luet := &Luet{}

	for _, o := range opts {
		err := o(luet)
		if err != nil {
			return nil
		}
	}

	if luet.log == nil {
		luet.log = NewNullLogger()
	}

	if luet.fs == nil {
		luet.fs = vfs.OSFS
	}

	if luet.context == nil {
		luetConfig := luet.createLuetConfig()
		luet.context = context.NewContext(context.WithConfig(luetConfig))
	}

	if luet.auth == nil {
		luet.auth = &dockTypes.AuthConfig{}
	}

	if len(luet.plugins) > 0 {
		bus.Manager.Initialize(luet.context, luet.plugins...)
		luet.log.Infof("Enabled plugins:")
		for _, p := range bus.Manager.Plugins {
			luet.log.Infof("* %s (at %s)", p.Name, p.Executable)
		}
	}

	return luet
}

func (l Luet) Unpack(target string, image string, local bool) error {
	l.log.Infof("Unpacking docker image: %s", image)
	if !local {
		info, err := docker.DownloadAndExtractDockerImage(l.context, image, target, l.auth, l.VerifyImageUnpack)
		if err != nil {
			return err
		}
		l.log.Infof("Pulled: %s %s", info.Target.Digest, info.Name)
		l.log.Infof("Size: %s", units.BytesSize(float64(info.Target.Size)))
	} else {
		info, err := docker.ExtractDockerImage(l.context, image, target)
		if err != nil {
			return err
		}
		l.log.Infof("Size: %s", units.BytesSize(float64(info.Target.Size)))
	}
	return nil
}

// UnpackFromChannel unpacks/installs a package from the release channel into the target dir by leveraging the
// luet install action to install to a local dir
func (l Luet) UnpackFromChannel(target string, pkg string) error {
	var toInstall luetTypes.Packages
	toInstall = append(toInstall, l.parsePackage(pkg))
	l.log.Debugf("Luet config: %+v", l.context.Config)

	inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
		Concurrency:                 l.context.Config.General.Concurrency,
		SolverOptions:               l.context.Config.Solver,
		NoDeps:                      false,
		Force:                       true,
		OnlyDeps:                    false,
		PreserveSystemEssentialData: true,
		DownloadOnly:                false,
		Ask:                         false,
		Relaxed:                     false,
		PackageRepositories:         l.context.Config.SystemRepositories,
		Context:                     l.context,
	})
	system := &installer.System{
		Database: database.NewInMemoryDatabase(false),
		Target:   target,
	}
	_, err := inst.SyncRepositories()
	if err != nil {
		return err
	}
	err = inst.Install(toInstall, system)
	return err
}

func (l Luet) parsePackage(p string) *luetTypes.Package {
	var cat, name string
	ver := ">=0"

	if strings.Contains(p, "@") {
		packageinfo := strings.Split(p, "@")
		ver = packageinfo[1]
		cat, name = packageData(packageinfo[0])
	} else {
		cat, name = packageData(p)
	}
	return &luetTypes.Package{Name: name, Category: cat, Version: ver, Uri: make([]string, 0)}
}

func packageData(p string) (string, string) {
	cat := ""
	name := ""
	if strings.Contains(p, "/") {
		packagedata := strings.Split(p, "/")
		cat = packagedata[0]
		name = packagedata[1]
	} else {
		name = p
	}
	return cat, name
}

func (l Luet) createLuetConfig() *luetTypes.LuetConfig {
	config := &luetTypes.LuetConfig{}

	// if there is a luet.yaml file, load the data from there
	if _, err := l.fs.Stat("/etc/luet/luet.yaml"); err == nil {
		l.log.Debugf("Loading luet config from /etc/luet/luet.yaml")
		f, err := l.fs.ReadFile("/etc/luet/luet.yaml")
		if err != nil {
			l.log.Errorf("Error reading luet.yaml file: %s", err)
		}
		err = yaml.Unmarshal(f, config)
		if err != nil {
			l.log.Errorf("Error unmarshalling luet.yaml file: %s", err)
		}
	} else {
		l.log.Debugf("Creating empty luet config")
	}

	err := config.Init()
	if err != nil {
		l.log.Debug("Error running init on luet config: %s", err)
	}
	// This is set on luet CLI to runtime.NumCPU but on here we have to manually set it
	if config.General.Concurrency == 0 {
		config.General.Concurrency = runtime.NumCPU()
	}
	return config
}
