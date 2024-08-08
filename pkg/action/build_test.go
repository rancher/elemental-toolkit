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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"

	"github.com/rancher/elemental-toolkit/v2/pkg/action"
	"github.com/rancher/elemental-toolkit/v2/pkg/config"
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/mocks"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

var _ = Describe("Build Actions", func() {
	var cfg *types.BuildConfig
	var runner *mocks.FakeRunner
	var fs vfs.FS
	var logger types.Logger
	var mounter *mocks.FakeMounter
	var syscall *mocks.FakeSyscall
	var client *mocks.FakeHTTPClient
	var cloudInit *mocks.FakeCloudInitRunner
	var extractor *mocks.FakeImageExtractor
	var cleanup func()
	var memLog *bytes.Buffer
	var bootloader *mocks.FakeBootloader

	BeforeEach(func() {
		runner = mocks.NewFakeRunner()
		syscall = &mocks.FakeSyscall{}
		mounter = mocks.NewFakeMounter()
		client = &mocks.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		bootloader = &mocks.FakeBootloader{}
		logger = types.NewBufferLogger(memLog)
		logger.SetLevel(logrus.DebugLevel)
		extractor = mocks.NewFakeImageExtractor(logger)
		cloudInit = &mocks.FakeCloudInitRunner{}
		fs, cleanup, _ = vfst.NewTestFS(map[string]interface{}{})
		cfg = config.NewBuildConfig(
			config.WithFs(fs),
			config.WithRunner(runner),
			config.WithLogger(logger),
			config.WithMounter(mounter),
			config.WithSyscall(syscall),
			config.WithClient(client),
			config.WithCloudInitRunner(cloudInit),
			config.WithImageExtractor(extractor),
			config.WithPlatform("linux/amd64"),
		)

	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Build ISO", Label("iso"), func() {
		var iso *types.LiveISO
		BeforeEach(func() {
			iso = config.NewISO()

			tmpDir, err := utils.TempDir(fs, "", "test")
			Expect(err).ShouldNot(HaveOccurred())

			cfg.Date = false
			cfg.OutDir = tmpDir

			runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
				switch cmd {
				case "xorriso":
					err := fs.WriteFile(filepath.Join(tmpDir, "elemental.iso"), []byte("profound thoughts"), constants.FilePerm)
					return []byte{}, err
				default:
					return []byte{}, nil
				}
			}
		})
		It("Successfully builds an ISO from an OCI image", func() {
			rootSrc, _ := types.NewSrcFromURI("oci:elementalos:latest")
			iso.RootFS = []*types.ImageSource{rootSrc}

			extractor.SideEffect = func(_, destination, platform string, _, _ bool) (string, error) {
				bootDir := filepath.Join(destination, "boot")
				logger.Debugf("Creating %s", bootDir)
				err := utils.MkdirAll(fs, bootDir, constants.DirPerm)
				if err != nil {
					return mocks.FakeDigest, err
				}
				err = utils.MkdirAll(fs, filepath.Join(destination, "lib/modules/6.4"), constants.DirPerm)
				if err != nil {
					return mocks.FakeDigest, err
				}
				_, err = fs.Create(filepath.Join(bootDir, "vmlinuz-6.4"))
				if err != nil {
					return mocks.FakeDigest, err
				}

				_, err = fs.Create(filepath.Join(bootDir, "initrd"))
				return mocks.FakeDigest, err
			}

			buildISO := action.NewBuildISOAction(cfg, iso, action.WithLiveBootloader(bootloader))
			err := buildISO.Run()

			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Fails on prepare EFI", func() {
			iso.BootloaderInRootFs = true

			rootSrc, _ := types.NewSrcFromURI("oci:elementalos:latest")
			iso.RootFS = append(iso.RootFS, rootSrc)

			buildISO := action.NewBuildISOAction(cfg, iso, action.WithLiveBootloader(bootloader))
			err := buildISO.Run()
			Expect(err).Should(HaveOccurred())
		})
		It("Fails on prepare ISO", func() {
			iso.BootloaderInRootFs = true

			rootSrc, _ := types.NewSrcFromURI("channel:system/elemental")
			iso.RootFS = append(iso.RootFS, rootSrc)

			buildISO := action.NewBuildISOAction(cfg, iso, action.WithLiveBootloader(bootloader))
			err := buildISO.Run()

			Expect(err).Should(HaveOccurred())
		})
		It("Fails if kernel or initrd is not found in rootfs", func() {
			rootSrc, _ := types.NewSrcFromURI("dir:/local/dir")
			iso.RootFS = []*types.ImageSource{rootSrc}

			err := utils.MkdirAll(fs, "/local/dir/boot", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())

			By("fails without kernel")
			buildISO := action.NewBuildISOAction(cfg, iso, action.WithLiveBootloader(bootloader))
			err = buildISO.Run()
			Expect(err).Should(HaveOccurred())

			By("fails without initrd")
			_, err = fs.Create("/local/dir/boot/vmlinuz")
			Expect(err).ShouldNot(HaveOccurred())
			buildISO = action.NewBuildISOAction(cfg, iso, action.WithLiveBootloader(bootloader))
			err = buildISO.Run()
			Expect(err).Should(HaveOccurred())
		})
		It("Fails installing uefi sources", func() {
			rootSrc, _ := types.NewSrcFromURI("docker:elemental:latest")
			iso.RootFS = []*types.ImageSource{rootSrc}
			uefiSrc, _ := types.NewSrcFromURI("dir:/overlay/efi")
			iso.UEFI = []*types.ImageSource{uefiSrc}

			buildISO := action.NewBuildISOAction(cfg, iso)
			err := buildISO.Run()
			Expect(err).Should(HaveOccurred())
		})
		It("Fails on ISO filesystem creation", func() {
			rootSrc, _ := types.NewSrcFromURI("oci:elementalos:latest")
			iso.RootFS = []*types.ImageSource{rootSrc}

			runner.SideEffect = func(command string, args ...string) ([]byte, error) {
				if command == "xorriso" {
					return []byte{}, errors.New("Burn ISO error")
				}
				return []byte{}, nil
			}

			buildISO := action.NewBuildISOAction(cfg, iso, action.WithLiveBootloader(bootloader))
			err := buildISO.Run()

			Expect(err).Should(HaveOccurred())
		})
	})
	Describe("Build disk", Label("disk", "build"), func() {
		var disk *types.DiskSpec

		BeforeEach(func() {
			tmpDir, err := utils.TempDir(fs, "", "test")
			Expect(err).ShouldNot(HaveOccurred())

			recDir := filepath.Join(tmpDir, "build/recovery.img.root")
			Expect(utils.MkdirAll(fs, filepath.Join(recDir, "boot"), constants.DirPerm)).To(Succeed())
			Expect(utils.MkdirAll(fs, filepath.Join(recDir, "/lib/modules/6.7"), constants.DirPerm)).To(Succeed())
			_, err = fs.Create(filepath.Join(recDir, "/boot/vmlinuz-6.7"))
			Expect(err).To(Succeed())
			_, err = fs.Create(filepath.Join(recDir, "/boot/elemental.initrd-6.7"))
			Expect(err).To(Succeed())

			cfg.Date = false
			cfg.OutDir = tmpDir
			disk = config.NewDisk(cfg)
			disk.System = types.NewDockerSrc("some/image/ref:tag")
			disk.Partitions.Recovery.Size = constants.MinPartSize
			disk.Partitions.State.Size = constants.MinPartSize
			disk.RecoverySystem.Source = types.NewDirSrc(recDir)
		})
		It("Successfully builds a full raw disk", func() {
			buildDisk, err := action.NewBuildDiskAction(cfg, disk, action.WithDiskBootloader(bootloader))
			Expect(err).NotTo(HaveOccurred())

			Expect(buildDisk.BuildDiskRun()).To(Succeed())

			Expect(runner.MatchMilestones([][]string{
				{"mksquashfs", "/tmp/test/build/recovery.img.root", "/tmp/test/build/recovery/boot/recovery.img"},
				{"mkfs.ext4", "-L", "COS_STATE"},
				{"losetup", "--show", "-f", "/tmp/test/build/state.part"},
				{"mkfs.vfat", "-n", "COS_GRUB"},
				{"mkfs.ext4", "-L", "COS_OEM"},
				{"losetup", "--show", "-f", "/tmp/test/build/oem.part"},
				{"mkfs.ext4", "-L", "COS_RECOVERY"},
				{"losetup", "--show", "-f", "/tmp/test/build/recovery.part"},
				{"mkfs.ext4", "-L", "COS_PERSISTENT"},
				{"losetup", "--show", "-f", "/tmp/test/build/persistent.part"},
				{"sgdisk", "-p", "-v", "/tmp/test/elemental.raw"},
				{"partx", "-u", "/tmp/test/elemental.raw"},
			})).To(Succeed())
		})
		It("Successfully builds an expandable disk", func() {
			disk.Expandable = true

			buildDisk, err := action.NewBuildDiskAction(cfg, disk, action.WithDiskBootloader(bootloader))
			Expect(err).NotTo(HaveOccurred())
			// Expandable setup can be executed in unprivileged envs,
			// hence it should not run any mount
			// test won't pass if any mount is called
			mounter.ErrorOnMount = true

			Expect(buildDisk.BuildDiskRun()).To(Succeed())

			Expect(runner.MatchMilestones([][]string{
				{"mksquashfs", "/tmp/test/build/recovery.img.root", "/tmp/test/build/recovery/boot/recovery.img"},
				{"mkfs.vfat", "-n", "COS_GRUB"},
				{"mkfs.ext4", "-L", "COS_OEM"},
				{"mkfs.ext4", "-L", "COS_RECOVERY"},
				{"sgdisk", "-p", "-v", "/tmp/test/elemental.raw"},
				{"partx", "-u", "/tmp/test/elemental.raw"},
			})).To(Succeed())
		})
		It("Fails to build an expandable disk if expandable cloud config cannot be written", func() {
			disk.Expandable = true
			buildDisk, err := action.NewBuildDiskAction(cfg, disk, action.WithDiskBootloader(bootloader))
			Expect(err).NotTo(HaveOccurred())

			// fails to render the expandable cloud-config data
			cloudInit.RenderErr = true

			Expect(buildDisk.BuildDiskRun()).NotTo(Succeed())

			Expect(runner.MatchMilestones([][]string{
				{"mksquashfs", "/tmp/test/build/recovery.img.root", "/tmp/test/build/recovery/boot/recovery.img"},
			})).To(Succeed())

			// failed before preparing partitions images
			Expect(runner.MatchMilestones([][]string{
				{"mkfs.vfat", "-n", "COS_GRUB"},
			})).NotTo(Succeed())
		})
		It("Transforms raw image into GCE image", Label("gce"), func() {
			tmpDir, err := utils.TempDir(fs, "", "")
			defer fs.RemoveAll(tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
			f, err := fs.Create(filepath.Join(tmpDir, "disk.raw"))
			Expect(err).ToNot(HaveOccurred())
			// Set a non rounded size
			f.Truncate(34 * 1024 * 1024)
			f.Close()
			err = action.Raw2Gce(filepath.Join(tmpDir, "disk.raw"), fs, logger, false)
			Expect(err).ToNot(HaveOccurred())
			// Log should have the rounded size (1Gb)
			Expect(memLog.String()).To(ContainSubstring(strconv.Itoa(1 * 1024 * 1024 * 1024)))
			// Should be a tar file
			//realPath, _ := fs.RawPath(tmpDir)
			//Expect(dockerArchive.IsArchivePath(filepath.Join(realPath, "disk.raw.tar.gz"))).To(BeTrue())
		})
		It("Transforms raw image into Azure image", func() {
			tmpDir, err := utils.TempDir(fs, "", "")
			defer fs.RemoveAll(tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
			f, err := fs.Create(filepath.Join(tmpDir, "disk.raw"))
			Expect(err).ToNot(HaveOccurred())
			// write something
			_ = f.Truncate(23 * 1024 * 1024)
			_ = f.Close()
			err = action.Raw2Azure(filepath.Join(tmpDir, "disk.raw"), fs, logger, true)
			Expect(err).ToNot(HaveOccurred())
			info, err := fs.Stat(filepath.Join(tmpDir, "disk.raw.vhd"))
			Expect(err).ToNot(HaveOccurred())
			// Should have be rounded up to the next MB
			Expect(info.Size()).To(BeNumerically("==", 23*1024*1024))

			// Read the header
			f, _ = fs.OpenFile(filepath.Join(tmpDir, "disk.raw.vhd"), os.O_RDONLY, constants.FilePerm)
			info, _ = f.Stat()
			// Dump the header from the file into our VHDHeader
			buff := make([]byte, 512)
			_, _ = f.ReadAt(buff, info.Size()-512)
			_ = f.Close()

			header := utils.VHDHeader{}
			err = binary.Read(bytes.NewBuffer(buff[:]), binary.BigEndian, &header)
			Expect(err).ToNot(HaveOccurred())
			// Just check the fields that we know the value of, that should indicate that the header is valid
			Expect(hex.EncodeToString(header.DiskType[:])).To(Equal("00000002"))
			Expect(hex.EncodeToString(header.Features[:])).To(Equal("00000002"))
			Expect(hex.EncodeToString(header.DataOffset[:])).To(Equal("ffffffffffffffff"))
		})
		It("Transforms raw image into Azure image (tiny image)", func() {
			// This tests that the resize works for tiny images
			// Not sure if we ever will encounter them (less than 1 Mb images?) but just in case
			tmpDir, err := utils.TempDir(fs, "", "")
			defer fs.RemoveAll(tmpDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
			f, err := fs.Create(filepath.Join(tmpDir, "disk.raw"))
			Expect(err).ToNot(HaveOccurred())
			// write something
			_, _ = f.WriteString("Hi")
			_ = f.Close()
			err = action.Raw2Azure(filepath.Join(tmpDir, "disk.raw"), fs, logger, true)
			Expect(err).ToNot(HaveOccurred())
			info, err := fs.Stat(filepath.Join(tmpDir, "disk.raw"))
			Expect(err).ToNot(HaveOccurred())
			// Should be smaller than 1Mb
			Expect(info.Size()).To(BeNumerically("<", 1*1024*1024))

			info, err = fs.Stat(filepath.Join(tmpDir, "disk.raw.vhd"))
			Expect(err).ToNot(HaveOccurred())
			// Should have be rounded up to the next MB
			Expect(info.Size()).To(BeNumerically("==", 1*1024*1024))

			// Read the header
			f, _ = fs.OpenFile(filepath.Join(tmpDir, "disk.raw.vhd"), os.O_RDONLY, constants.FilePerm)
			info, _ = f.Stat()
			// Dump the header from the file into our VHDHeader
			buff := make([]byte, 512)
			_, _ = f.ReadAt(buff, info.Size()-512)
			_ = f.Close()

			header := utils.VHDHeader{}
			err = binary.Read(bytes.NewBuffer(buff[:]), binary.BigEndian, &header)
			Expect(err).ToNot(HaveOccurred())
			// Just check the fields that we know the value of, that should indicate that the header is valid
			Expect(hex.EncodeToString(header.DiskType[:])).To(Equal("00000002"))
			Expect(hex.EncodeToString(header.Features[:])).To(Equal("00000002"))
			Expect(hex.EncodeToString(header.DataOffset[:])).To(Equal("ffffffffffffffff"))
		})
	})
})
