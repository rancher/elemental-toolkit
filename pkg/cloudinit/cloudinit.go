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

	"github.com/rancher/yip/pkg/executor"
	"github.com/rancher/yip/pkg/plugins"
	"github.com/rancher/yip/pkg/schema"
	"github.com/twpayne/go-vfs/v4"
	"gopkg.in/yaml.v3"

	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"

	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

type YipCloudInitRunner struct {
	exec    executor.Executor
	fs      vfs.FS
	console plugins.Console
}

// NewYipCloudInitRunner returns a default yip cloud init executor with the Elemental plugin set.
// It accepts a logger which is used inside the runner.
func NewYipCloudInitRunner(l types.Logger, r types.Runner, fs vfs.FS) *YipCloudInitRunner {
	y := &YipCloudInitRunner{
		fs: fs, console: newCloudInitConsole(l, r),
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
			plugins.Timesyncd,
			plugins.Systemctl,
			plugins.Environment,
			plugins.SystemdFirstboot,
			plugins.DataSources,
			layoutPlugin,
		),
	)
	y.exec = exec
	return y
}

func (ci YipCloudInitRunner) Run(stage string, args ...string) error {
	return ci.exec.Run(stage, ci.fs, ci.console, args...)
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
