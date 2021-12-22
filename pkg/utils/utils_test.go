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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental-cli/pkg/constants"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"testing"
)

func TestCommonSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elemental utils suite")
}

var _ = Describe("Utils", func() {
	var config *v1.RunConfig
	var runner v1.Runner
	var logger v1.Logger
	var syscall v1.SyscallInterface
	var client v1.HTTPClient
	var mounter mount.Interface
	var fs afero.Fs

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
		Context("on success", func() {
			It("command should be called in the chroot", func() {
				syscallInterface := &v1mock.FakeSyscall{}
				config.Syscall = syscallInterface
				chroot := utils.NewChroot(
					"/whatever",
					config,
				)
				defer chroot.Close()
				_, err := chroot.Run("chroot-command")
				Expect(err).To(BeNil())
				Expect(syscallInterface.WasChrootCalledWith("/whatever")).To(BeTrue())
			})
		})
		Context("on failure", func() {
			It("should return error if failed to chroot", func() {
				syscallInterface := &v1mock.FakeSyscall{ErrorOnChroot: true}
				config.Syscall = syscallInterface
				chroot := utils.NewChroot(
					"/whatever",
					config,
				)
				defer chroot.Close()
				_, err := chroot.Run("chroot-command")
				Expect(err).ToNot(BeNil())
				Expect(syscallInterface.WasChrootCalledWith("/whatever")).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("chroot error"))
			})
			It("should return error if failed to mount on prepare", func() {
				mounter := v1mock.NewErrorMounter()
				mounter.ErrorOnMount = true
				config.Mounter = mounter

				chroot := utils.NewChroot(
					"/whatever",
					config,
				)
				_, err := chroot.Run("chroot-command")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("mount error"))
			})
			It("should return error if failed to unmount on close", func() {
				mounter := v1mock.NewErrorMounter()
				mounter.ErrorOnUnmount = true
				config.Mounter = mounter

				chroot := utils.NewChroot(
					"/whatever",
					config,
				)
				err := chroot.Close()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unmount error"))
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
	Context("FindLabel", func() {
		It("returns empty and no error if label does not exist", func() {
			runner := v1mock.NewTestRunnerV2()
			runner.ReturnValue = []byte("")
			out, err := utils.FindLabel(runner, "FAKE")
			Expect(err).To(BeNil())
			Expect(out).To(BeEmpty())
		})
		It("returns path and no error if label does exist", func() {
			runner := v1mock.NewTestRunnerV2()
			runner.ReturnValue = []byte("/dev/fake")
			out, err := utils.FindLabel(runner, "FAKE")
			Expect(err).To(BeNil())
			Expect(out).To(Equal("/dev/fake"))
		})
		It("returns empty and error if there is an error", func() {
			runner := v1mock.NewTestRunnerV2()
			runner.ReturnError = errors.New("something")
			out, err := utils.FindLabel(runner, "FAKE")
			Expect(err).ToNot(BeNil())
			Expect(out).To(Equal(""))
		})
	})
	Context("Grub", func() {
		Context("Install", func() {
			BeforeEach(func() {
				config.Target = "/dev/test"
				config.StateDir = "/state"
			})
			It("installs with default values", func() {
				buf := &bytes.Buffer{}
				logger := log.New()
				logger.SetOutput(buf)

				_ = fs.MkdirAll("/state/grub2/", 0666)
				_ = fs.MkdirAll("/etc/cos/", 0666)
				err := afero.WriteFile(fs, "/etc/cos/grub.cfg", []byte("console=tty1"), 0644)
				Expect(err).To(BeNil())

				config.Logger = logger
				config.Target = "/dev/test"
				config.StateDir = "/state"
				config.GrubConf = "/etc/cos/grub.cfg"

				grub := utils.NewGrub(config)
				err = grub.Install()
				Expect(err).To(BeNil())

				Expect(buf).To(ContainSubstring("Installing GRUB.."))
				Expect(buf).To(ContainSubstring("Grub install to device /dev/test complete"))
				Expect(buf).ToNot(ContainSubstring("efi"))
				Expect(buf.String()).ToNot(ContainSubstring("Adding extra tty (serial) to grub.cfg"))
				targetGrub, err := afero.ReadFile(fs, "/state/grub2/grub.cfg")
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
				_ = fs.MkdirAll("/state/grub2/", 0666)
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
				targetGrub, err := afero.ReadFile(fs, "/state/grub2/grub.cfg")
				Expect(err).To(BeNil())
				Expect(targetGrub).To(ContainSubstring("console=tty1 console=serial"))

			})
		})
	})
})
