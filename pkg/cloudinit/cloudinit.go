go: inconsistent vendoring in /home/frelon/src/elemental-toolkit:
	github.com/rancher/yip@v1.4.6: is explicitly required in go.mod, but not marked as explicit in vendor/modules.txt
	github.com/mudler/yip@v1.4.6: is marked as explicit in vendor/modules.txt, but not explicitly required in go.mod

	To ignore the vendor directory, use -mod=readonly or -mod=mod.
	To sync the vendor directory, run:
		go mod vendor
/*
Copyright © 2022 - 2024 SUSE LLC

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

package cloudinit

import (
	"path/filepath"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/utils"
	"github.com/rancher/yip/pkg/executor"
	"github.com/rancher/yip/pkg/plugins"
	"github.com/rancher/yip/pkg/schema"
	v1vfs "github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
	"gopkg.in/yaml.v3"

	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
)

type YipCloudInitRunner struct {
	exec    executor.Executor
	fs      vfs.FS
	v1fs    v1vfs.FS
	console plugins.Console
}

// NewYipCloudInitRunner returns a default yip cloud init executor with the Elemental plugin set.
// It accepts a logger which is used inside the runner.
func NewYipCloudInitRunner(l v1.Logger, r v1.Runner, fs vfs.FS) *YipCloudInitRunner {
	var v1fs v1vfs.FS
	var err error
	var root string

	// This is just to convert vfs instances of version v4 to equivalents of version 1
	// required because yip is stuck on v1 and the plugin API requries a v1
	v1fs = v1vfs.OSFS
	if _, ok := fs.(*vfst.TestFS); ok {
		root, err = fs.RawPath("/")
		if err != nil {
			l.Errorf("failed to set testfs to yip runner: %v. Fallback to v1vfs.OSFS filesystem.", err)
			v1fs = v1vfs.OSFS
		} else {
			v1fs = v1vfs.NewPathFS(v1fs, root)
			l.Debugf("Yip running on a TestFS based on %s", root)
		}
	} else if _, ok := fs.(*vfs.ReadOnlyFS); ok {
		v1fs = v1vfs.NewReadOnlyFS(v1vfs.OSFS)
	}

	y := &YipCloudInitRunner{
		fs: fs, console: newCloudInitConsole(l, r),
		v1fs: v1fs,
	}
	exec := executor.NewExecutor(
		executor.WithConditionals(
			plugins.NodeConditional,
			plugins.IfConditional,
		),
		executor.WithLogger(l),
		executor.WithPlugins(
			// Note, the plugin execution order depends on the order passed here
			plugins.DNS,
			plugins.Download,
			plugins.Git,
			plugins.Entities,
			plugins.EnsureDirectories,
			plugins.EnsureFiles,
			plugins.Commands,
			plugins.DeleteEntities,
			plugins.Hostname,
			plugins.Sysctl,
			plugins.User,
			plugins.SSH,
			plugins.LoadModules,
			plugins.Timesyncd,
			plugins.Systemctl,
			plugins.Environment,
			plugins.SystemdFirstboot,
			plugins.DataSources,
			y.layoutPlugin,
		),
	)
	y.exec = exec
	return y
}

func (ci YipCloudInitRunner) Run(stage string, args ...string) error {
	return ci.exec.Run(stage, ci.v1fs, ci.console, args...)
}

func (ci *YipCloudInitRunner) SetModifier(m schema.Modifier) {
	ci.exec.Modifier(m)
}

// Useful for testing purposes
func (ci *YipCloudInitRunner) SetFs(fs vfs.FS) {
	ci.fs = fs
}

func (ci *YipCloudInitRunner) CloudInitFileRender(target string, config *schema.YipConfig) error {
	out, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	err = utils.MkdirAll(ci.fs, filepath.Dir(target), constants.DirPerm)
	if err != nil {
		return err
	}
	return ci.fs.WriteFile(target, out, constants.FilePerm)
}
