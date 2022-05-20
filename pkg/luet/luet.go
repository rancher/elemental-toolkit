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

package luet

import (
	"crypto/md5"
	"errors"
	"fmt"
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
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	"github.com/twpayne/go-vfs"
	"gopkg.in/yaml.v3"
)

type Luet struct {
	log               v1.Logger
	context           *context.Context
	auth              *dockTypes.AuthConfig
	fs                v1.FS
	plugins           []string
	VerifyImageUnpack bool
	TmpDir            string
}

type Options func(l *Luet) error

func WithPlugins(plugins ...string) func(r *Luet) error {
	return func(l *Luet) error {
		l.SetPlugins(plugins...)
		return nil
	}
}

func WithConfig(cfg *luetTypes.LuetConfig) func(r *Luet) error {
	return func(l *Luet) error {
		ctx := context.NewContext(
			context.WithConfig(cfg),
		)
		l.context = ctx
		return nil
	}
}

func WithAuth(auth *dockTypes.AuthConfig) func(r *Luet) error {
	return func(l *Luet) error {
		l.auth = auth
		return nil
	}
}

func WithLogger(log v1.Logger) func(r *Luet) error {
	return func(l *Luet) error {
		l.log = log
		return nil
	}
}

func WithFs(fs v1.FS) func(r *Luet) error {
	return func(l *Luet) error {
		l.fs = fs
		return nil
	}
}

func WithLuetTempDir(tmpDir string) func(r *Luet) error {
	return func(r *Luet) error {
		r.TmpDir = tmpDir
		return nil
	}
}

func NewLuet(opts ...Options) *Luet {

	luet := &Luet{}

	for _, o := range opts {
		err := o(luet)
		if err != nil {
			return nil
		}
	}

	if luet.log == nil {
		luet.log = v1.NewNullLogger()
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

	return luet
}

func (l *Luet) SetPlugins(plugins ...string) {
	l.plugins = plugins
}

func (l *Luet) GetPlugins() []string {
	return l.plugins
}

func (l *Luet) InitPlugins() {
	if len(l.plugins) > 0 {
		bus.Manager.Initialize(l.context, l.plugins...)
		l.log.Infof("Enabled plugins:")
		for _, p := range bus.Manager.Plugins {
			l.log.Infof("* %s (at %s)", p.Name, p.Executable)
		}
	}
}

func (l Luet) Unpack(target string, image string, local bool) error {
	l.log.Infof("Unpacking a container image: %s", image)
	l.InitPlugins()
	if local {
		l.log.Infof("Using an image from local cache")
		info, err := docker.ExtractDockerImage(l.context, image, target)
		if err != nil {
			if strings.Contains(err.Error(), "reference does not exist") {
				return errors.New("Container image does not exist locally")
			}
			return err
		}
		l.log.Infof("Size: %s", units.BytesSize(float64(info.Target.Size)))
	} else {
		l.log.Infof("Pulling an image from remote repository")
		info, err := docker.DownloadAndExtractDockerImage(l.context, image, target, l.auth, l.VerifyImageUnpack)
		if err != nil {
			return err
		}
		l.log.Infof("Pulled: %s %s", info.Target.Digest, info.Name)
		l.log.Infof("Size: %s", units.BytesSize(float64(info.Target.Size)))
	}

	return nil
}

// initLuetRepository returns a Luet repository from a given v1.Repository. It runs heuristics
// to determine the type from the URL if this is not provided:
// 1. Repo type is disk if the URL is an existing local path
// 2. Repo type is http is scheme is 'http' or 'https'
// 3. Repo type is docker if the URL is of type [<dommain>[:<port>]]/<path>
// Returns error if the type does not match any of any criteria.
func (l Luet) initLuetRepository(repo v1.Repository) (luetTypes.LuetRepository, error) {
	if repo.URI == "" {
		return luetTypes.LuetRepository{}, fmt.Errorf("Invalid repository, no URI is provided: %v", repo)
	}

	name := repo.Name
	if name != "" {
		// Compute a deterministic name from URI
		name = fmt.Sprintf("%x", md5.Sum([]byte(repo.URI)))
	}

	repoType := repo.Type
	if repoType == "" {
		if exists, _ := utils.Exists(l.fs, repo.URI); exists {
			repoType = "disk"
		} else if http, _ := utils.IsHTTPURI(repo.URI); http {
			repoType = "http"
		} else if utils.ValidContainerReference(repo.URI) {
			repoType = "docker"
		} else {
			return luetTypes.LuetRepository{}, fmt.Errorf("Invalid Luet repository URI: %s", repo.URI)
		}
	}

	if repo.ReferenceID == "" {
		repo.ReferenceID = "repository.yaml"
	}

	return luetTypes.LuetRepository{
		Name:        name,
		Priority:    repo.Priority,
		Enable:      true,
		Urls:        []string{repo.URI},
		Type:        repoType,
		ReferenceID: repo.ReferenceID,
	}, nil
}

// UnpackFromChannel unpacks/installs a package from the release channel into the target dir by leveraging the
// luet install action to install to a local dir
func (l Luet) UnpackFromChannel(target string, pkg string, repositories ...v1.Repository) error {
	var toInstall luetTypes.Packages
	l.InitPlugins()

	toInstall = append(toInstall, l.parsePackage(pkg))

	repos := l.context.Config.SystemRepositories
	if len(repositories) > 0 {
		repos = luetTypes.LuetRepositories{}
		for _, r := range repositories {
			repo, err := l.initLuetRepository(r)
			if err != nil {
				return err
			}
			repos = append(repos, repo)
		}
	}

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
		PackageRepositories:         repos,
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
	if l.TmpDir != "" {
		config.System.TmpDirBase = l.TmpDir
		config.System.PkgsCachePath = l.TmpDir
	}
	return config
}
