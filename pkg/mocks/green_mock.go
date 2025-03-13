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
)

type LiveBootLoaderMock struct {
	ErrorEFI bool
	ErrorISO bool
}

func (g *LiveBootLoaderMock) PrepareEFI(_, _ string) error {
	if g.ErrorEFI {
		return fmt.Errorf("failed preparing EFI binaries")
	}
	return nil
}

func (g *LiveBootLoaderMock) PrepareISO(_, _ string) error {
	if g.ErrorISO {
		return fmt.Errorf("failed preparing ISO bootloader binaries")
	}
	return nil
}
