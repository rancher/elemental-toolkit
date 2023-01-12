/*
Copyright © 2022 - 2023 SUSE LLC

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
	"os/exec"
	"strings"
)

type Runner interface {
	InitCmd(string, ...string) *exec.Cmd
	Run(string, ...string) ([]byte, error)
	RunCmd(cmd *exec.Cmd) ([]byte, error)
	GetLogger() Logger
	SetLogger(logger Logger)
}

type RealRunner struct {
	Logger Logger
}

func (r RealRunner) InitCmd(command string, args ...string) *exec.Cmd {
	return exec.Command(command, args...)
}

func (r RealRunner) RunCmd(cmd *exec.Cmd) ([]byte, error) {
	return cmd.CombinedOutput()
}

func (r RealRunner) Run(command string, args ...string) ([]byte, error) {
	cmd := r.InitCmd(command, args...)
	if r.Logger != nil {
		r.Logger.Debugf("Running cmd: '%s %s'", command, strings.Join(args, " "))
	}
	return r.RunCmd(cmd)
}

func (r RealRunner) GetLogger() Logger {
	return r.Logger
}

func (r *RealRunner) SetLogger(logger Logger) {
	r.Logger = logger
}
