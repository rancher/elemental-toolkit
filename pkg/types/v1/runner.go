package v1

import "os/exec"

type Runner interface {
	Run(string, ...string) ([]byte, error)
}

type RealRunner struct{}

func (r RealRunner) Run(command string, args ...string) ([]byte, error) {
	out, err := exec.Command(command, args...).CombinedOutput()
	return out, err
}
