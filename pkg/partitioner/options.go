package partitioner

import (
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/afero"
)

type DiskOptions func(d *Disk) error

func WithFS(fs afero.Fs) func(d *Disk) error {
	return func(d *Disk) error {
		d.fs = fs
		return nil
	}
}

func WithRunner(runner v1.Runner) func(d *Disk) error {
	return func(d *Disk) error {
		d.runner = runner
		return nil
	}
}

func WithLogger(logger v1.Logger) func(d *Disk) error {
	return func(d *Disk) error {
		d.logger = logger
		return nil
	}
}
