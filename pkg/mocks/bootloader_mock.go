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

package mocks

import (
	"fmt"

	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
)

var _ v1.Bootloader = (*FakeBootloader)(nil)

type FakeBootloader struct {
	ErrorInstall                bool
	ErrorInstallConfig          bool
	ErrorDoEFIEntries           bool
	ErrorInstallEFI             bool
	ErrorInstallEFIFallback     bool
	ErrorInstallEFIElemental    bool
	ErrorSetPersistentVariables bool
	ErrorSetDefaultEntry        bool
}

func (f *FakeBootloader) Install(_, _, _ string) error {
	if f.ErrorInstall {
		return fmt.Errorf("error installing grub")
	}
	return nil
}

func (f *FakeBootloader) InstallConfig(_, _ string) error {
	if f.ErrorInstallConfig {
		return fmt.Errorf("error installing grub config")
	}
	return nil
}

func (f *FakeBootloader) InstallEFI(_, _, _, _ string) error {
	if f.ErrorInstallEFI {
		return fmt.Errorf("error installing efi binaries")
	}
	return nil
}

func (f *FakeBootloader) InstallEFIFallbackBinaries(_, _, _ string) error {
	if f.ErrorInstallEFIFallback {
		return fmt.Errorf("error installing fallback efi binaries")
	}
	return nil
}

func (f *FakeBootloader) InstallEFIElementalBinaries(_, _, _ string) error {
	if f.ErrorInstallEFIFallback {
		return fmt.Errorf("error installing elemental efi binaries")
	}
	return nil
}

func (f *FakeBootloader) DoEFIEntries(_, _ string) error {
	if f.ErrorDoEFIEntries {
		return fmt.Errorf("error setting efi entries")
	}
	return nil
}

func (f *FakeBootloader) SetPersistentVariables(_ string, _ map[string]string) error {
	if f.ErrorSetPersistentVariables {
		return fmt.Errorf("error setting persistent variables")
	}
	return nil
}

func (f *FakeBootloader) SetDefaultEntry(_, _, _ string) error {
	if f.ErrorSetDefaultEntry {
		return fmt.Errorf("error setting default entry")
	}
	return nil
}
