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
	"bytes"
	"errors"
	"fmt"
	luetTypes "github.com/mudler/luet/pkg/api/core/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/action"
	conf "github.com/rancher-sandbox/elemental/pkg/config"
	"github.com/rancher-sandbox/elemental/pkg/constants"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/rancher-sandbox/elemental/pkg/utils"
	v1mock "github.com/rancher-sandbox/elemental/tests/mocks"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const printOutput = `BYT;
/dev/loop0:50593792s:loopback:512:512:gpt:Loopback device:;`
const partTmpl = `
%d:%ss:%ss:2048s:ext4::type=83;`

func TestElementalSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Actions test suite")
}

var _ = Describe("Actions", func() {
	var config *v1.RunConfig
	var runner *v1mock.FakeRunner
	var fs afero.Fs
	var logger v1.Logger
	var mounter *v1mock.ErrorMounter
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHttpClient
	var cloudInit *v1mock.FakeCloudInitRunner

	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHttpClient{}
		logger = v1.NewNullLogger()
		fs = afero.NewMemMapFs()
		cloudInit = &v1mock.FakeCloudInitRunner{}
		config = conf.NewRunConfig(
			v1.WithFs(fs),
			v1.WithRunner(runner),
			v1.WithLogger(logger),
			v1.WithMounter(mounter),
			v1.WithSyscall(syscall),
			v1.WithClient(client),
			v1.WithCloudInitRunner(cloudInit),
		)
	})

	Describe("Reset Setup", Label("reset"), func() {
		var lsblkTmpl, bootedFrom, blkidOut, label, cmdFail string
		BeforeEach(func() {
			fs.Create(constants.EfiDevice)
			bootedFrom = constants.RecoverySquashFile
			blkidOut = "/dev/device1"
			lsblkTmpl = `{
  "blockdevices": [
    {
      "label": "%s", "size": 0, "fstype": "ext4", "mountpoint":"",
      "path":"/dev/device1", "pkname":"/dev/device"
    }
  ]
}`
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmd == cmdFail {
					return []byte{}, errors.New("Command failed")
				}
				switch cmd {
				case "cat":
					return []byte(bootedFrom), nil
				case "lsblk":
					return []byte(fmt.Sprintf(lsblkTmpl, label)), nil
				case "blkid":
					label = args[1]
					return []byte(blkidOut), nil
				default:
					return []byte{}, nil
				}
			}
		})
		It("Configures reset command", func() {
			Expect(action.ResetSetup(config)).To(BeNil())
			Expect(config.Target).To(Equal("/dev/device"))
			Expect(config.ActiveImage.Source.Source).To(Equal(constants.IsoBaseTree))
			Expect(config.ActiveImage.Source.IsDir).To(BeTrue())
		})
		It("Configures reset command with --docker-image", func() {
			config.DockerImg = "some-image"
			Expect(action.ResetSetup(config)).To(BeNil())
			Expect(config.Target).To(Equal("/dev/device"))
			Expect(config.ActiveImage.Source.Source).To(Equal("some-image"))
			Expect(config.ActiveImage.Source.IsDocker).To(BeTrue())
		})
		It("Configures reset command with --directory", func() {
			config.Directory = "/some/local/dir"
			Expect(action.ResetSetup(config)).To(BeNil())
			Expect(config.Target).To(Equal("/dev/device"))
			Expect(config.ActiveImage.Source.Source).To(Equal("/some/local/dir"))
			Expect(config.ActiveImage.Source.IsDir).To(BeTrue())
		})
		It("Fails if not booting from recovery", func() {
			bootedFrom = ""
			Expect(action.ResetSetup(config)).NotTo(BeNil())
		})
		It("Fails if partitions are not found", func() {
			cmdFail = "blkid"
			Expect(action.ResetSetup(config)).NotTo(BeNil())
		})
	})
	Describe("Reset Action", Label("reset"), func() {
		var activeImg v1.Image
		var statePart, persistentPart, oemPart *v1.Partition
		var cmdFail, activeTree, activeMount string
		var err error
		BeforeEach(func() {
			activeTree, err = os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			activeMount, err = os.MkdirTemp("", "elemental")
			Expect(err).To(BeNil())
			fs.Create(filepath.Join(constants.RunningStateDir, "cOS", constants.RecoveryImgFile))
			statePart = &v1.Partition{
				Label:      constants.StateLabel,
				Path:       "/dev/device1",
				Disk:       "/dev/device",
				FS:         constants.LinuxFs,
				Name:       constants.StatePartName,
				MountPoint: constants.StateDir,
			}
			oemPart = &v1.Partition{
				Label:      constants.OEMLabel,
				Path:       "/dev/device2",
				Disk:       "/dev/device",
				FS:         constants.LinuxFs,
				Name:       constants.OEMPartName,
				MountPoint: constants.PersistentDir,
			}
			persistentPart = &v1.Partition{
				Label:      constants.PersistentLabel,
				Path:       "/dev/device3",
				Disk:       "/dev/device",
				FS:         constants.LinuxFs,
				Name:       constants.PersistentPartName,
				MountPoint: constants.OEMDir,
			}
			activeImg.File = filepath.Join(statePart.MountPoint, "cOS", constants.ActiveImgFile)
			activeImg.Label = constants.ActiveLabel
			activeImg.Size = 16
			activeImg.FS = constants.LinuxImgFs
			activeImg.MountPoint = activeMount
			config.ActiveImage = activeImg
			config.Partitions = append(config.Partitions, statePart, oemPart, persistentPart)
			config.Target = statePart.Disk
			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				if cmdFail == cmd {
					return []byte{}, errors.New("Command failed")
				}
				switch cmd {
				case "blkid":
					return []byte("/dev/device"), nil
				default:
					return []byte{}, nil
				}
			}
		})
		AfterEach(func() {
			os.RemoveAll(activeTree)
			os.RemoveAll(activeMount)
		})
		It("Successfully resets on non-squashfs recovery", func() {
			config.Reboot = true
			Expect(action.ResetRun(config)).To(BeNil())
		})
		It("Successfully resets on non-squashfs recovery including persistent data", func() {
			config.ResetPersistent = true
			Expect(action.ResetRun(config)).To(BeNil())
		})
		It("Successfully resets on squashfs recovery", func() {
			config.PowerOff = true
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			Expect(action.ResetRun(config)).To(BeNil())
		})
		It("Successfully resets despite having errors on hooks", func() {
			cloudInit.Error = true
			Expect(action.ResetRun(config)).To(BeNil())
		})
		It("Successfully resets from a docker image", func() {
			config.ActiveImage.Source.Source = "my/image:latest"
			config.ActiveImage.Source.IsDocker = true
			luet := v1mock.NewFakeLuet()
			config.Luet = luet
			Expect(action.ResetRun(config)).To(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})
		It("Fails installing grub", func() {
			cmdFail = "grub2-install"
			Expect(action.ResetRun(config)).NotTo(BeNil())
		})
		It("Fails formatting state partition", func() {
			cmdFail = "mkfs.ext4"
			Expect(action.ResetRun(config)).NotTo(BeNil())
		})
		It("Fails setting the active label on non-squashfs recovery", func() {
			cmdFail = "tune2fs"
			Expect(action.ResetRun(config)).NotTo(BeNil())
		})
		It("Fails setting the passive label on squashfs recovery", func() {
			cmdFail = "tune2fs"
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			Expect(action.ResetRun(config)).NotTo(BeNil())
		})
		It("Fails mounting partitions", func() {
			mounter.ErrorOnMount = true
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			Expect(action.ResetRun(config)).NotTo(BeNil())
		})
		It("Fails unmounting partitions", func() {
			mounter.ErrorOnUnmount = true
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			Expect(action.ResetRun(config)).NotTo(BeNil())
		})
		It("Fails unpacking docker image ", func() {
			config.ActiveImage.Source.Source = "my/image:latest"
			config.ActiveImage.Source.IsDocker = true
			luet := v1mock.NewFakeLuet()
			luet.OnUnpackError = true
			config.Luet = luet
			Expect(action.ResetRun(config)).NotTo(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})
	})
	Describe("Install Setup", Label("install"), func() {
		Describe("On efi system", Label("efi"), func() {
			It(fmt.Sprintf("sets part to %s and boot to %s", v1.GPT, v1.ESP), func() {
				_, _ = fs.Create(constants.EfiDevice)
				err := action.InstallSetup(config)
				Expect(err).To(BeNil())
				Expect(config.PartTable).To(Equal(v1.GPT))
				Expect(config.BootFlag).To(Equal(v1.ESP))
			})
		})
		Describe("On --force-efi flag", func() {
			It(fmt.Sprintf("sets part to %s and boot to %s", v1.GPT, v1.ESP), func() {
				config.ForceEfi = true
				err := action.InstallSetup(config)
				Expect(err).To(BeNil())
				Expect(config.PartTable).To(Equal(v1.GPT))
				Expect(config.BootFlag).To(Equal(v1.ESP))
			})
		})
		Describe("On --force-gpt flag", func() {
			It(fmt.Sprintf("sets part to %s and boot to %s", v1.GPT, v1.BIOS), func() {
				config.ForceGpt = true
				err := action.InstallSetup(config)
				Expect(err).To(BeNil())
				Expect(config.PartTable).To(Equal(v1.GPT))
				Expect(config.BootFlag).To(Equal(v1.BIOS))
			})
		})
		Describe("On default values", func() {
			It(fmt.Sprintf("sets part to %s and boot to %s", v1.MSDOS, v1.BOOT), func() {
				err := action.InstallSetup(config)
				Expect(err).To(BeNil())
				Expect(config.PartTable).To(Equal(v1.MSDOS))
				Expect(config.BootFlag).To(Equal(v1.BOOT))
			})
		})
	})

	Describe("Install Action", Label("install"), func() {
		var device, activeTree, activeMount, cmdFail string
		var activeSize uint
		var err error

		BeforeEach(func() {
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

			action.InstallSetup(config)
		})

		AfterEach(func() {
			os.RemoveAll(activeTree)
			os.RemoveAll(activeMount)
		})

		It("Successfully installs", func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true

			config.ActiveImage.MountPoint = activeMount
			config.Reboot = true
			Expect(action.InstallRun(config)).To(BeNil())
			Expect(runner.IncludesCmds([][]string{{"reboot", "-f"}}))
		})

		It("Successfully installs despite hooks failure", Label("hooks"), func() {
			cloudInit.Error = true
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			config.PowerOff = true
			Expect(action.InstallRun(config)).To(BeNil())
			Expect(runner.IncludesCmds([][]string{{"poweroff", "-f"}}))
		})

		It("Successfully installs from ISO", Label("iso"), func() {
			fs.Create("cOS.iso")
			config.Iso = "cOS.iso"
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			Expect(action.InstallRun(config)).To(BeNil())
		})

		It("Successfully installs without formatting despite detecting a previous installation", Label("no-format", "disk"), func() {
			config.NoFormat = true
			config.Force = true
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			Expect(action.InstallRun(config)).To(BeNil())
		})

		It("Successfully installs a docker image", Label("docker"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = "my/image:latest"
			config.ActiveImage.Source.IsDocker = true
			config.ActiveImage.MountPoint = activeMount
			luet := v1mock.NewFakeLuet()
			config.Luet = luet
			Expect(action.InstallRun(config)).To(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})

		It("Successfully installs and adds remote cloud-config", Label("cloud-config"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			config.CloudInit = "http://my.config.org"
			Expect(action.InstallRun(config)).To(BeNil())
			Expect(client.WasGetCalledWith("http://my.config.org")).To(BeTrue())
		})

		It("Fails if disk doesn't exist", Label("disk"), func() {
			config.Target = "nonexistingdisk"
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails if some hook fails and strict is set", Label("strict"), func() {
			config.Target = device
			config.Strict = true
			cloudInit.Error = true
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails to install from ISO if the ISO is not found", Label("iso"), func() {
			config.Iso = "nonexistingiso"
			config.Target = device
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails to install without formatting if a previous install is detected", Label("no-format", "disk"), func() {
			config.NoFormat = true
			config.Force = false
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails to mount partitions", Label("disk", "mount"), func() {
			config.Target = device
			mounter.ErrorOnMount = true
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails on parted errors", Label("disk", "partitions"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			cmdFail = "parted"
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails to unmount partitions", Label("disk", "partitions"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true

			config.ActiveImage.MountPoint = activeMount
			mounter.ErrorOnUnmount = true
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails to create a filesystem image", Label("disk", "image"), func() {
			config.Target = device
			config.Fs = afero.NewReadOnlyFs(fs)
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails if luet fails to unpack image", Label("image", "luet", "unpack"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = "my/image:latest"
			config.ActiveImage.Source.IsDocker = true
			config.ActiveImage.MountPoint = activeMount
			luet := v1mock.NewFakeLuet()
			luet.OnUnpackError = true
			config.Luet = luet
			Expect(action.InstallRun(config)).NotTo(BeNil())
			Expect(luet.UnpackCalled()).To(BeTrue())
		})

		It("Fails if requested remote cloud config can't be downloaded", Label("cloud-config"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			config.CloudInit = "http://my.config.org"
			client.Error = true
			Expect(action.InstallRun(config)).NotTo(BeNil())
			Expect(client.WasGetCalledWith("http://my.config.org")).To(BeTrue())
		})

		It("Fails on grub2-install errors", Label("grub"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			cmdFail = "grub2-install"
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails copying Passive image", Label("copy", "active"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			cmdFail = "tune2fs"
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})

		It("Fails setting the grub default entry", Label("grub"), func() {
			config.Target = device
			config.ActiveImage.Size = activeSize
			config.ActiveImage.Source.Source = activeTree
			config.ActiveImage.Source.IsDir = true
			config.ActiveImage.MountPoint = activeMount
			cmdFail = "grub2-editenv"
			Expect(action.InstallRun(config)).NotTo(BeNil())
		})
	})
	Describe("Upgrade Action", Label("upgrade"), func() {
		var upgrade *action.UpgradeAction
		var memLog *bytes.Buffer
		var luet *v1.Luet
		activeImg := fmt.Sprintf("%s/cOS/%s", constants.RunningStateDir, constants.ActiveImgFile)
		passiveImg := fmt.Sprintf("%s/cOS/%s", constants.RunningStateDir, constants.PassiveImgFile)
		recoveryImgSquash := fmt.Sprintf("%s/cOS/%s", constants.UpgradeRecoveryDir, constants.RecoverySquashFile)
		recoveryImg := fmt.Sprintf("%s/cOS/%s", constants.UpgradeRecoveryDir, constants.RecoveryImgFile)
		transitionImgSquash := fmt.Sprintf("%s/cOS/%s", constants.UpgradeRecoveryDir, constants.TransitionSquashFile)
		transitionImg := fmt.Sprintf("%s/cOS/%s", constants.RunningStateDir, constants.TransitionImgFile)
		transitionImgRecovery := fmt.Sprintf("%s/cOS/%s", constants.UpgradeRecoveryDir, constants.TransitionImgFile)

		BeforeEach(func() {
			memLog = &bytes.Buffer{}
			logger = v1.NewBufferLogger(memLog)
			config.Logger = logger
			logger.SetLevel(logrus.DebugLevel)
			luet = v1.NewLuet(v1.WithLuetLogger(logger))
			config.Luet = luet
			// These values are loaded from /etc/cos/config normally via CMD
			config.StateLabel = constants.StateLabel
			config.PassiveLabel = constants.PassiveLabel
			config.RecoveryLabel = constants.RecoveryLabel
			config.ActiveLabel = constants.ActiveLabel
			config.UpgradeImage = "system/cos-config"
			config.RecoveryImage = "system/cos-config"
			config.ImgSize = 10
			// Create fake /etc/os-release
			_ = afero.WriteFile(fs, filepath.Join(utils.GetUpgradeTempDir(config), "etc", "os-release"), []byte("GRUB_ENTRY_NAME=TESTOS"), os.ModePerm)
		})
		It("Fails if some hook fails and strict is set", func() {
			runner.SideEffect = func(command string, args ...string) ([]byte, error) {
				if command == "blkid" && args[0] == "--label" && args[1] == constants.StateLabel {
					return []byte("/dev/active"), nil
				}
				if command == "blkid" && args[0] == "--label" && args[1] == constants.RecoveryLabel {
					return []byte("/dev/passive"), nil
				}
				if command == "lsblk" {
					return []byte(`{"blockdevices":[{"label":"fake","size":1,"partlabel":"pfake","fstype":"fakefs","partflags":null,"mountpoint":"/mnt/fake", "path": "/dev/fake1"}]}`), nil
				}
				if command == "cat" && args[0] == "/proc/cmdline" {
					return []byte(constants.ActiveLabel), nil
				}
				return []byte{}, nil
			}
			config.Runner = runner
			config.DockerImg = "alpine"
			config.Strict = true
			cloudInit.Error = true
			upgrade = action.NewUpgradeAction(config)
			err := upgrade.Run()
			Expect(err).To(HaveOccurred())
			// Make sure is a cloud init error!
			Expect(err.Error()).To(ContainSubstring("cloud init"))
		})
		Describe(fmt.Sprintf("Booting from %s", constants.ActiveLabel), Label("active_label"), func() {
			BeforeEach(func() {
				runner.SideEffect = func(command string, args ...string) ([]byte, error) {
					if command == "blkid" && args[0] == "--label" && args[1] == constants.StateLabel {
						return []byte("/dev/active"), nil
					}
					if command == "blkid" && args[0] == "--label" && args[1] == constants.RecoveryLabel {
						return []byte("/dev/passive"), nil
					}

					if command == "lsblk" {
						return []byte(`{"blockdevices":[{"label":"fake","size":1,"partlabel":"pfake","fstype":"fakefs","partflags":null,"mountpoint": "/run/initramfs/cos-state", "path": "/dev/fake1"}]}`), nil
					}
					if command == "cat" && args[0] == "/proc/cmdline" {
						return []byte(constants.ActiveLabel), nil
					}
					if command == "mv" && args[0] == "-f" && args[1] == activeImg && args[2] == passiveImg {
						// we doing backup, do the "move"
						source, _ := afero.ReadFile(fs, activeImg)
						_ = afero.WriteFile(fs, passiveImg, source, os.ModePerm)
						_ = fs.RemoveAll(activeImg)
					}
					if command == "mv" && args[0] == "-f" && args[1] == transitionImg && args[2] == activeImg {
						// we doing the image substitution, do the "move"
						source, _ := afero.ReadFile(fs, transitionImg)
						_ = afero.WriteFile(fs, activeImg, source, os.ModePerm)
						_ = fs.RemoveAll(transitionImg)
					}
					return []byte{}, nil
				}
				config.Runner = runner
				// Create fake active/passive files
				_ = afero.WriteFile(fs, activeImg, []byte("active"), os.ModePerm)
				_ = afero.WriteFile(fs, passiveImg, []byte("passive"), os.ModePerm)
			})
			AfterEach(func() {
				_ = fs.RemoveAll(activeImg)
				_ = fs.RemoveAll(passiveImg)
			})
			It("Successfully upgrades from docker image", Label("docker", "root"), func() {
				config.DockerImg = "alpine"
				upgrade = action.NewUpgradeAction(config)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Expect cos-state to have been mounted with our fake lsblk values
				fakeMounted := mount.MountPoint{
					Device: "/dev/fake1",
					Path:   "/run/initramfs/cos-state",
					Type:   "fakefs",
				}
				Expect(mounter.List()).To(ContainElement(fakeMounted))

				// This should be the new image
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should be the config.ImgSize as its truncated from the upgrade
				Expect(info.Size()).To(BeNumerically("==", int64(config.ImgSize*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Should have backed up active to passive
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be a tiny image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(config.ImgSize*1024*1024)))
				f, _ := afero.ReadFile(fs, passiveImg)
				// This should be a backup so it should read active
				Expect(f).To(ContainSubstring("active"))

				// Expect transition image to be gone
				_, err = fs.Stat(transitionImg)
				Expect(err).To(HaveOccurred())
			})
			It("Successfully upgrades from directory", Label("directory", "root"), func() {
				config.DirectoryUpgrade, _ = os.MkdirTemp("", "elemental")
				// Create the dir on real os as rsync works on the real os
				defer os.RemoveAll(config.DirectoryUpgrade)
				// create a random file on it
				err := os.WriteFile(fmt.Sprintf("%s/file.file", config.DirectoryUpgrade), []byte("something"), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())

				upgrade = action.NewUpgradeAction(config)
				err = upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Not much that we can create here as the dir copy was done on the real os, but we do the rest of the ops on a mem one
				// This should be the new image
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should not be empty
				Expect(info.Size()).To(BeNumerically("==", int64(config.ImgSize*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Should have backed up active to passive
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be an really small image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(config.ImgSize*1024*1024)))
				f, _ := afero.ReadFile(fs, passiveImg)
				// This should be a backup so it should read active
				Expect(f).To(ContainSubstring("active"))

				// Expect transition image to be gone
				_, err = fs.Stat(transitionImg)
				Expect(err).To(HaveOccurred())

			})
			It("Successfully upgrades from channel upgrade", Label("channel", "root"), func() {
				config.ChannelUpgrades = true
				// Required paths
				tmpDirBase, _ := os.MkdirTemp("", "elemental")
				pkgCache, _ := os.MkdirTemp("", "elemental")
				dbPath, _ := os.MkdirTemp("", "elemental")
				defer os.RemoveAll(tmpDirBase)
				defer os.RemoveAll(pkgCache)
				defer os.RemoveAll(dbPath)
				// create new config here to add system repos
				luetSystemConfig := luetTypes.LuetSystemConfig{
					DatabasePath:   dbPath,
					PkgsCachePath:  pkgCache,
					DatabaseEngine: "memory",
					TmpDirBase:     tmpDirBase,
				}
				luetGeneralConfig := luetTypes.LuetGeneralConfig{Debug: false, Quiet: true, Concurrency: runtime.NumCPU()}
				luetSolver := luetTypes.LuetSolverOptions{}
				repos := luetTypes.LuetRepositories{}
				repo := luetTypes.LuetRepository{
					Name:           "cos",
					Description:    "cos official",
					Urls:           []string{"quay.io/costoolkit/releases-green"},
					Type:           "docker",
					Priority:       1,
					Enable:         true,
					Cached:         true,
					Authentication: make(map[string]string),
				}
				repos = append(repos, repo)

				cfg := luetTypes.LuetConfig{System: luetSystemConfig, Solver: luetSolver, General: luetGeneralConfig, SystemRepositories: repos}
				luet = v1.NewLuet(v1.WithLuetLogger(logger), v1.WithLuetConfig(&cfg))
				config.Luet = luet

				upgrade = action.NewUpgradeAction(config)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Not much that we can create here as the dir copy was done on the real os, but we do the rest of the ops on a mem one
				// This should be the new image
				// Should probably do well in mounting the image and checking contents to make sure everything worked
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should not be empty
				Expect(info.Size()).To(BeNumerically("==", int64(config.ImgSize*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Should have backed up active to passive
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be an really small image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(config.ImgSize*1024*1024)))
				f, _ := afero.ReadFile(fs, passiveImg)
				// This should be a backup so it should read active
				Expect(f).To(ContainSubstring("active"))

				// Expect transition image to be gone
				_, err = fs.Stat(transitionImg)
				Expect(err).To(HaveOccurred())
			})
			It("Successfully upgrades with cosign", Pending, Label("channel", "cosign", "root"), func() {})
			It("Successfully upgrades with mtree", Pending, Label("channel", "mtree", "root"), func() {})
			It("Successfully upgrades with strict", Pending, Label("channel", "strict", "root"), func() {})
		})
		Describe(fmt.Sprintf("Booting from %s", constants.PassiveLabel), Label("passive_label"), func() {
			BeforeEach(func() {
				runner.SideEffect = func(command string, args ...string) ([]byte, error) {
					if command == "blkid" && args[0] == "--label" && args[1] == constants.StateLabel {
						return []byte("/dev/active"), nil
					}
					if command == "blkid" && args[0] == "--label" && args[1] == constants.RecoveryLabel {
						return []byte("/dev/passive"), nil
					}

					if command == "lsblk" {
						return []byte(`{"blockdevices":[{"label":"fake","size":1,"partlabel":"pfake","fstype":"fakefs","partflags":null,"mountpoint":"/run/initramfs/cos-state", "path": "/dev/fake1"}]}`), nil
					}
					if command == "cat" && args[0] == "/proc/cmdline" {
						return []byte(constants.PassiveLabel), nil
					}
					if command == "mv" && args[0] == "-f" && args[1] == transitionImg && args[2] == activeImg {
						// we doing the image substitution, do the "move"
						source, _ := afero.ReadFile(fs, transitionImg)
						_ = afero.WriteFile(fs, activeImg, source, os.ModePerm)
						_ = fs.RemoveAll(transitionImg)
					}
					return []byte{}, nil
				}
				config.Runner = runner
				// Create fake active/passive files
				_ = afero.WriteFile(fs, activeImg, []byte("active"), os.ModePerm)
				_ = afero.WriteFile(fs, passiveImg, []byte("passive"), os.ModePerm)
			})
			AfterEach(func() {
				_ = fs.RemoveAll(activeImg)
				_ = fs.RemoveAll(passiveImg)
			})
			It("does not backup active img to passive", Label("docker", "root"), func() {
				config.DockerImg = "alpine"
				upgrade = action.NewUpgradeAction(config)
				err := upgrade.Run()
				Expect(err).ToNot(HaveOccurred())
				// Check that the rebrand worked with our os-release value
				Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

				// Expect cos-state to have been mounted with our fake lsblk values
				fakeMounted := mount.MountPoint{
					Device: "/dev/fake1",
					Path:   "/run/initramfs/cos-state",
					Type:   "fakefs",
				}
				Expect(mounter.List()).To(ContainElement(fakeMounted))

				// This should be the new image
				info, err := fs.Stat(activeImg)
				Expect(err).ToNot(HaveOccurred())
				// Image size should not be empty
				Expect(info.Size()).To(BeNumerically("==", int64(config.ImgSize*1024*1024)))
				Expect(info.IsDir()).To(BeFalse())

				// Passive should have not been touched
				info, err = fs.Stat(passiveImg)
				Expect(err).ToNot(HaveOccurred())
				// Should be a tiny image as it should only contain our text
				// As this was generated by us at the start test and moved by the upgrade from active.iomg
				Expect(info.Size()).To(BeNumerically(">", 0))
				Expect(info.Size()).To(BeNumerically("<", int64(config.ImgSize*1024*1024)))
				f, _ := afero.ReadFile(fs, passiveImg)
				Expect(f).To(ContainSubstring("passive"))

				// Expect transition image to be gone
				_, err = fs.Stat(transitionImg)
				Expect(err).To(HaveOccurred())

			})
		})
		Describe(fmt.Sprintf("Booting from %s", constants.RecoveryLabel), Label("recovery_label"), func() {
			BeforeEach(func() {
				config.RecoveryUpgrade = true
			})
			Describe("Using squashfs", Label("squashfs"), func() {
				BeforeEach(func() {
					runner.SideEffect = func(command string, args ...string) ([]byte, error) {
						if command == "blkid" && args[0] == "--label" && args[1] == constants.RecoveryLabel {
							return []byte("/dev/active"), nil
						}
						if command == "blkid" && args[0] == "--label" && args[1] == constants.StateLabel {
							return []byte("/dev/passive"), nil
						}

						if command == "lsblk" {
							return []byte(`{"blockdevices":[{"label":"fake","size":1,"partlabel":"pfake","fstype":"fakefs","partflags":null,"mountpoint":"/run/initramfs/live", "path": "/dev/fake1"}]}`), nil
						}
						if command == "cat" && args[0] == "/proc/cmdline" {
							return []byte(constants.RecoveryLabel), nil
						}
						if command == "mksquashfs" && args[0] == "/tmp/upgrade" && args[1] == "/run/initramfs/live/cOS/transition.squashfs" {
							// create the transition img for squash to fake it
							_, _ = fs.Create(transitionImgSquash)
						}
						if command == "mv" && args[0] == "-f" && args[1] == transitionImgSquash && args[2] == recoveryImgSquash {
							// fake "move"
							f, _ := afero.ReadFile(fs, transitionImgSquash)
							_ = afero.WriteFile(fs, recoveryImgSquash, f, os.ModePerm)
							_ = fs.RemoveAll(transitionImgSquash)
						}
						if command == "blkid" && args[0] == "--label" && args[1] == constants.StateLabel {
							// rebrand looks for this
							return []byte("/dev/state"), nil
						}

						return []byte{}, nil
					}
					config.Runner = runner
					// Create recoveryImgSquash so ti identifies that we are using squash recovery
					_ = afero.WriteFile(fs, recoveryImgSquash, []byte("recovery"), os.ModePerm)
				})
				AfterEach(func() {
					_ = fs.RemoveAll(activeImg)
					_ = fs.RemoveAll(passiveImg)
				})
				It("Successfully upgrades recovery from docker image", Label("docker", "root"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := afero.ReadFile(fs, recoveryImgSquash)
					Expect(f).To(ContainSubstring("recovery"))

					config.DockerImg = "alpine"
					upgrade = action.NewUpgradeAction(config)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// Check that the rebrand worked with our os-release value
					Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

					// Expect cos-state to have been remounted back on RO
					fakeMounted := mount.MountPoint{
						Device: "/dev/fake1",
						Path:   "/run/initramfs/live",
						Type:   "fakefs",
					}
					Expect(mounter.List()).To(ContainElement(fakeMounted))

					// This should be the new image
					info, err = fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically("==", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ = afero.ReadFile(fs, recoveryImgSquash)
					Expect(f).ToNot(ContainSubstring("recovery"))

					// Transition squash should not exist
					info, err = fs.Stat(transitionImgSquash)
					Expect(err).To(HaveOccurred())

				})
				It("Successfully upgrades recovery from directory", Label("directory", "root"), func() {
					config.DirectoryUpgrade, _ = os.MkdirTemp("", "elemental")
					// Create the dir on real os as rsync works on the real os
					defer os.RemoveAll(config.DirectoryUpgrade)
					// create a random file on it
					_ = os.WriteFile(fmt.Sprintf("%s/file.file", config.DirectoryUpgrade), []byte("something"), os.ModePerm)

					upgrade = action.NewUpgradeAction(config)
					err := upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// Check that the rebrand worked with our os-release value
					Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

					// Expect cos-state to have been remounted back on RO
					fakeMounted := mount.MountPoint{
						Device: "/dev/fake1",
						Path:   "/run/initramfs/live",
						Type:   "fakefs",
					}
					Expect(mounter.List()).To(ContainElement(fakeMounted))

					// This should be the new image
					info, err := fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically("==", 0))
					Expect(info.IsDir()).To(BeFalse())

					// Transition squash should not exist
					info, err = fs.Stat(transitionImgSquash)
					Expect(err).To(HaveOccurred())

				})
				It("Successfully upgrades recovery from channel upgrade", Label("channel", "root"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := afero.ReadFile(fs, recoveryImgSquash)
					Expect(f).To(ContainSubstring("recovery"))

					config.ChannelUpgrades = true
					// Required paths
					tmpDirBase := filepath.Join(os.TempDir(), "tmpluet")
					defer os.RemoveAll(tmpDirBase)
					// create new config here to add system repos
					luetSystemConfig := luetTypes.LuetSystemConfig{
						DatabasePath:   filepath.Join(tmpDirBase, "db"),
						PkgsCachePath:  filepath.Join(tmpDirBase, "cache"),
						DatabaseEngine: "",
						TmpDirBase:     tmpDirBase,
					}
					luetGeneralConfig := luetTypes.LuetGeneralConfig{Debug: false, Quiet: true, Concurrency: runtime.NumCPU()}
					luetSolver := luetTypes.LuetSolverOptions{}
					repos := luetTypes.LuetRepositories{}
					repo := luetTypes.LuetRepository{
						Name:           "cos",
						Description:    "cos official",
						Urls:           []string{"quay.io/costoolkit/releases-green"},
						Type:           "docker",
						Priority:       1,
						Enable:         true,
						Cached:         true,
						Authentication: make(map[string]string),
					}
					repos = append(repos, repo)

					cfg := luetTypes.LuetConfig{System: luetSystemConfig, Solver: luetSolver, General: luetGeneralConfig, SystemRepositories: repos}
					luet = v1.NewLuet(v1.WithLuetLogger(logger), v1.WithLuetConfig(&cfg))
					config.Luet = luet

					upgrade = action.NewUpgradeAction(config)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// Check that the rebrand worked with our os-release value
					Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

					// Expect cos-state to have been remounted back on RO
					fakeMounted := mount.MountPoint{
						Device: "/dev/fake1",
						Path:   "/run/initramfs/live",
						Type:   "fakefs",
					}
					Expect(mounter.List()).To(ContainElement(fakeMounted))

					// This should be the new image
					info, err = fs.Stat(recoveryImgSquash)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be empty
					Expect(info.Size()).To(BeNumerically("==", 0))
					Expect(info.IsDir()).To(BeFalse())
					f, _ = afero.ReadFile(fs, recoveryImgSquash)
					Expect(f).ToNot(ContainSubstring("recovery"))

					// Transition squash should not exist
					info, err = fs.Stat(transitionImgSquash)
					Expect(err).To(HaveOccurred())
				})
			})
			Describe("Not using squashfs", Label("non-squashfs"), func() {
				BeforeEach(func() {
					runner.SideEffect = func(command string, args ...string) ([]byte, error) {
						if command == "blkid" && args[0] == "--label" && args[1] == constants.RecoveryLabel {
							return []byte("/dev/active"), nil
						}
						if command == "blkid" && args[0] == "--label" && args[1] == constants.StateLabel {
							return []byte("/dev/passive"), nil
						}
						if command == "lsblk" {
							return []byte(`{"blockdevices":[{"label":"fake","size":1,"partlabel":"pfake","fstype":"fakefs","partflags":null,"mountpoint":"/run/initramfs/live", "path": "/dev/fake1"}]}`), nil
						}
						if command == "cat" && args[0] == "/proc/cmdline" {
							return []byte(constants.RecoveryLabel), nil
						}
						if command == "mv" && args[0] == "-f" && args[1] == transitionImgRecovery && args[2] == recoveryImg {
							// fake "move"
							f, _ := afero.ReadFile(fs, transitionImgRecovery)
							_ = afero.WriteFile(fs, recoveryImg, f, os.ModePerm)
							_ = fs.RemoveAll(transitionImgRecovery)
						}
						if command == "blkid" && args[0] == "--label" && args[1] == constants.StateLabel {
							// rebrand looks for this
							return []byte("/dev/state"), nil
						}
						return []byte{}, nil
					}
					config.Runner = runner
					_ = afero.WriteFile(fs, recoveryImg, []byte("recovery"), os.ModePerm)

				})
				AfterEach(func() {
					_ = fs.RemoveAll(activeImg)
					_ = fs.RemoveAll(passiveImg)
					_ = fs.RemoveAll(recoveryImg)
				})
				It("Successfully upgrades recovery from docker image", Label("docker", "root"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should not be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.Size()).To(BeNumerically("<", int64(config.ImgSize*1024*1024)))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := afero.ReadFile(fs, recoveryImg)
					Expect(f).To(ContainSubstring("recovery"))

					config.DockerImg = "alpine"
					config.Logger.SetLevel(logrus.DebugLevel)
					upgrade = action.NewUpgradeAction(config)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// Check that the rebrand worked with our os-release value
					Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

					// Expect cos-state to have been remounted back on RO
					fakeMounted := mount.MountPoint{
						Device: "/dev/fake1",
						Path:   "/run/initramfs/live",
						Type:   "fakefs",
					}
					Expect(mounter.List()).To(ContainElement(fakeMounted))

					// Should have created recovery image
					info, err = fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be default size
					Expect(info.Size()).To(BeNumerically("==", int64(config.ImgSize*1024*1024)))

					// Expect the rest of the images to not be there
					for _, img := range []string{activeImg, passiveImg, recoveryImgSquash} {
						exists, _ := afero.Exists(fs, img)
						Expect(exists).To(BeFalse())
					}

				})
				It("Successfully upgrades recovery from directory", Label("directory", "root"), func() {
					config.DirectoryUpgrade, _ = os.MkdirTemp("", "elemental")
					// Create the dir on real os as rsync works on the real os
					defer os.RemoveAll(config.DirectoryUpgrade)
					// create a random file on it
					_ = os.WriteFile(fmt.Sprintf("%s/file.file", config.DirectoryUpgrade), []byte("something"), os.ModePerm)

					upgrade = action.NewUpgradeAction(config)
					err := upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// Check that the rebrand worked with our os-release value
					Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

					// Expect cos-state to have been remounted back on RO
					fakeMounted := mount.MountPoint{
						Device: "/dev/fake1",
						Path:   "/run/initramfs/live",
						Type:   "fakefs",
					}
					Expect(mounter.List()).To(ContainElement(fakeMounted))

					// This should be the new image
					info, err := fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should be default size
					Expect(info.Size()).To(BeNumerically("==", int64(config.ImgSize*1024*1024)))
					Expect(info.IsDir()).To(BeFalse())

					// Transition squash should not exist
					info, err = fs.Stat(transitionImgRecovery)
					Expect(err).To(HaveOccurred())
				})
				It("Successfully upgrades recovery from channel upgrade", Label("channel", "root"), func() {
					// This should be the old image
					info, err := fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Image size should not be empty
					Expect(info.Size()).To(BeNumerically(">", 0))
					Expect(info.Size()).To(BeNumerically("<", int64(config.ImgSize*1024*1024)))
					Expect(info.IsDir()).To(BeFalse())
					f, _ := afero.ReadFile(fs, recoveryImg)
					Expect(f).To(ContainSubstring("recovery"))

					config.ChannelUpgrades = true
					// Required paths
					tmpDirBase, _ := os.MkdirTemp("", "elemental")
					pkgCache, _ := os.MkdirTemp("", "elemental")
					dbPath, _ := os.MkdirTemp("", "elemental")
					defer os.RemoveAll(tmpDirBase)
					defer os.RemoveAll(pkgCache)
					defer os.RemoveAll(dbPath)
					// create new config here to add system repos
					luetSystemConfig := luetTypes.LuetSystemConfig{
						DatabasePath:   dbPath,
						PkgsCachePath:  pkgCache,
						DatabaseEngine: "memory",
						TmpDirBase:     tmpDirBase,
					}
					luetGeneralConfig := luetTypes.LuetGeneralConfig{Debug: false, Quiet: true, Concurrency: runtime.NumCPU()}
					luetSolver := luetTypes.LuetSolverOptions{}
					repos := luetTypes.LuetRepositories{}
					repo := luetTypes.LuetRepository{
						Name:           "cos",
						Description:    "cos official",
						Urls:           []string{"quay.io/costoolkit/releases-green"},
						Type:           "docker",
						Priority:       1,
						Enable:         true,
						Cached:         true,
						Authentication: make(map[string]string),
					}
					repos = append(repos, repo)

					cfg := luetTypes.LuetConfig{System: luetSystemConfig, Solver: luetSolver, General: luetGeneralConfig, SystemRepositories: repos}
					luet = v1.NewLuet(v1.WithLuetLogger(logger), v1.WithLuetConfig(&cfg))
					config.Luet = luet

					upgrade = action.NewUpgradeAction(config)
					err = upgrade.Run()
					Expect(err).ToNot(HaveOccurred())

					// Check that the rebrand worked with our os-release value
					Expect(memLog).To(ContainSubstring("default_menu_entry=TESTOS"))

					// Expect cos-state to have been remounted back on RO
					fakeMounted := mount.MountPoint{
						Device: "/dev/fake1",
						Path:   "/run/initramfs/live",
						Type:   "fakefs",
					}
					Expect(mounter.List()).To(ContainElement(fakeMounted))

					// Should have created recovery image
					info, err = fs.Stat(recoveryImg)
					Expect(err).ToNot(HaveOccurred())
					// Should have default image size
					Expect(info.Size()).To(BeNumerically("==", int64(config.ImgSize*1024*1024)))

					// Expect the rest of the images to not be there
					for _, img := range []string{activeImg, passiveImg, recoveryImgSquash} {
						exists, _ := afero.Exists(fs, img)
						Expect(exists).To(BeFalse())
					}
				})
			})
		})
	})
})
