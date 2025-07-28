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

package action_test

import (
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"github.com/rancher/elemental-toolkit/pkg/action"
	"github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/features"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

var _ = Describe("Init Action", func() {
	var cfg *v1.RunConfig
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var cleanup func()
	var memLog *bytes.Buffer

	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		logger.SetLevel(logrus.DebugLevel)
		fs, cleanup, _ = vfst.NewTestFS(map[string]interface{}{})
		cfg = config.NewRunConfig(
			config.WithFs(fs),
			config.WithRunner(runner),
			config.WithLogger(logger),
		)
	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Init System", Label("init"), func() {
		var spec *v1.InitSpec
		var enabledUnits []string
		var mkinitrdCalled bool
		BeforeEach(func() {
			spec = config.NewInitSpec()
			enabledUnits = []string{}
			mkinitrdCalled = false

			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				switch cmd {
				case "systemctl":
					if args[0] == "enable" {
						enabledUnits = append(enabledUnits, args[1])
					}
					return []byte{}, nil
				case "dracut":
					mkinitrdCalled = true
					return []byte{}, nil
				default:
					return []byte{}, nil
				}
			}
		})
		It("Shows an error if /.dockerenv does not exist", func() {
			err := action.RunInit(cfg, spec)
			Expect(err).ToNot(BeNil())
			Expect(len(enabledUnits)).To(Equal(0))
		})
		It("Successfully runs init and install files", func() {
			err := fs.WriteFile("/.dockerenv", []byte{}, 0644)
			Expect(err).To(BeNil())

			err = action.RunInit(cfg, spec)
			Expect(err).To(BeNil())

			feats, err := features.Get([]string{features.FeatureElementalSetup})
			Expect(err).To(BeNil())
			Expect(len(feats)).To(Equal(1))
			Expect(len(enabledUnits)).To(Equal(len(feats[0].Units)))

			for _, unit := range enabledUnits {
				exists, err := utils.Exists(fs, fmt.Sprintf("/usr/lib/systemd/system/%v", unit))
				Expect(err).To(BeNil())
				Expect(exists).To(BeTrue())
			}

			Expect(mkinitrdCalled).To(BeTrue())
		})
	})
})
