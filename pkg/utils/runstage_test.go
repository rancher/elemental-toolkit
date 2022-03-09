/*
Copyright © 2022 SUSE LLC

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
	"github.com/rancher-sandbox/elemental/pkg/cloudinit"
	conf "github.com/rancher-sandbox/elemental/pkg/config"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	v1mock "github.com/rancher-sandbox/elemental/tests/mocks"
	log "github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
)

func writeCmdline(s string, fs v1.FS) error {
	if err := fs.Mkdir("/proc", os.ModePerm); err != nil {
		return err
	}
	return fs.WriteFile("/proc/cmdline", []byte(s), os.ModePerm)
}

var _ = Describe("run stage", Label("RunStage", "root"), func() {
	var config *v1.RunConfig
	var runner *v1mock.FakeRunner
	var logger v1.Logger
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var mounter *v1mock.ErrorMounter
	var fs vfs.FS
	var memLog *bytes.Buffer

	var cleanup func()

	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		// Use a different config with a buffer for logger, so we can check the output
		// We also use the real fs
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		fs, cleanup, _ = vfst.NewTestFS(nil)

		config = conf.NewRunConfig(
			v1.WithFs(fs),
			v1.WithRunner(runner),
			v1.WithLogger(logger),
			v1.WithMounter(mounter),
			v1.WithSyscall(syscall),
			v1.WithClient(client),
		)

		config.CloudInitRunner = cloudinit.NewYipCloudInitRunner(config.Logger, config.Runner, fs)
	})
	AfterEach(func() { cleanup() })

	It("fails if strict mode is enabled", Label("strict"), func() {
		d, err := utils.TempDir(fs, "", "elemental")
		Expect(err).ToNot(HaveOccurred())
		_ = fs.WriteFile(fmt.Sprintf("%s/test.yaml", d), []byte("stages: [foo,bar]"), os.ModePerm)
		defer os.RemoveAll(d)
		config.Logger.SetLevel(log.DebugLevel)
		config.CloudInitPaths = d
		config.Strict = true
		fmt.Println(memLog.String())
		Expect(utils.RunStage("c3po", config)).ToNot(BeNil())
	})

	It("does not fail but prints errors by default", Label("strict"), func() {
		writeCmdline("stages.c3po[0].datasource", fs)

		config.Logger.SetLevel(log.DebugLevel)
		config.Strict = false
		out := utils.RunStage("c3po", config)
		Expect(out).To(BeNil())
		Expect(memLog.String()).To(ContainSubstring("parsing returned errors"))
	})

	It("Goes over extra paths", func() {
		d, err := utils.TempDir(fs, "", "elemental")
		Expect(err).ToNot(HaveOccurred())
		_ = fs.WriteFile(fmt.Sprintf("%s/test.yaml", d), []byte{}, os.ModePerm)
		defer os.RemoveAll(d)
		config.Logger.SetLevel(log.DebugLevel)
		config.CloudInitPaths = d
		Expect(utils.RunStage("luke", config)).To(BeNil())
		Expect(memLog.String()).To(ContainSubstring(fmt.Sprintf("Adding extra paths: %s", d)))
		Expect(memLog).To(ContainSubstring("luke"))
		Expect(memLog).To(ContainSubstring("luke.before"))
		Expect(memLog).To(ContainSubstring("luke.after"))
	})

	It("parses cmdline uri", func() {
		d, _ := utils.TempDir(fs, "", "elemental")
		_ = fs.WriteFile(fmt.Sprintf("%s/test.yaml", d), []byte{}, os.ModePerm)
		defer os.RemoveAll(d)
		writeCmdline(fmt.Sprintf("cos.setup=%s/test.yaml", d), fs)

		Expect(utils.RunStage("padme", config)).To(BeNil())
		Expect(memLog).To(ContainSubstring("padme"))
		Expect(memLog).To(ContainSubstring(fmt.Sprintf("%s/test.yaml", d)))
	})

	It("parses cmdline uri with dotnotation", func() {
		writeCmdline("stages.leia[0].commands[0]='echo beepboop'", fs)
		config.Logger.SetLevel(log.DebugLevel)
		Expect(utils.RunStage("leia", config)).To(BeNil())
		Expect(memLog).To(ContainSubstring("leia"))
		Expect(memLog).To(ContainSubstring("running command `echo beepboop`"))

		// try with a non-clean cmdline
		writeCmdline("BOOT=death-star single stages.leia[0].commands[0]='echo beepboop'", fs)
		Expect(utils.RunStage("leia", config)).To(BeNil())
		Expect(memLog).To(ContainSubstring("leia"))
		Expect(memLog).To(ContainSubstring("running command `echo beepboop`"))
		Expect(memLog.String()).ToNot(ContainSubstring("/proc/cmdline parsing returned errors while unmarshalling"))
		Expect(memLog.String()).ToNot(ContainSubstring("Some errors found but were ignored. Enable --strict mode to fail on those or --debug to see them in the log"))
	})

	It("ignores YAML errors", func() {
		config.Logger.SetLevel(log.DebugLevel)
		writeCmdline("BOOT=death-star sing1!~@$%6^&**le /varlib stag_#var<Lib stages[0]='utterly broken by breaking schema'", fs)
		Expect(utils.RunStage("leia", config)).To(BeNil())
		Expect(memLog.String()).To(ContainSubstring("/proc/cmdline parsing returned errors while unmarshalling"))
		Expect(memLog.String()).ToNot(ContainSubstring("Some errors found but were ignored. Enable --strict mode to fail on those or --debug to see them in the log"))
	})
})
