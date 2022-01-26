/*
Copyright © 2021 SUSE LLC

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
	"errors"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	v1mock "github.com/rancher-sandbox/elemental/tests/mocks"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"io/ioutil"
	"os"
	"testing"
)

func getNamesFromListFiles(list []os.FileInfo) []string {
	var names []string
	for _, f := range list {
		names = append(names, f.Name())
	}
	return names
}

func TestCommonSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elemental utils suite")
}

var _ = Describe("Utils", func() {
	var config *v1.RunConfig
	var runner v1.Runner
	var logger v1.Logger
	var syscall *v1mock.FakeSyscall
	var client v1.HTTPClient
	var mounter *v1mock.ErrorMounter
	var fs afero.Fs
	var memLog *bytes.Buffer

	BeforeEach(func() {
		runner = &v1mock.FakeRunner{}
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHttpClient{}
		logger = v1.NewNullLogger()
		fs = afero.NewMemMapFs()
		config = v1.NewRunConfig(
			v1.WithFs(fs),
			v1.WithRunner(runner),
			v1.WithLogger(logger),
			v1.WithMounter(mounter),
			v1.WithSyscall(syscall),
			v1.WithClient(client),
		)
	})
	Context("Chroot", func() {
		var chroot *utils.Chroot
		BeforeEach(func() {
			chroot = utils.NewChroot(
				"/whatever",
				config,
			)
		})
		Context("on success", func() {
			It("command should be called in the chroot", func() {
				_, err := chroot.Run("chroot-command")
				Expect(err).To(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
			})
			It("commands should be called with a customized chroot", func() {
				chroot.SetExtraMounts(map[string]string{"/real/path": "/in/chroot/path"})
				Expect(chroot.Prepare()).To(BeNil())
				defer chroot.Close()
				_, err := chroot.Run("chroot-command")
				Expect(err).To(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
				_, err = chroot.Run("chroot-another-command")
				Expect(err).To(BeNil())
			})
		})
		Context("on failure", func() {
			It("should return error if chroot-command fails", func() {
				runner := runner.(*v1mock.FakeRunner)
				runner.ErrorOnCommand = true
				_, err := chroot.Run("chroot-command")
				Expect(err).NotTo(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
			})
			It("should return error if preparing twice before closing", func() {
				Expect(chroot.Prepare()).To(BeNil())
				defer chroot.Close()
				Expect(chroot.Prepare()).NotTo(BeNil())
				Expect(chroot.Close()).To(BeNil())
				Expect(chroot.Prepare()).To(BeNil())
			})
			It("should return error if failed to chroot", func() {
				syscall.ErrorOnChroot = true
				_, err := chroot.Run("chroot-command")
				Expect(err).ToNot(BeNil())
				Expect(syscall.WasChrootCalledWith("/whatever")).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("chroot error"))
			})
			It("should return error if failed to mount on prepare", func() {
				mounter.ErrorOnMount = true
				_, err := chroot.Run("chroot-command")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("mount error"))
			})
			It("should return error if failed to unmount on close", func() {
				mounter.ErrorOnUnmount = true
				_, err := chroot.Run("chroot-command")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("Failed closing chroot"))
			})
		})
	})
	Context("TestBootedFrom", func() {
		It("returns true if we are booting from label FAKELABEL", func() {
			runner := v1mock.NewTestRunnerV2()
			runner.ReturnValue = []byte("")
			Expect(utils.BootedFrom(runner, "FAKELABEL")).To(BeFalse())
		})
		It("returns false if we are not booting from label FAKELABEL", func() {
			runner := v1mock.NewTestRunnerV2()
			runner.ReturnValue = []byte("FAKELABEL")
			Expect(utils.BootedFrom(runner, "FAKELABEL")).To(BeTrue())
		})
	})
	Context("GetDeviceByLabel", func() {
		var runner *v1mock.TestRunnerV2
		var cmds [][]string
		BeforeEach(func() {
			runner = v1mock.NewTestRunnerV2()
			cmds = [][]string{
				{"udevadm", "settle"},
				{"blkid", "--label", "FAKE"},
			}
		})
		It("returns found device", func() {
			runner.ReturnValue = []byte("/some/device")
			out, err := utils.GetDeviceByLabel(runner, "FAKE", 1)
			Expect(err).To(BeNil())
			Expect(out).To(Equal("/some/device"))
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("fails to run blkid", func() {
			runner.ReturnError = errors.New("failed running blkid")
			_, err := utils.GetDeviceByLabel(runner, "FAKE", 1)
			Expect(err).NotTo(BeNil())
			Expect(runner.CmdsMatch(cmds)).To(BeNil())
		})
		It("fails if no device is found in two attempts", func() {
			runner.ReturnValue = []byte("")
			_, err := utils.GetDeviceByLabel(runner, "FAKE", 2)
			Expect(err).NotTo(BeNil())
			Expect(runner.CmdsMatch(append(cmds, cmds...))).To(BeNil())
		})
	})
	Context("CopyFile", func() {
		It("Copies source to target", func() {
			fs.Create("/some/file")
			_, err := fs.Stat("/some/otherfile")
			Expect(err).NotTo(BeNil())
			Expect(utils.CopyFile(fs, "/some/file", "/some/otherfile")).To(BeNil())
			_, err = fs.Stat("/some/otherfile")
			Expect(err).To(BeNil())
		})
		It("Fails to open non existing file", func() {
			Expect(utils.CopyFile(fs, "/some/file", "/some/otherfile")).NotTo(BeNil())
			_, err := fs.Stat("/some/otherfile")
			Expect(err).NotTo(BeNil())
		})
		It("Fails to copy on non writable target", func() {
			fs.Create("/some/file")
			_, err := fs.Stat("/some/otherfile")
			Expect(err).NotTo(BeNil())
			fs = afero.NewReadOnlyFs(fs)
			Expect(utils.CopyFile(fs, "/some/file", "/some/otherfile")).NotTo(BeNil())
			_, err = fs.Stat("/some/otherfile")
			Expect(err).NotTo(BeNil())
		})
	})
	Context("CreateDirStructure", func() {
		It("Creates essential directories", func() {
			Expect(utils.CreateDirStructure(fs, "/my/root")).To(BeNil())
			for _, dir := range []string{"sys", "proc", "dev", "tmp", "boot", "usr/local", "oem"} {
				_, err := fs.Stat(fmt.Sprintf("/my/root/%s", dir))
				Expect(err).To(BeNil())
			}
		})
		It("Fails on non writable target", func() {
			fs = afero.NewReadOnlyFs(fs)
			Expect(utils.CreateDirStructure(fs, "/my/root")).NotTo(BeNil())
		})
	})
	Context("SyncData", func() {
		It("Copies all files from source to target", func() {
			sourceDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(sourceDir)
			destDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(destDir)

			for i := 0; i < 5; i++ {
				_, _ = os.CreateTemp(sourceDir, "file*")
			}

			Expect(utils.SyncData(sourceDir, destDir)).To(BeNil())

			filesDest, err := ioutil.ReadDir(destDir)
			destNames := getNamesFromListFiles(filesDest)
			filesSource, err := ioutil.ReadDir(sourceDir)
			SourceNames := getNamesFromListFiles(filesSource)

			// Should be the same files in both dirs now
			Expect(destNames).To(Equal(SourceNames))
		})
		It("should not fail if dirs are empty", func() {
			sourceDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(sourceDir)
			destDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(destDir)
			Expect(utils.SyncData(sourceDir, destDir)).To(BeNil())
		})
		It("should fail if destination does not exist", func() {
			sourceDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(sourceDir)
			Expect(utils.SyncData(sourceDir, "/welp")).NotTo(BeNil())
		})
		It("should fail if source does not exist", func() {
			destDir, err := os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			defer os.RemoveAll(destDir)
			Expect(utils.SyncData("/welp", destDir)).NotTo(BeNil())
		})
	})
	Context("Grub", func() {
		Context("Install", func() {
			BeforeEach(func() {
				config.Target = "/dev/test"
			})
			It("installs with default values", func() {
				buf := &bytes.Buffer{}
				logger := log.New()
				logger.SetOutput(buf)

				_ = fs.MkdirAll(fmt.Sprintf("%s/grub2/", constants.StateDir), 0666)
				_ = fs.MkdirAll("/etc/cos/", 0666)
				err := afero.WriteFile(fs, "/etc/cos/grub.cfg", []byte("console=tty1"), 0644)
				Expect(err).To(BeNil())

				config.Logger = logger
				config.GrubConf = "/etc/cos/grub.cfg"

				grub := utils.NewGrub(config)
				err = grub.Install()
				Expect(err).To(BeNil())

				Expect(buf).To(ContainSubstring("Installing GRUB.."))
				Expect(buf).To(ContainSubstring("Grub install to device /dev/test complete"))
				Expect(buf).ToNot(ContainSubstring("efi"))
				Expect(buf.String()).ToNot(ContainSubstring("Adding extra tty (serial) to grub.cfg"))
				targetGrub, err := afero.ReadFile(fs, fmt.Sprintf("%s/grub2/grub.cfg", constants.StateDir))
				Expect(err).To(BeNil())
				// Should not be modified at all
				Expect(targetGrub).To(ContainSubstring("console=tty1"))

			})
			It("installs with efi on efi system", func() {
				buf := &bytes.Buffer{}
				logger := log.New()
				logger.SetOutput(buf)
				logger.SetLevel(log.DebugLevel)
				_, _ = fs.Create("/etc/cos/grub.cfg")
				_, _ = fs.Create(constants.EfiDevice)

				config.Logger = logger

				grub := utils.NewGrub(config)
				err := grub.Install()
				Expect(err).To(BeNil())

				Expect(buf.String()).To(ContainSubstring("--target=x86_64-efi"))
				Expect(buf.String()).To(ContainSubstring("--efi-directory"))
				Expect(buf.String()).To(ContainSubstring("Installing grub efi for arch x86_64"))
			})
			It("installs with efi with --force-efi", func() {
				buf := &bytes.Buffer{}
				logger := log.New()
				logger.SetOutput(buf)
				logger.SetLevel(log.DebugLevel)
				_, _ = fs.Create("/etc/cos/grub.cfg")

				config.Logger = logger
				config.ForceEfi = true

				grub := utils.NewGrub(config)
				err := grub.Install()
				Expect(err).To(BeNil())

				Expect(buf.String()).To(ContainSubstring("--target=x86_64-efi"))
				Expect(buf.String()).To(ContainSubstring("--efi-directory"))
				Expect(buf.String()).To(ContainSubstring("Installing grub efi for arch x86_64"))
			})
			It("installs with extra tty", func() {
				buf := &bytes.Buffer{}
				logger := log.New()
				logger.SetOutput(buf)
				_ = fs.MkdirAll(fmt.Sprintf("%s/grub2/", constants.StateDir), 0666)
				_ = fs.MkdirAll("/etc/cos/", 0666)
				err := afero.WriteFile(fs, "/etc/cos/grub.cfg", []byte("console=tty1"), 0644)
				Expect(err).To(BeNil())
				_, _ = fs.Create("/dev/serial")

				config.Logger = logger
				config.Tty = "serial"
				config.GrubConf = "/etc/cos/grub.cfg"

				grub := utils.NewGrub(config)
				err = grub.Install()
				Expect(err).To(BeNil())

				Expect(buf.String()).To(ContainSubstring("Adding extra tty (serial) to grub.cfg"))
				targetGrub, err := afero.ReadFile(fs, fmt.Sprintf("%s/grub2/grub.cfg", constants.StateDir))
				Expect(err).To(BeNil())
				Expect(targetGrub).To(ContainSubstring("console=tty1 console=serial"))

			})

		})
		Context("SetPersistentVariables", func() {
			It("Sets the grub environment file", func() {
				runner := v1mock.NewTestRunnerV2()
				config.Runner = runner
				grub := utils.NewGrub(config)
				Expect(grub.SetPersistentVariables(
					"somefile", map[string]string{"key1": "value1", "key2": "value2"},
				)).To(BeNil())
				Expect(runner.CmdsMatch([][]string{
					{"grub2-editenv", "somefile", "set", "key1=value1"},
					{"grub2-editenv", "somefile", "set", "key2=value2"},
				})).To(BeNil())
			})
			It("Fails running grub2-editenv", func() {
				runner := v1mock.NewTestRunnerV2()
				runner.ReturnError = errors.New("grub error")
				config.Runner = runner
				grub := utils.NewGrub(config)
				Expect(grub.SetPersistentVariables(
					"somefile", map[string]string{"key1": "value1", "key2": "value2"},
				)).NotTo(BeNil())
				Expect(runner.CmdsMatch([][]string{
					{"grub2-editenv", "somefile", "set", "key1=value1"},
				})).To(BeNil())
			})
		})
	})
	Context("RunStage", func() {
		BeforeEach(func() {
			// Use a different config with a buffer for logger, so we can check the output
			// We also use the real fs
			memLog = &bytes.Buffer{}
			logger = v1.NewBufferLogger(memLog)
			fs = afero.NewOsFs()
			config = v1.NewRunConfig(
				v1.WithFs(fs),
				v1.WithRunner(runner),
				v1.WithLogger(logger),
				v1.WithMounter(mounter),
				v1.WithSyscall(syscall),
				v1.WithClient(client),
			)
		})
		It("fails if strict mode is enabled", func() {
			config.Logger.SetLevel(log.DebugLevel)
			config.Strict = true
			r := v1mock.NewTestRunnerV2()
			r.ReturnValue = []byte("stages.c3po[0].datasource") // this should fail as its wrong data
			config.Runner = r
			Expect(utils.RunStage("c3po", config)).ToNot(BeNil())
		})
		It("does not fail but prints errors by default", func() {
			config.Logger.SetLevel(log.DebugLevel)
			config.Strict = false
			r := v1mock.NewTestRunnerV2()
			r.ReturnValue = []byte("stages.c3po[0].datasource") // this should fail as its wrong data
			config.Runner = r
			out := utils.RunStage("c3po", config)
			Expect(out).To(BeNil())
			Expect(memLog).To(ContainSubstring("Some errors found but were ignored"))
		})
		It("Goes over extra paths", func() {
			d, _ := afero.TempDir(fs, "", "elemental")
			_ = afero.WriteFile(fs, fmt.Sprintf("%s/test.yaml", d), []byte{}, os.ModePerm)
			defer os.RemoveAll(d)
			config.Logger.SetLevel(log.DebugLevel)
			config.CloudInitPaths = d
			Expect(utils.RunStage("luke", config)).To(BeNil())
			Expect(memLog).To(ContainSubstring(fmt.Sprintf("Adding extra paths: %s", d)))
			Expect(memLog).To(ContainSubstring("luke"))
			Expect(memLog).To(ContainSubstring("luke.before"))
			Expect(memLog).To(ContainSubstring("luke.after"))
		})
		It("parses cmdline uri", func() {
			d, _ := afero.TempDir(fs, "", "elemental")
			_ = afero.WriteFile(fs, fmt.Sprintf("%s/test.yaml", d), []byte{}, os.ModePerm)
			defer os.RemoveAll(d)

			r := v1mock.NewTestRunnerV2()
			r.ReturnValue = []byte(fmt.Sprintf("cos.setup=%s/test.yaml", d))
			config.Runner = r
			Expect(utils.RunStage("padme", config)).To(BeNil())
			Expect(memLog).To(ContainSubstring("padme"))
			Expect(memLog).To(ContainSubstring(fmt.Sprintf("%s/test.yaml", d)))
		})
		It("parses cmdline uri with dotnotation", func() {
			config.Logger.SetLevel(log.DebugLevel)
			r := v1mock.NewTestRunnerV2()
			r.ReturnValue = []byte("stages.leia[0].commands[0]='echo beepboop'")
			config.Runner = r
			Expect(utils.RunStage("leia", config)).To(BeNil())
			Expect(memLog).To(ContainSubstring("leia"))
			Expect(memLog).To(ContainSubstring("running command `echo beepboop`"))
			Expect(memLog).To(ContainSubstring("Command output: beepboop"))

			r = v1mock.NewTestRunnerV2()
			// try with a non-clean cmdline
			r.ReturnValue = []byte("BOOT=death-star single stages.leia[0].commands[0]='echo beepboop'")
			config.Runner = r
			Expect(utils.RunStage("leia", config)).To(BeNil())
			Expect(memLog).To(ContainSubstring("leia"))
			Expect(memLog).To(ContainSubstring("running command `echo beepboop`"))
			Expect(memLog).To(ContainSubstring("Command output: beepboop"))
		})
	})
})
