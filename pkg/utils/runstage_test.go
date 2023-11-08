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

package utils_test

import (
	"bytes"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"github.com/rancher/elemental-toolkit/pkg/cloudinit"
	conf "github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const testingStages = `
stages:
  luke:
  - commands:
    - echo "I have a very bad feeling about this"
`

func writeCmdline(s string, fs v1.FS) error {
	if err := fs.Mkdir("/proc", os.ModePerm); err != nil {
		return err
	}
	return fs.WriteFile("/proc/cmdline", []byte(s), os.ModePerm)
}

var _ = Describe("run stage", Label("RunStage"), func() {
	var config *v1.Config
	var runner *v1mock.FakeRunner
	var logger v1.Logger
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var mounter *v1mock.FakeMounter
	var fs vfs.FS
	var memLog *bytes.Buffer

	var cleanup func()
	var strict bool

	BeforeEach(func() {
		strict = false
		runner = v1mock.NewFakeRunner()
		// Use a different config with a buffer for logger, so we can check the output
		// We also use the real fs
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		fs, cleanup, _ = vfst.NewTestFS(nil)

		config = conf.NewConfig(
			conf.WithFs(fs),
			conf.WithRunner(runner),
			conf.WithLogger(logger),
			conf.WithMounter(mounter),
			conf.WithSyscall(syscall),
			conf.WithClient(client),
		)

		config.CloudInitRunner = cloudinit.NewYipCloudInitRunner(config.Logger, config.Runner, fs)
	})
	AfterEach(func() { cleanup() })

	It("fails if strict mode is enabled", Label("strict"), func() {
		d, err := utils.TempDir(fs, "", "elemental")
		Expect(err).ToNot(HaveOccurred())
		_ = fs.WriteFile(fmt.Sprintf("%s/test.yaml", d), []byte("stages: [foo,bar]"), os.ModePerm)
		strict = true
		Expect(utils.RunStage(config, "c3po", strict, d)).ToNot(BeNil())
	})

	It("does not fail but prints errors by default", Label("strict"), func() {
		writeCmdline("stages.c3po[0].datasource", fs)

		config.Logger.SetLevel(log.DebugLevel)
		out := utils.RunStage(config, "c3po", strict)
		Expect(out).To(BeNil())
		Expect(memLog.String()).To(ContainSubstring("parsing returned errors"))
	})

	It("Goes over extra paths", func() {
		d, err := utils.TempDir(fs, "", "elemental")
		Expect(err).ToNot(HaveOccurred())
		err = fs.WriteFile(fmt.Sprintf("%s/extra.yaml", d), []byte(testingStages), os.ModePerm)
		Expect(err).ShouldNot(HaveOccurred())
		config.Logger.SetLevel(log.DebugLevel)

		Expect(utils.RunStage(config, "luke", strict, d)).To(BeNil())
		Expect(memLog.String()).To(ContainSubstring(fmt.Sprintf("Reading '%s/extra.yaml'", d)))
		Expect(memLog).To(ContainSubstring("luke"))
		Expect(memLog).To(ContainSubstring("luke.before"))
		Expect(memLog).To(ContainSubstring("luke.after"))
	})

	It("parses cmdline uri", func() {
		d, _ := utils.TempDir(fs, "", "elemental")
		_ = fs.WriteFile(fmt.Sprintf("%s/test.yaml", d), []byte(testingStages), os.ModePerm)

		writeCmdline(fmt.Sprintf("cos.setup=%s/test.yaml", d), fs)
		config.Logger.SetLevel(log.DebugLevel)

		Expect(utils.RunStage(config, "luke", strict)).To(BeNil())
		Expect(memLog).To(ContainSubstring("luke"))
		Expect(memLog).To(ContainSubstring(fmt.Sprintf("%s/test.yaml", d)))
	})

	It("parses cmdline uri with dotnotation", func() {
		writeCmdline("stages.leia[0].commands[0]='echo beepboop'", fs)
		config.Logger.SetLevel(log.DebugLevel)
		Expect(utils.RunStage(config, "leia", strict)).To(BeNil())
		Expect(memLog).To(ContainSubstring("leia"))
		Expect(memLog).To(ContainSubstring("running command `echo beepboop`"))

		// try with a non-clean cmdline
		writeCmdline("BOOT=death-star single stages.leia[0].commands[0]='echo beepboop'", fs)
		Expect(utils.RunStage(config, "leia", strict)).To(BeNil())
		Expect(memLog).To(ContainSubstring("leia"))
		Expect(memLog).To(ContainSubstring("running command `echo beepboop`"))
		Expect(memLog.String()).ToNot(ContainSubstring("/proc/cmdline parsing returned errors while unmarshalling"))
		Expect(memLog.String()).ToNot(ContainSubstring("Some errors found but were ignored. Enable --strict mode to fail on those or --debug to see them in the log"))
	})

	It("ignores YAML errors", func() {
		config.Logger.SetLevel(log.DebugLevel)
		writeCmdline("BOOT=death-star sing1!~@$%6^&**le /varlib stag_#var<Lib stages[0]='utterly broken by breaking schema'", fs)
		Expect(utils.RunStage(config, "leia", strict)).To(BeNil())
		Expect(memLog.String()).To(ContainSubstring("/proc/cmdline parsing returned errors while unmarshalling"))
		Expect(memLog.String()).ToNot(ContainSubstring("Some errors found but were ignored. Enable --strict mode to fail on those or --debug to see them in the log"))
	})

	It("ignores non existing cloud-init paths", func() {
		ci := &v1mock.FakeCloudInitRunner{}
		config.CloudInitRunner = ci
		Expect(utils.MkdirAll(fs, "/existing", constants.DirPerm)).To(Succeed())
		// Symlinks to existing directoryes are also valid
		Expect(fs.Symlink("/existing", "/symlinkToExistingDir")).To(Succeed())

		writeCmdline("cos.setup=/wrongpath", fs)

		Expect(utils.RunStage(
			config, "stage", strict, "/nonexisting", "/existing",
			"/symlinkToExistingDir", "https://my.domain.org/cloud-file",
		)).To(BeNil())

		Expect(ci.GetStageArgs("stage")).To(ContainElement("/existing"))
		Expect(ci.GetStageArgs("stage")).To(ContainElement("/symlinkToExistingDir"))
		Expect(ci.GetStageArgs("stage")).To(ContainElement("https://my.domain.org/cloud-file"))
		Expect(ci.GetStageArgs("stage")).NotTo(ContainElement("/nonexisting"))
		Expect(ci.GetStageArgs("stage")).NotTo(ContainElement("/wrongpath"))
	})
})
