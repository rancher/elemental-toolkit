/*
   Copyright © 2022 - 2024 SUSE LLC

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

package action_test

import (
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/action"
	"github.com/rancher/elemental-toolkit/v2/pkg/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/features"
	v2mock "github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ = Describe("Init Action", func() {
	var cfg *v2.RunConfig
	var runner *v2mock.FakeRunner
	var fs vfs.FS
	var logger v2.Logger
	var cleanup func()
	var memLog *bytes.Buffer
	var expectedNumUnits int

	BeforeEach(func() {
		runner = v2mock.NewFakeRunner()
		memLog = &bytes.Buffer{}
		logger = v2.NewBufferLogger(memLog)
		logger.SetLevel(logrus.DebugLevel)
		fs, cleanup, _ = vfst.NewTestFS(map[string]interface{}{})
		cfg = config.NewRunConfig(
			config.WithFs(fs),
			config.WithRunner(runner),
			config.WithLogger(logger),
		)

		feats, err := features.Get(features.All)
		Expect(err).To(BeNil())

		expectedNumUnits = 0
		for _, feat := range feats {
			expectedNumUnits += len(feat.Units)
		}
	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Init System", Label("init"), func() {
		var spec *v2.InitSpec
		var enabledUnits []string
		var errCmd, initrdFile string

		BeforeEach(func() {
			spec = config.NewInitSpec()
			enabledUnits = []string{}
			initrdFile = "/boot/elemental.initrd-6.4"

			// Emulate running in a dockerenv
			Expect(fs.WriteFile("/.dockerenv", []byte{}, 0644)).To(Succeed())

			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmd == errCmd {
					return []byte{}, fmt.Errorf("failed calling %s", cmd)
				}
				switch cmd {
				case "systemctl":
					if args[0] == "enable" {
						enabledUnits = append(enabledUnits, args[1])
					}
					return []byte{}, nil
				case "dracut":
					_, err := fs.Create(initrdFile)
					Expect(err).To(Succeed())
					return []byte{}, nil
				default:
					return []byte{}, nil
				}
			}

			// Create a kernel file and modules folder
			Expect(utils.MkdirAll(fs, "/lib/modules/6.4", constants.DirPerm)).To(Succeed())
			Expect(utils.MkdirAll(fs, "/boot", constants.DirPerm)).To(Succeed())
			_, err := fs.Create("/boot/vmlinuz-6.4")
			Expect(err).To(Succeed())
		})
		It("Shows an error if /.dockerenv does not exist", func() {
			Expect(fs.Remove("/.dockerenv")).To(Succeed())
			Expect(action.RunInit(cfg, spec)).ToNot(Succeed())
			Expect(len(enabledUnits)).To(Equal(0))
		})
		It("Successfully runs init and install files", func() {
			Expect(action.RunInit(cfg, spec)).To(Succeed())

			Expect(len(enabledUnits)).To(Equal(expectedNumUnits))

			for _, unit := range enabledUnits {
				exists, err := utils.Exists(fs, fmt.Sprintf("/etc/systemd/system/%v", unit))
				Expect(err).To(BeNil())
				Expect(exists).To(BeTrue())
			}

			exists, _ := utils.Exists(fs, "/boot/elemental.initrd-6.4")
			Expect(exists).To(BeTrue())

			// Check expected initrd and kernel files are created
			exists, _ = utils.Exists(fs, "/boot/vmlinuz")
			Expect(exists).To(BeTrue())
			exists, _ = utils.Exists(fs, "/boot/initrd")
			Expect(exists).To(BeTrue())
		})
		It("fails if requested feature does not exist", func() {
			spec.Features = append(spec.Features, "nonexisting")
			Expect(action.RunInit(cfg, spec)).NotTo(Succeed())
			Expect(len(enabledUnits)).To(Equal(0))
		})
		It("fails if the kernel file is not there", func() {
			Expect(fs.Remove("/boot/vmlinuz-6.4")).To(Succeed())
			Expect(action.RunInit(cfg, spec)).NotTo(Succeed())

			Expect(len(enabledUnits)).To(Equal(expectedNumUnits))

			for _, unit := range enabledUnits {
				exists, err := utils.Exists(fs, fmt.Sprintf("/etc/systemd/system/%v", unit))
				Expect(err).To(BeNil())
				Expect(exists).To(BeTrue())
			}
		})
		It("fails on dracut call", func() {
			errCmd = "dracut"
			Expect(action.RunInit(cfg, spec)).NotTo(Succeed())

			Expect(len(enabledUnits)).To(Equal(expectedNumUnits))

			for _, unit := range enabledUnits {
				exists, err := utils.Exists(fs, fmt.Sprintf("/etc/systemd/system/%v", unit))
				Expect(err).To(BeNil())
				Expect(exists).To(BeTrue())
			}
		})
		It("fails if initrd is not found", func() {
			initrdFile = "/boot/wrongInird"
			Expect(action.RunInit(cfg, spec)).NotTo(Succeed())

			Expect(len(enabledUnits)).To(Equal(expectedNumUnits))

			for _, unit := range enabledUnits {
				exists, err := utils.Exists(fs, fmt.Sprintf("/etc/systemd/system/%v", unit))
				Expect(err).To(BeNil())
				Expect(exists).To(BeTrue())
			}
		})
	})
})
