package partitioner

import (
	"errors"
	"fmt"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"regexp"
)

type MkfsCall struct {
	fileSystem string
	label      string
	customOpts []string
	dev        string
	runner     v1.Runner
}

func NewMkfsCall(dev string, fileSystem string, label string, runner v1.Runner, customOpts ...string) *MkfsCall {
	return &MkfsCall{dev: dev, fileSystem: fileSystem, label: label, runner: runner, customOpts: customOpts}
}

func (mkfs MkfsCall) buildOptions() ([]string, error) {
	opts := []string{}

	linuxFS, _ := regexp.MatchString("ext[2-4]|xfs", mkfs.fileSystem)
	fatFS, _ := regexp.MatchString("fat|vfat", mkfs.fileSystem)

	switch {
	case linuxFS:
		if mkfs.label != "" {
			opts = append(opts, "-L")
			opts = append(opts, mkfs.label)
		}
		if len(mkfs.customOpts) > 0 {
			opts = append(opts, mkfs.customOpts...)
		}
		opts = append(opts, mkfs.dev)
	case fatFS:
		if mkfs.label != "" {
			opts = append(opts, "-i")
			opts = append(opts, mkfs.label)
		}
		if len(mkfs.customOpts) > 0 {
			opts = append(opts, mkfs.customOpts...)
		}
		opts = append(opts, mkfs.dev)
	default:
		return []string{}, errors.New(fmt.Sprintf("Unsupported filesystem: %s", mkfs.fileSystem))
	}
	return opts, nil
}

func (mkfs MkfsCall) Apply() (string, error) {
	opts, err := mkfs.buildOptions()
	if err != nil {
		return "", err
	}
	tool := fmt.Sprintf("mkfs.%s", mkfs.fileSystem)
	out, err := mkfs.runner.Run(tool, opts...)
	return string(out), err
}
