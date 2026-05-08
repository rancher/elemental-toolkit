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

package partitioner

import (
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

type DiskOptions func(d *Disk) error

func WithFS(fs types.FS) func(d *Disk) error {
	return func(d *Disk) error {
		d.fs = fs
		return nil
	}
}

func WithRunner(runner types.Runner) func(d *Disk) error {
	return func(d *Disk) error {
		d.runner = runner
		return nil
	}
}

func WithLogger(logger types.Logger) func(d *Disk) error {
	return func(d *Disk) error {
		d.logger = logger
		return nil
	}
}

func WithGdisk() func(d *Disk) error {
	return func(d *Disk) error {
		d.partBackend = Gdisk
		return nil
	}
}

func WithMounter(mounter types.Mounter) func(d *Disk) error {
	return func(d *Disk) error {
		d.mounter = mounter
		return nil
	}
}
