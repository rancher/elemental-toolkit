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

package live_test

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/live"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GreenLiveBootloader", Label("green", "live"), func() {
	var cfg *v1.BuildConfig
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var mounter *v1mock.ErrorMounter
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var cloudInit *v1mock.FakeCloudInitRunner
	var cleanup func()
	var memLog *bytes.Buffer
	var iso *v1.LiveISO
	var rootDir, imageDir, uefiDir string
	var i386BinChrootPath string
	BeforeEach(func() {
		var err error
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		logger.SetLevel(logrus.DebugLevel)
		cloudInit = &v1mock.FakeCloudInitRunner{}
		fs, cleanup, _ = vfst.NewTestFS(map[string]interface{}{})
		cfg = config.NewBuildConfig(
			config.WithFs(fs),
			config.WithRunner(runner),
			config.WithLogger(logger),
			config.WithMounter(mounter),
			config.WithSyscall(syscall),
			config.WithClient(client),
			config.WithCloudInitRunner(cloudInit),
		)
		iso = config.NewISO()

		rootDir, err = utils.TempDir(fs, "", "rootDir")
		Expect(err).ShouldNot(HaveOccurred())
		imageDir, err = utils.TempDir(fs, "", "imageDir")
		Expect(err).ShouldNot(HaveOccurred())
		uefiDir, err = utils.TempDir(fs, "", "uefiDir")
		Expect(err).ShouldNot(HaveOccurred())

		// Create mock EFI files
		err = utils.MkdirAll(fs, filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi"), constants.DirPerm)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(
			filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi/grub.efi"),
			[]byte("x86_64_efi"), constants.FilePerm,
		)
		Expect(err).ShouldNot(HaveOccurred())

		err = utils.MkdirAll(fs, filepath.Join(rootDir, "/usr/share/efi/x86_64"), constants.DirPerm)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(
			filepath.Join(rootDir, "/usr/share/efi/x86_64/shim.efi"),
			[]byte("shim"), constants.FilePerm,
		)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(
			filepath.Join(rootDir, "/usr/share/efi/x86_64/MokManager.efi"),
			[]byte("mokmanager"), constants.FilePerm,
		)
		Expect(err).ShouldNot(HaveOccurred())

		// Create mock BIOS files
		i386BinChrootPath = "/usr/share/grub2/i386-pc"
		err = utils.MkdirAll(fs, i386BinChrootPath, constants.DirPerm)
		Expect(err).ShouldNot(HaveOccurred())

		err = fs.WriteFile(filepath.Join(i386BinChrootPath, "cdboot.img"), []byte("cdboot.img"), constants.FilePerm)
		Expect(err).ShouldNot(HaveOccurred())

		i386BinPath := filepath.Join(rootDir, i386BinChrootPath)
		err = utils.MkdirAll(fs, i386BinPath, constants.DirPerm)
		Expect(err).ShouldNot(HaveOccurred())

		err = fs.WriteFile(filepath.Join(i386BinPath, "eltorito.img"), []byte("eltorito"), constants.FilePerm)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(filepath.Join(i386BinPath, "boot_hybrid.img"), []byte("boot_hybrid"), constants.FilePerm)
		Expect(err).ShouldNot(HaveOccurred())

		syslinuxPath := filepath.Join(rootDir, "/usr/share/syslinux")
		err = utils.MkdirAll(fs, syslinuxPath, constants.DirPerm)
		Expect(err).ShouldNot(HaveOccurred())

		err = fs.WriteFile(filepath.Join(syslinuxPath, "isolinux.bin"), []byte("isolinux"), constants.FilePerm)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(filepath.Join(syslinuxPath, "menu.c32"), []byte("menu"), constants.FilePerm)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(filepath.Join(syslinuxPath, "chain.c32"), []byte("chain"), constants.FilePerm)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(filepath.Join(syslinuxPath, "mboot.c32"), []byte("mboot"), constants.FilePerm)
		Expect(err).ShouldNot(HaveOccurred())
	})
	AfterEach(func() {
		cleanup()
	})
	It("Creates eltorito image", func() {
		runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
			switch cmd {
			case "grub2-mkimage":
				err := fs.WriteFile(filepath.Join(i386BinChrootPath, "core.img"), []byte("core.img"), constants.FilePerm)
				return []byte{}, err
			default:
				return []byte{}, nil
			}
		}
		green := live.NewGreenLiveBootLoader(cfg, iso)
		eltorito, err := green.BuildEltoritoImg(rootDir)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(eltorito).To(Equal("/usr/share/grub2/i386-pc/eltorito.img"))
		out, err := fs.ReadFile(eltorito)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(out)).To(Equal("cdboot.imgcore.img"))
	})
	It("Fails creating eltorito image, grub2-mkimage failure", func() {
		runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
			switch cmd {
			case "grub2-mkimage":
				return []byte{}, fmt.Errorf("failed creating core image")
			default:
				return []byte{}, nil
			}
		}
		green := live.NewGreenLiveBootLoader(cfg, iso)
		_, err := green.BuildEltoritoImg(rootDir)
		Expect(err).Should(HaveOccurred())
	})
	It("Fails creating eltorito image, concatenating files failure", func() {
		// fake runner does not create a fake core.img
		green := live.NewGreenLiveBootLoader(cfg, iso)
		_, err := green.BuildEltoritoImg(rootDir)
		Expect(err).Should(HaveOccurred())
	})
	It("Copies the EFI image binaries for x86_64", func() {
		green := live.NewGreenLiveBootLoader(cfg, iso)
		err := green.PrepareEFI(rootDir, uefiDir)
		Expect(err).ShouldNot(HaveOccurred())
		exists, _ := utils.Exists(fs, filepath.Join(uefiDir, "EFI/BOOT/grub.cfg"))
		Expect(exists).To(BeTrue())
	})
	It("Fails to copy the EFI image binaries if there is no shim", func() {
		// Missing shim image
		err := fs.RemoveAll(filepath.Join(rootDir, "/usr/share/efi/x86_64"))
		Expect(err).ShouldNot(HaveOccurred())

		green := live.NewGreenLiveBootLoader(cfg, iso)
		err = green.PrepareEFI(rootDir, uefiDir)
		Expect(err).Should(HaveOccurred())
	})
	It("Fails to copy the EFI image binaries if there is no grub", func() {
		// Missing grub image
		err := fs.RemoveAll(filepath.Join(rootDir, "/usr/share/grub2"))
		Expect(err).ShouldNot(HaveOccurred())

		green := live.NewGreenLiveBootLoader(cfg, iso)
		err = green.PrepareEFI(rootDir, uefiDir)
		Expect(err).Should(HaveOccurred())
	})
	It("Fails to copy the EFI image binaries for unsupported arch", func() {
		cfg.Platform = &v1.Platform{Arch: "unknown"}

		green := live.NewGreenLiveBootLoader(cfg, iso)
		err := green.PrepareEFI(rootDir, uefiDir)
		Expect(err).Should(HaveOccurred())
	})
	It("Copies the EFI image binaries for arm64", func() {
		platform, err := v1.NewPlatformFromArch(constants.ArchArm64)
		Expect(err).ShouldNot(HaveOccurred())
		cfg.Platform = platform
		err = utils.MkdirAll(fs, filepath.Join(rootDir, "/usr/share/grub2/arm64-efi"), constants.DirPerm)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(
			filepath.Join(rootDir, "/usr/share/grub2/arm64-efi/grub.efi"),
			[]byte("arm64-efi"), constants.FilePerm,
		)
		Expect(err).ShouldNot(HaveOccurred())

		err = utils.MkdirAll(fs, filepath.Join(rootDir, "/usr/share/efi/aarch64"), constants.DirPerm)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(
			filepath.Join(rootDir, "/usr/share/efi/aarch64/shim.efi"),
			[]byte("shim"), constants.FilePerm,
		)
		Expect(err).ShouldNot(HaveOccurred())
		err = fs.WriteFile(
			filepath.Join(rootDir, "/usr/share/efi/aarch64/MokManager.efi"),
			[]byte("mokmanager"), constants.FilePerm,
		)
		Expect(err).ShouldNot(HaveOccurred())

		green := live.NewGreenLiveBootLoader(cfg, iso)
		err = green.PrepareEFI(rootDir, uefiDir)
		Expect(err).ShouldNot(HaveOccurred())
		exists, _ := utils.Exists(fs, filepath.Join(uefiDir, "EFI/BOOT/grub.cfg"))
		Expect(exists).To(BeTrue())
	})
	It("Prepares ISO root with BIOS bootloader files", func() {
		runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
			switch cmd {
			case "grub2-mkimage":
				err := fs.WriteFile(filepath.Join(i386BinChrootPath, "core.img"), []byte("core.img"), constants.FilePerm)
				return []byte{}, err
			default:
				return []byte{}, nil
			}
		}
		iso.Firmware = v1.BIOS
		green := live.NewGreenLiveBootLoader(cfg, iso)
		err := green.PrepareISO(rootDir, imageDir)
		Expect(err).ShouldNot(HaveOccurred())

		exists, _ := utils.Exists(fs, filepath.Join(imageDir, "EFI/BOOT"))
		Expect(exists).To(BeFalse())
		exists, _ = utils.Exists(fs, filepath.Join(imageDir, "boot/grub2/grub.cfg"))
		Expect(exists).To(BeTrue())
	})
	It("Failes to prepare ISO root with BIOS bootloader building grub image", func() {
		runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
			switch cmd {
			case "grub2-mkimage":
				return []byte{}, fmt.Errorf("failed building image")
			default:
				return []byte{}, nil
			}
		}
		iso.Firmware = v1.BIOS
		green := live.NewGreenLiveBootLoader(cfg, iso)
		err := green.PrepareISO(rootDir, imageDir)
		Expect(err).Should(HaveOccurred())
	})
	It("Failes to prepare ISO root with BIOS bootloader files on missing syslinux loaders", func() {
		// Missing grub image
		err := fs.RemoveAll(filepath.Join(rootDir, "/usr/share/syslinux"))
		Expect(err).ShouldNot(HaveOccurred())

		runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
			switch cmd {
			case "grub2-mkimage":
				err := fs.WriteFile(filepath.Join(i386BinChrootPath, "core.img"), []byte("core.img"), constants.FilePerm)
				return []byte{}, err
			default:
				return []byte{}, nil
			}
		}
		iso.Firmware = v1.BIOS
		green := live.NewGreenLiveBootLoader(cfg, iso)
		err = green.PrepareISO(rootDir, imageDir)
		Expect(err).Should(HaveOccurred())
	})
	It("Prepares ISO root with EFI bootloader files", func() {
		green := live.NewGreenLiveBootLoader(cfg, iso)
		err := green.PrepareISO(rootDir, imageDir)
		Expect(err).ShouldNot(HaveOccurred())

		exists, _ := utils.Exists(fs, filepath.Join(imageDir, "EFI/BOOT/bootx64.efi"))
		Expect(exists).To(BeTrue())
		exists, _ = utils.Exists(fs, filepath.Join(imageDir, "EFI/BOOT/MokManager.efi"))
		Expect(exists).To(BeTrue())
		exists, _ = utils.Exists(fs, filepath.Join(imageDir, "boot/grub2/grub.cfg"))
		Expect(exists).To(BeTrue())
	})
})
