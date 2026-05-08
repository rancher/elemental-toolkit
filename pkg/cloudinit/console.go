/*
Copyright © 2022 - 2026 SUSE LLC

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
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-multierror"

	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

// cloudInitConsole represents a yip's Console implementations using
// the elemental types.Runner interface.
type cloudInitConsole struct {
	runner types.Runner
	logger types.Logger
}

// newCloudInitConsole returns an instance of the cloudInitConsole based on the
// given types.Runner and types.Logger.
func newCloudInitConsole(l types.Logger, r types.Runner) *cloudInitConsole {
	return &cloudInitConsole{logger: l, runner: r}
}

// getRunner returns the internal runner used within this Console
func (c cloudInitConsole) getRunner() types.Runner {
	return c.runner
}

// Run runs a command using the types.Runner internal instance
func (c cloudInitConsole) Run(command string, opts ...func(cmd *exec.Cmd)) (string, error) {
	c.logger.Debugf("running command `%s`", command)
	cmd := c.runner.InitCmd("sh", "-c", command)
	for _, o := range opts {
		o(cmd)
	}
	out, err := c.runner.RunCmd(cmd)
	if err != nil {
		return string(out), fmt.Errorf("failed to run %s: %v", command, err)
	}

	return string(out), err
}

// Start runs a non blocking command using the types.Runner internal instance
func (c cloudInitConsole) Start(cmd *exec.Cmd, opts ...func(cmd *exec.Cmd)) error {
	c.logger.Debugf("running command `%s`", cmd)
	for _, o := range opts {
		o(cmd)
	}
	return cmd.Run()
}

// RunTemplate runs a sequence of non-blocking templated commands using the types.Runner internal instance
func (c cloudInitConsole) RunTemplate(st []string, template string) error {
	var errs error

	for _, svc := range st {
		out, err := c.Run(fmt.Sprintf(template, svc))
		if err != nil {
			c.logger.Error(out)
			c.logger.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}
