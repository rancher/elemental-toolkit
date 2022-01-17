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

package action_test

import (
	"errors"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental-cli/pkg/action"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"github.com/spf13/afero"
	"os"
	"testing"
)

const printOutput = `BYT;
/dev/loop0:50593792s:loopback:512:512:gpt:Loopback device:;`
const partTmpl = `
%d:%ss:%ss:2048s:ext4::type=83;`

func TestElementalSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	//config.DefaultReporterConfig.SlowSpecThreshold = 10
	RunSpecs(t, "Actions test suite")
}

var _ = Describe("Actions", func() {
	var config *v1.RunConfig
	var runner *v1mock.TestRunnerV2
	var fs afero.Fs
	var logger v1.Logger
	var mounter *v1mock.ErrorMounter
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHttpClient
	var cloudInit *v1mock.FakeCloudInitRunner

	BeforeEach(func() {
		runner = v1mock.NewTestRunnerV2()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHttpClient{}
		logger = v1.NewNullLogger()
		fs = afero.NewMemMapFs()
		cloudInit = &v1mock.FakeCloudInitRunner{}
		config = v1.NewRunConfig(
			v1.WithFs(fs),
			v1.WithRunner(runner),
			v1.WithLogger(logger),
			v1.WithMounter(mounter),
			v1.WithSyscall(syscall),
			v1.WithClient(client),
			v1.WithCloudInitRunner(cloudInit),
		)
	})
	Context("Install Action", func() {
		var install *action.InstallAction
		var device, activeTree, activeMount, cmdFail string
		var activeSize uint
		var err error

		BeforeEach(func() {
			install = action.NewInstallAction(config)
			activeTree, err = os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			activeMount, err = os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			activeSize = 16
			device = "/disk/device"
			fs.Create(device)

			partNum := 0
			partedOut := printOutput
			cmdFail = ""
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmdFail == cmd {
					return []byte{}, errors.New(fmt.Sprintf("failed on %s", cmd))
				}
				switch cmd {
				case "parted":
					idx := 0
					for i, arg := range args {
						if arg == "mkpart" {
							idx = i
							break
						}
					}
					if idx > 0 {
						partNum++
						partedOut += fmt.Sprintf(partTmpl, partNum, args[idx+3], args[idx+4])
					}
					return []byte(partedOut), nil
				case "lsblk", "blkid":
					return []byte(fmt.Sprintf("/some/device%d part", partNum)), nil
				default:
					return []byte{}, nil
				}
			}

			config.DigestSetup()
		})

		AfterEach(func() {
			os.RemoveAll(activeTree)
			os.RemoveAll(activeMount)
		})

		It("Successfully installs", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			Expect(install.Run()).To(BeNil())
		})

		It("Successfully installs despite hooks failure", func() {
			cloudInit.Error = true
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			Expect(install.Run()).To(BeNil())
		})

		It("Successfully installs from ISO", func() {
			fs.Create("cOS.iso")
			config.Iso = "cOS.iso"
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			Expect(install.Run()).To(BeNil())
		})

		It("Successfully installs without formatting despite detecting a previous installation", func() {
			config.NoFormat = true
			config.Force = true
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			Expect(install.Run()).To(BeNil())
		})

		It("Successfully installs a docker image", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			config.DockerImg = "my/image:latest"
			luet := v1mock.NewFakeLuet()
			config.Luet = luet
			Expect(install.Run()).To(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})

		It("Successfully installs and adds remote cloud-config", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			config.CloudInit = "http://my.config.org"
			Expect(install.Run()).To(BeNil())
			Expect(client.WasGetCalledWith("http://my.config.org")).To(BeTrue())
		})

		It("Fails if disk doesn't exist", func() {
			config.Target = "nonexistingdisk"
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails if some hook fails and strict is set", func() {
			config.Target = device
			config.Strict = true
			cloudInit.Error = true
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails to install from ISO if the ISO is not found", func() {
			config.Iso = "nonexistingiso"
			config.Target = device
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails to install without formatting if a previous install is detected", func() {
			config.NoFormat = true
			config.Force = false
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails to mount partitions", func() {
			config.Target = device
			mounter.ErrorOnMount = true
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails on parted errors", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			cmdFail = "parted"
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails to unmount partitions", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			mounter.ErrorOnUnmount = true
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails to create a filesystem image", func() {
			config.Target = device
			config.Fs = afero.NewReadOnlyFs(fs)
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails if luet fails to unpack image", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			config.DockerImg = "my/image:latest"
			luet := v1mock.NewFakeLuet()
			luet.OnUnpackError = true
			config.Luet = luet
			Expect(install.Run()).NotTo(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})

		It("Fails if requested remote clound config can't be downloaded", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			config.CloudInit = "http://my.config.org"
			client.Error = true
			Expect(install.Run()).NotTo(BeNil())
			Expect(client.WasGetCalledWith("http://my.config.org")).To(BeTrue())
		})

		It("Fails on grub2-install errors", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			cmdFail = "grub2-install"
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails copying Passive image", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			cmdFail = "tune2fs"
			Expect(install.Run()).NotTo(BeNil())
		})

		It("Fails setting the grub default entry", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.RootTree = activeTree
			config.ActiveImage.MountPoint = activeMount
			cmdFail = "grub2-editenv"
			Expect(install.Run()).NotTo(BeNil())
		})
	})
})
