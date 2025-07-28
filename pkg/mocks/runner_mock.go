/*
Copyright © 2022 - 2025 SUSE LLC

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

package mocks

import (
	"fmt"
	"os/exec"
	"strings"

	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
)

type FakeRunner struct {
	cmds        [][]string
	ReturnValue []byte
	SideEffect  func(command string, args ...string) ([]byte, error)
	ReturnError error
	Logger      v1.Logger
	CmdNotFound string
}

func NewFakeRunner() *FakeRunner {
	return &FakeRunner{cmds: [][]string{}, ReturnValue: []byte{}, SideEffect: nil, ReturnError: nil}
}

func (r *FakeRunner) CommandExists(command string) bool {
	return command != r.CmdNotFound
}

func (r *FakeRunner) Run(command string, args ...string) ([]byte, error) {
	r.debug(fmt.Sprintf("Running cmd: '%s %s'", command, strings.Join(args, " ")))
	r.InitCmd(command, args...)
	out, err := r.RunCmd(nil)
	if err != nil {
		r.error(fmt.Sprintf("Error running command: %s", err.Error()))
	}
	return out, err
}

func (r *FakeRunner) RunCmd(_ *exec.Cmd) ([]byte, error) {
	if r.SideEffect != nil {
		if len(r.cmds) > 0 {
			lastCmd := len(r.cmds) - 1
			return r.SideEffect(r.cmds[lastCmd][0], r.cmds[lastCmd][1:]...)
		}
	}
	return r.ReturnValue, r.ReturnError
}

func (r *FakeRunner) InitCmd(command string, args ...string) *exec.Cmd {
	r.cmds = append(r.cmds, append([]string{command}, args...))
	return nil
}

func (r *FakeRunner) ClearCmds() {
	r.cmds = [][]string{}
}

// CmdsMatch matches the commands list in order. Note HasPrefix is being used to evaluate the
// match, so expecting initial part of the command is enough to get a match.
// It facilitates testing commands with dynamic arguments (aka temporary files)
func (r FakeRunner) CmdsMatch(cmdList [][]string) error {
	if len(cmdList) != len(r.cmds) {
		return fmt.Errorf("number of calls mismatch, expected %d calls but got %d", len(cmdList), len(r.cmds))
	}
	for i, cmd := range cmdList {
		expect := strings.Join(cmd[:], " ")
		got := strings.Join(r.cmds[i][:], " ")
		if !strings.HasPrefix(got, expect) {
			return fmt.Errorf("Expected command: '%s.*' got: '%s'", expect, got)
		}
	}
	return nil
}

// IncludesCmds checks the given commands were executed in any order.
// Note it uses HasPrefix to match commands, see CmdsMatch.
func (r FakeRunner) IncludesCmds(cmdList [][]string) error {
	for _, cmd := range cmdList {
		expect := strings.Join(cmd[:], " ")
		found := false
		for _, rcmd := range r.cmds {
			got := strings.Join(rcmd[:], " ")
			if strings.HasPrefix(got, expect) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("command '%s.*' not found", expect)
		}
	}
	return nil
}

// MatchMilestones matches all the given commands were executed in the provided
// order. Note it uses HasPrefix to match commands, see CmdsMatch.
func (r FakeRunner) MatchMilestones(cmdList [][]string) error {
	var match string
	for _, cmd := range r.cmds {
		if len(cmdList) == 0 {
			break
		}
		got := strings.Join(cmd[:], " ")
		match = strings.Join(cmdList[0][:], " ")
		if !strings.HasPrefix(got, match) {
			continue
		}

		cmdList = cmdList[1:]
	}

	if len(cmdList) > 0 {
		return fmt.Errorf("command '%s' not executed", match)
	}

	return nil
}

// GetCmds returns the list of commands recorded by this FakeRunner instance
// this is helpful to debug tests
func (r FakeRunner) GetCmds() [][]string {
	return r.cmds
}

func (r FakeRunner) GetLogger() v1.Logger {
	return r.Logger
}

func (r *FakeRunner) SetLogger(logger v1.Logger) {
	r.Logger = logger
}

func (r FakeRunner) error(msg string) {
	if r.Logger != nil {
		r.Logger.Error(msg)
	}
}

func (r FakeRunner) debug(msg string) {
	if r.Logger != nil {
		r.Logger.Debug(msg)
	}
}
