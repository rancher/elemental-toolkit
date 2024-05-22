/*
Copyright © 2021 - 2024 SUSE LLC

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

package efi_test

import (
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	efilib "github.com/canonical/go-efilib"

	"github.com/rancher/elemental-toolkit/v2/pkg/efi"
	"github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

var _ = Describe("EFI Manager", Label("efi", "manager"), func() {
	var memLog *bytes.Buffer
	var logger types.Logger

	BeforeEach(func() {
		memLog = &bytes.Buffer{}
		logger = types.NewBufferLogger(memLog)
		logger.SetLevel(logrus.DebugLevel)
	})

	It("creates a BootManager without error", func() {
		var vars efi.Variables = mocks.NewMockEFIVariables()

		manager, err := efi.NewBootManagerForVariables(logger, vars)
		Expect(err).To(BeNil())

		Expect(manager).ToNot(BeNil())
	})

	It("creates a BootManager with ReadLoadOptions error", func() {
		var vars efi.Variables = mocks.NewMockEFIVariables().WithLoadOptionError(fmt.Errorf("cannot read device path: cannot decode node: 1: invalid length 14 bytes (too large)"))

		err := vars.SetVariable(efilib.GlobalVariable, "BootOrder", []byte("0001"), efilib.AttributeNonVolatile)
		Expect(err).To(BeNil())

		err = vars.SetVariable(efilib.GlobalVariable, "Boot0001", []byte("test.efi"), efilib.AttributeNonVolatile)
		Expect(err).To(BeNil())

		manager, err := efi.NewBootManagerForVariables(logger, vars)
		Expect(err).To(BeNil())

		Expect(manager).ToNot(BeNil())
	})
})
