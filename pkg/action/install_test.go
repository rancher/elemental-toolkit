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

package action_test

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/jaypipes/ghw/pkg/block"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"github.com/rancher/elemental-toolkit/pkg/action"
	conf "github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const printOutput = `BYT;
/dev/loop0:50593792s:loopback:512:512:gpt:Loopback device:;`
const partTmpl = `
%d:%ss:%ss:2048s:ext4::type=83;`

var _ = Describe("Install action tests", func() {
	var config *v1.RunConfig
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var mounter *v1mock.FakeMounter
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var cloudInit *v1mock.FakeCloudInitRunner
	var extractor *v1mock.FakeImageExtractor
	var cleanup func()
	var memLog *bytes.Buffer
	var ghwTest v1mock.GhwMock
	var bootloader *v1mock.FakeBootloader

	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewFakeMounter()
		client = &v1mock.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		logger.SetLevel(v1.DebugLevel())
		extractor = v1mock.NewFakeImageExtractor(logger)
		var err error
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).Should(BeNil())

		cloudInit = &v1mock.FakeCloudInitRunner{}
		config = conf.NewRunConfig(
			conf.WithFs(fs),
			conf.WithRunner(runner),
			conf.WithLogger(logger),
			conf.WithMounter(mounter),
			conf.WithSyscall(syscall),
			conf.WithClient(client),
			conf.WithCloudInitRunner(cloudInit),
			conf.WithImageExtractor(extractor),
			conf.WithPlatform("linux/amd64"),
		)
	})

	AfterEach(func() {
		cleanup()
	})

	Describe("Install Action", Label("install"), func() {
		var device, cmdFail string
		var err error
		var cmdline func() ([]byte, error)
		var spec *v1.InstallSpec
		var installer *action.InstallAction

		BeforeEach(func() {
			device = "/some/device"
			err = utils.MkdirAll(fs, filepath.Dir(device), constants.DirPerm)
			Expect(err).To(BeNil())
			_, err = fs.Create(device)
			Expect(err).ShouldNot(HaveOccurred())

			bootloader = &v1mock.FakeBootloader{}

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
						_, _ = fs.Create(fmt.Sprintf("/some/device%d", partNum))
					}
					return []byte(partedOut), nil
				case "lsblk":
					return []byte(`{
"blockdevices":
    [
        {"label": "COS_ACTIVE", "type": "loop", "path": "/some/loop0"},
        {"label": "COS_OEM", "type": "part", "path": "/some/device1"},
        {"label": "COS_RECOVERY", "type": "part", "path": "/some/device2"},
        {"label": "COS_STATE", "type": "part", "path": "/some/device3"},
        {"label": "COS_PERSISTENT", "type": "part", "path": "/some/device4"}
    ]
}`), nil
				case "cat":
					if args[0] == "/proc/cmdline" {
						return cmdline()
					}
					return []byte{}, nil
				default:
					return []byte{}, nil
				}
			}
			// Need to create the IsoBaseTree, like if we are booting from iso
			err = utils.MkdirAll(fs, constants.ISOBaseTree, constants.DirPerm)
			Expect(err).To(BeNil())

			spec = conf.NewInstallSpec(config.Config)
			spec.Active.Size = 16

			grubCfg := filepath.Join(constants.WorkingImgDir, constants.GrubCfgPath, constants.GrubCfg)
			err = utils.MkdirAll(fs, filepath.Dir(grubCfg), constants.DirPerm)
			Expect(err).To(BeNil())
			_, err = fs.Create(grubCfg)
			Expect(err).To(BeNil())

			// Set default cmdline function so we dont panic :o
			cmdline = func() ([]byte, error) {
				return []byte{}, nil
			}
			mainDisk := block.Disk{
				Name: "device",
				Partitions: []*block.Partition{
					{
						Name:            "device1",
						FilesystemLabel: "COS_GRUB",
						Type:            "ext4",
					},
					{
						Name:            "device2",
						FilesystemLabel: "COS_STATE",
						Type:            "ext4",
					},
					{
						Name:            "device3",
						FilesystemLabel: "COS_PERSISTENT",
						Type:            "ext4",
					},
					{
						Name:            "device4",
						FilesystemLabel: "COS_ACTIVE",
						Type:            "ext4",
					},
					{
						Name:            "device5",
						FilesystemLabel: "COS_PASSIVE",
						Type:            "ext4",
					},
					{
						Name:            "device5",
						FilesystemLabel: "COS_RECOVERY",
						Type:            "ext4",
					},
					{
						Name:            "device6",
						FilesystemLabel: "COS_OEM",
						Type:            "ext4",
					},
				},
			}
			ghwTest = v1mock.GhwMock{}
			ghwTest.AddDisk(mainDisk)
			ghwTest.CreateDevices()

			installer = action.NewInstallAction(config, spec, action.WithInstallBootloader(bootloader))
		})
		AfterEach(func() {
			ghwTest.Clean()
		})

		It("Successfully installs", func() {
			spec.Target = device
			config.Reboot = true
			Expect(installer.Run()).To(BeNil())
			Expect(runner.IncludesCmds([][]string{{"reboot", "-f"}}))
		})

		It("Sets the executable /run/cos/ejectcd so systemd can eject the cd on restart", func() {
			_ = utils.MkdirAll(fs, "/usr/lib/systemd/system-shutdown", constants.DirPerm)
			_, err := fs.Stat("/usr/lib/systemd/system-shutdown/eject")
			Expect(err).To(HaveOccurred())
			// Override cmdline to return like we are booting from cd
			cmdline = func() ([]byte, error) {
				return []byte("cdroot"), nil
			}
			spec.Target = device
			config.EjectCD = true
			Expect(installer.Run()).To(BeNil())
			_, err = fs.Stat("/usr/lib/systemd/system-shutdown/eject")
			Expect(err).ToNot(HaveOccurred())
			file, err := fs.ReadFile("/usr/lib/systemd/system-shutdown/eject")
			Expect(err).ToNot(HaveOccurred())
			Expect(file).To(ContainSubstring(constants.EjectScript))
		})

		It("ejectcd does nothing if we are not booting from cd", func() {
			_ = utils.MkdirAll(fs, "/usr/lib/systemd/system-shutdown", constants.DirPerm)
			_, err := fs.Stat("/usr/lib/systemd/system-shutdown/eject")
			Expect(err).To(HaveOccurred())
			spec.Target = device
			config.EjectCD = true
			Expect(installer.Run()).To(BeNil())
			_, err = fs.Stat("/usr/lib/systemd/system-shutdown/eject")
			Expect(err).To(HaveOccurred())
		})

		It("Successfully installs despite hooks failure", Label("hooks"), func() {
			cloudInit.Error = true
			spec.Target = device
			config.PowerOff = true
			Expect(installer.Run()).To(BeNil())
			Expect(runner.IncludesCmds([][]string{{"poweroff", "-f"}}))
		})

		It("Successfully installs without formatting despite detecting a previous installation", Label("no-format", "disk"), func() {
			spec.NoFormat = true
			spec.Force = true
			spec.Target = device
			Expect(installer.Run()).To(BeNil())
		})

		It("Successfully installs a docker image", Label("docker"), func() {
			spec.Target = device
			spec.Active.Source = v1.NewDockerSrc("my/image:latest")
			Expect(installer.Run()).To(BeNil())
		})

		It("Successfully installs and adds remote cloud-config", Label("cloud-config"), func() {
			spec.Target = device
			spec.CloudInit = []string{"http://my.config.org"}
			utils.MkdirAll(fs, constants.OEMDir, constants.DirPerm)
			_, err := fs.Create(filepath.Join(constants.OEMDir, "90_custom.yaml"))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(installer.Run()).To(BeNil())
			Expect(client.WasGetCalledWith("http://my.config.org")).To(BeTrue())
		})

		It("Fails setting the persistent grub variables", func() {
			spec.Target = device
			bootloader.ErrorSetPersistentVariables = true
			err = installer.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("setting persistent variables"))
		})

		It("Fails setting the default grub entry", func() {
			spec.Target = device
			bootloader.ErrorSetDefaultEntry = true
			err = installer.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("setting default entry"))
		})

		It("Fails if disk doesn't exist", Label("disk"), func() {
			spec.Target = "nonexistingdisk"
			Expect(installer.Run()).NotTo(BeNil())
		})

		It("Fails if some hook fails and strict is set", Label("strict"), func() {
			spec.Target = device
			config.Strict = true
			cloudInit.Error = true
			Expect(installer.Run()).NotTo(BeNil())
		})

		It("Fails to install from ISO if the ISO is not found", Label("iso"), func() {
			spec.Iso = "nonexistingiso"
			spec.Target = device
			Expect(installer.Run()).NotTo(BeNil())
		})

		It("Fails to install from ISO as rsync can't find the temporary root tree", Label("iso"), func() {
			fs.Create("cOS.iso")
			spec.Iso = "cOS.iso"
			spec.Target = device
			Expect(installer.Run()).NotTo(BeNil())
			Expect(spec.Active.Source.Value()).To(ContainSubstring("/rootfs"))
			Expect(spec.Active.Source.IsDir()).To(BeTrue())
		})

		It("Fails to install without formatting if a previous install is detected", Label("no-format", "disk"), func() {
			spec.NoFormat = true
			spec.Force = false
			spec.Target = device
			Expect(installer.Run()).NotTo(BeNil())
		})

		It("Fails to mount partitions", Label("disk", "mount"), func() {
			spec.Target = device
			mounter.ErrorOnMount = true
			Expect(installer.Run()).NotTo(BeNil())
		})

		It("Fails on blkdeactivate errors", Label("disk", "partitions"), func() {
			spec.Target = device
			cmdFail = "blkdeactivate"
			Expect(installer.Run()).NotTo(BeNil())
			Expect(runner.MatchMilestones([][]string{{"parted"}}))
		})

		It("Fails on parted errors", Label("disk", "partitions"), func() {
			spec.Target = device
			cmdFail = "parted"
			Expect(installer.Run()).NotTo(BeNil())
			Expect(runner.MatchMilestones([][]string{{"parted"}}))
		})

		It("Fails to unmount partitions", Label("disk", "partitions"), func() {
			spec.Target = device
			mounter.ErrorOnUnmount = true
			Expect(installer.Run()).NotTo(BeNil())
		})

		It("Fails to create a filesystem image", Label("disk", "image"), func() {
			spec.Target = device
			config.Fs = vfs.NewReadOnlyFS(fs)
			Expect(installer.Run()).NotTo(BeNil())
		})

		It("Fails if requested remote cloud config can't be downloaded", Label("cloud-config"), func() {
			spec.Target = device
			spec.CloudInit = []string{"http://my.config.org"}
			client.Error = true
			Expect(installer.Run()).NotTo(BeNil())
			Expect(client.WasGetCalledWith("http://my.config.org")).To(BeTrue())
		})

		It("Fails on grub install errors", Label("grub"), func() {
			spec.Target = device
			bootloader.ErrorInstall = true
			Expect(installer.Run()).NotTo(BeNil())
		})

		It("Fails copying Passive image", Label("copy", "active"), func() {
			spec.Target = device
			cmdFail = "tune2fs"
			Expect(installer.Run()).NotTo(BeNil())
			Expect(runner.MatchMilestones([][]string{{"tune2fs", "-L", constants.PassiveLabel}}))
		})

		It("Fails setting the grub default entry", Label("grub"), func() {
			spec.Target = device
			spec.GrubDefEntry = "cOS"
			bootloader.ErrorSetDefaultEntry = true
			Expect(installer.Run()).NotTo(BeNil())
			Expect(runner.MatchMilestones([][]string{{"grub2-editenv", filepath.Join(constants.StateDir, constants.GrubOEMEnv)}}))
		})
	})
})
