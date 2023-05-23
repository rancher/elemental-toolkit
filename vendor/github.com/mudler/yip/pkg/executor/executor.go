//   Copyright 2020 Ettore Di Giacinto <mudler@mocaccino.org>
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package executor

import (
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/plugins"
	"github.com/sirupsen/logrus"
	"github.com/spectrocloud-labs/herd"
	"github.com/twpayne/go-vfs"

	"github.com/mudler/yip/pkg/schema"
)

// Executor an executor applies a yip config
type Executor interface {
	Apply(string, schema.YipConfig, vfs.FS, plugins.Console) error
	Run(string, vfs.FS, plugins.Console, ...string) error
	Plugins([]Plugin)
	Conditionals([]Plugin)
	Modifier(m schema.Modifier)
	Analyze(string, vfs.FS, plugins.Console, ...string)
	Graph(string, vfs.FS, plugins.Console, string) ([][]herd.GraphEntry, error)
}

type Plugin func(logger.Interface, schema.Stage, vfs.FS, plugins.Console) error

type Options func(d *DefaultExecutor) error

// WithLogger sets the logger for the cloudrunner
func WithLogger(i logger.Interface) Options {
	return func(d *DefaultExecutor) error {
		d.logger = i
		return nil
	}
}

// WithPlugins sets the plugins for the cloudrunner
func WithPlugins(p ...Plugin) Options {
	return func(d *DefaultExecutor) error {
		d.plugins = p
		return nil
	}
}

// WithConditionals sets the conditionals for the cloudrunner
func WithConditionals(p ...Plugin) Options {
	return func(d *DefaultExecutor) error {
		d.conditionals = p
		return nil
	}
}

// NewExecutor returns an executor from the stringified version of it.
func NewExecutor(opts ...Options) Executor {
	d := &DefaultExecutor{
		logger: logrus.New(),
		conditionals: []Plugin{
			plugins.NodeConditional,
			plugins.IfConditional,
		},
		plugins: []Plugin{
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
			plugins.Layout,
		},
	}

	for _, o := range opts {
		o(d)
	}
	return d
}
