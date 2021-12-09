package mocks

import (
	"os"
	"os/exec"
)

type TestRunner struct{}

func (r *TestRunner) Run(command string, args ...string) ([]byte, error) {
	var cs []string
	// If the command is trying to get the cmdline call the TestHelperBootedFrom test
	// Maybe a switch statement would be better here??
	if command == "cat" && len(args) > 0 && args[0] == "/proc/cmdline" {
		cs = []string{"-test.run=TestHelperBootedFrom", "--"}
		cs = append(cs, args...)
	} else {
		return make([]byte, 0), nil
	}
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	out, err := cmd.CombinedOutput()
	return out, err
}
