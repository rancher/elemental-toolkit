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

package bootloader_test

import (
	"bytes"
	"fmt"
	"path/filepath"

	efi "github.com/canonical/go-efilib"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/elemental-toolkit/cmd"
	"github.com/rancher/elemental-toolkit/pkg/bootloader"
	"github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	eleefi "github.com/rancher/elemental-toolkit/pkg/efi"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
)

var _ = Describe("Booloader", Label("bootloader", "grub"), func() {
	var logger v1.Logger
	var fs vfs.FS
	var runner *v1mock.FakeRunner
	var cleanup func()
	var err error
	var grub *bootloader.Grub
	var cfg *v1.Config
	var rootDir, bootDir, efiDir string
	var grubCfg, osRelease []byte
	var efivars eleefi.Variables
	var relativeTo string

	BeforeEach(func() {
		logger = v1.NewNullLogger()
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).Should(BeNil())
		runner = v1mock.NewFakeRunner()
		grubCfg = []byte("grub configuration")
		osRelease = []byte("GRUB_ENTRY_NAME=some-name")

		// Ensure this tests do not run with privileges
		Expect(cmd.CheckRoot()).NotTo(Succeed())

		// EFI directory
		efiDir = "/some/efi/directory"
		Expect(utils.MkdirAll(fs, efiDir, constants.DirPerm)).To(Succeed())

		// Root tree
		rootDir = "/some/working/directory"
		Expect(utils.MkdirAll(fs, rootDir, constants.DirPerm)).To(Succeed())

		// Boot directory
		bootDir = "/some/other/working/directory"
		Expect(utils.MkdirAll(fs, rootDir, constants.DirPerm)).To(Succeed())

		// Efi binaries
		Expect(utils.MkdirAll(fs, filepath.Join(rootDir, "/usr/share/efi/x86_64/"), constants.DirPerm)).To(Succeed())
		Expect(fs.WriteFile(filepath.Join(rootDir, "/usr/share/efi/x86_64/shim.efi"), []byte(""), constants.FilePerm)).To(Succeed())
		Expect(fs.WriteFile(filepath.Join(rootDir, "/usr/share/efi/x86_64/MokManager.efi"), []byte(""), constants.FilePerm)).To(Succeed())

		// Grub Modules
		Expect(utils.MkdirAll(fs, filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi"), constants.DirPerm)).To(Succeed())
		Expect(fs.WriteFile(filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi/grub.efi"), []byte(""), constants.FilePerm)).To(Succeed())
		Expect(fs.WriteFile(filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi/loopback.mod"), []byte(""), constants.FilePerm)).To(Succeed())
		Expect(fs.WriteFile(filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi/squash4.mod"), []byte(""), constants.FilePerm)).To(Succeed())
		Expect(fs.WriteFile(filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi/xzio.mod"), []byte(""), constants.FilePerm)).To(Succeed())

		// os-release file
		Expect(utils.MkdirAll(fs, filepath.Join(rootDir, "/etc"), constants.DirPerm)).To(Succeed())
		Expect(fs.WriteFile(filepath.Join(rootDir, "/etc/os-release"), osRelease, constants.FilePerm)).To(Succeed())

		// Grub config file
		Expect(utils.MkdirAll(fs, filepath.Join(rootDir, constants.GrubCfgPath), constants.DirPerm)).To(Succeed())
		Expect(fs.WriteFile(filepath.Join(rootDir, constants.GrubCfgPath, constants.GrubCfg), grubCfg, constants.FilePerm)).To(Succeed())

		// EFI vars to test bootmanager
		efivars = &eleefi.MockEFIVariables{}
		err := fs.Mkdir("/EFI", constants.DirPerm)
		Expect(err).ToNot(HaveOccurred())
		err = fs.WriteFile("/EFI/test.efi", []byte(""), constants.FilePerm)
		Expect(err).ToNot(HaveOccurred())
		relativeTo, _ = fs.RawPath("/EFI")

		cfg = config.NewConfig(
			config.WithLogger(logger),
			config.WithRunner(runner),
			config.WithFs(fs),
		)
	})

	It("installs without errors", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.Install(rootDir, bootDir, "DEVICE_LABEL")).To(Succeed())

		// Check everything is copied in boot directory
		data, err := fs.ReadFile(fmt.Sprintf("%s/grub2/grub.cfg", bootDir))
		Expect(err).To(BeNil())
		Expect(data).To(Equal(grubCfg))
		_, err = fs.Stat(fmt.Sprintf("%s/grub2/x86_64-efi/loopback.mod", bootDir))
		Expect(err).To(BeNil())
		_, err = fs.Stat(fmt.Sprintf("%s/grub2/x86_64-efi/xzio.mod", bootDir))
		Expect(err).To(BeNil())
		_, err = fs.Stat(fmt.Sprintf("%s/grub2/x86_64-efi/squash4.mod", bootDir))
		Expect(err).To(BeNil())

		// Check everything is copied in EFI directory
		_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/BOOT/MokManager.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/BOOT/grub.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/BOOT/bootx64.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/ELEMENTAL/shim.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/ELEMENTAL/MokManager.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(constants.EfiDir, "EFI/ELEMENTAL/grub.efi"))
		Expect(err).To(BeNil())
	})

	It("fails to install if squash4.mod is missing", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(fs.Remove(filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi/squash4.mod"))).To(Succeed())
		Expect(grub.Install(rootDir, bootDir, "DEVICE_LABEL")).ToNot(Succeed())
	})

	It("fails to install if it can't write efi boot entry", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(false), bootloader.WithGrubClearBootEntry(false))
		Expect(grub.Install(rootDir, bootDir, "DEVICE_LABEL")).ToNot(Succeed())
	})

	It("fails to install if it can't clear efi boot entries", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(false), bootloader.WithGrubClearBootEntry(true))
		Expect(grub.Install(rootDir, bootDir, "DEVICE_LABEL")).ToNot(Succeed())
	})

	It("fails to install if grub.cfg is missing", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(fs.Remove(filepath.Join(rootDir, constants.GrubCfgPath, constants.GrubCfg))).To(Succeed())
		Expect(grub.Install(rootDir, bootDir, "DEVICE_LABEL")).ToNot(Succeed())
	})

	It("installs grub.cfg without errors", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.InstallConfig(rootDir, bootDir)).To(Succeed())

		// Check everything is copied in boot directory
		data, err := fs.ReadFile(fmt.Sprintf("%s/grub2/grub.cfg", bootDir))
		Expect(err).To(BeNil())
		Expect(data).To(Equal(grubCfg))
	})

	It("fails to install grub.cfg without write permissions", func() {
		cfg.Fs = vfs.NewReadOnlyFS(fs)
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.InstallConfig(rootDir, bootDir)).NotTo(Succeed())
	})

	It("fails to install grub.cfg if the file is missing", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(fs.Remove(filepath.Join(rootDir, constants.GrubCfgPath, constants.GrubCfg))).To(Succeed())
		Expect(grub.InstallConfig(rootDir, bootDir)).NotTo(Succeed())
	})

	It("installs EFI binaries without errors", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.InstallEFI(rootDir, bootDir, efiDir, "DEVICE_LABEL")).To(Succeed())

		// Check everything is copied in boot directory
		_, err = fs.Stat(fmt.Sprintf("%s/grub2/x86_64-efi/loopback.mod", bootDir))
		Expect(err).To(BeNil())
		_, err = fs.Stat(fmt.Sprintf("%s/grub2/x86_64-efi/xzio.mod", bootDir))
		Expect(err).To(BeNil())
		_, err = fs.Stat(fmt.Sprintf("%s/grub2/x86_64-efi/squash4.mod", bootDir))
		Expect(err).To(BeNil())

		// Check everything is copied in EFI directory
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/MokManager.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/grub.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/bootx64.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/ELEMENTAL/shim.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/ELEMENTAL/MokManager.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/ELEMENTAL/grub.efi"))
		Expect(err).To(BeNil())
	})

	It("fails to install EFI binaries if some module is missing", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(fs.Remove(filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi/xzio.mod"))).To(Succeed())
		Expect(grub.InstallEFI(rootDir, bootDir, efiDir, "DEVICE_LABEL")).NotTo(Succeed())
	})

	It("fails to install EFI binaries without write permission", func() {
		cfg.Fs = vfs.NewReadOnlyFS(fs)
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.InstallEFI(rootDir, bootDir, efiDir, "DEVICE_LABEL")).NotTo(Succeed())
	})

	It("fails to install EFI binaries if efi image is not found", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(fs.Remove(filepath.Join(rootDir, "/usr/share/grub2/x86_64-efi/grub.efi"))).To(Succeed())
		Expect(grub.InstallEFI(rootDir, bootDir, efiDir, "DEVICE_LABEL")).NotTo(Succeed())
	})

	It("fails to install EFI binaries if shim image is not found", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(fs.Remove(filepath.Join(rootDir, "/usr/share/efi/x86_64/shim.efi"))).To(Succeed())
		Expect(grub.InstallEFI(rootDir, bootDir, efiDir, "DEVICE_LABEL")).NotTo(Succeed())
	})

	It("fails to install EFI binaries if mok not found", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(fs.Remove(filepath.Join(rootDir, "/usr/share/efi/x86_64/MokManager.efi"))).To(Succeed())
		Expect(grub.InstallEFI(rootDir, bootDir, efiDir, "DEVICE_LABEL")).NotTo(Succeed())
	})

	It("installs EFI fallback binaries without errors", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.InstallEFIFallbackBinaries(rootDir, efiDir, "DEVICE_LABEL")).To(Succeed())

		// Check everything is copied in EFI directory
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/MokManager.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/grub.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/bootx64.efi"))
		Expect(err).To(BeNil())
		// Elemental entry is not installed, just fallback
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/ELEMENTAL"))
		Expect(err).NotTo(BeNil())
	})

	It("installs EFI fallback binaries without errors for arm", func() {
		cfg.Platform.Arch = "arm64"
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.InstallEFIFallbackBinaries(rootDir, efiDir, "DEVICE_LABEL")).To(Succeed())

		// Check everything is copied in EFI directory
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/MokManager.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/grub.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT/bootaa64.efi"))
		Expect(err).To(BeNil())
		// Elemental entry is not installed, just fallback
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/ELEMENTAL"))
		Expect(err).NotTo(BeNil())
	})

	It("fails to install EFI fallback binaries for an unsupported platform", func() {
		cfg.Platform.Arch = "exotic-arch"
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.InstallEFIFallbackBinaries(rootDir, efiDir, "DEVICE_LABEL")).NotTo(Succeed())
	})

	It("installs EFI Elemental binaries without errors", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.InstallEFIElementalBinaries(rootDir, efiDir, "DEVICE_LABEL")).To(Succeed())

		// Check everything is copied in EFI directory
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/ELEMENTAL/MokManager.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/ELEMENTAL/grub.efi"))
		Expect(err).To(BeNil())
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/ELEMENTAL/shim.efi"))
		Expect(err).To(BeNil())
		// Fallback entry is not installed, just the elemental one
		_, err = fs.Stat(filepath.Join(efiDir, "EFI/BOOT"))
		Expect(err).NotTo(BeNil())
	})

	It("fails to install if it can't write efi boot entry", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(false), bootloader.WithGrubClearBootEntry(false))
		Expect(grub.DoEFIEntries("shim.efi", efiDir)).NotTo(Succeed())
	})

	It("fails to install if it can't clear efi boot entries", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(false), bootloader.WithGrubClearBootEntry(true))
		Expect(grub.DoEFIEntries("shim.efi", efiDir)).NotTo(Succeed())
	})

	It("Sets the grub environment file", func() {
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.SetPersistentVariables(
			"somefile", map[string]string{"key1": "value1", "key2": "value2"},
		)).To(BeNil())
		Expect(runner.IncludesCmds([][]string{
			{"grub2-editenv", "somefile", "set", "key1=value1"},
			{"grub2-editenv", "somefile", "set", "key2=value2"},
		})).To(BeNil())
	})

	It("Fails running grub2-editenv", func() {
		runner.ReturnError = fmt.Errorf("grub error")
		grub = bootloader.NewGrub(cfg, bootloader.WithGrubDisableBootEntry(true))
		Expect(grub.SetPersistentVariables(
			"somefile", map[string]string{"key1": "value1"},
		)).NotTo(BeNil())
		Expect(runner.CmdsMatch([][]string{
			{"grub2-editenv", "somefile", "set", "key1=value1"},
		})).To(BeNil())
	})

	It("Sets the proper entry", func() {
		// We need to pass the relative path because bootmanager works on real paths
		grub = bootloader.NewGrub(cfg)
		err := grub.CreateEntry("test.efi", relativeTo, efivars)
		Expect(err).ToNot(HaveOccurred())
		vars, _ := efivars.ListVariables()
		// Only one entry should have been created
		// Second one is the BootOrder!
		Expect(len(vars)).To(Equal(2))
		// Load the options and check that its correct
		variable, _, err := efivars.GetVariable(vars[0].GUID, "Boot0000")
		option, err := efi.ReadLoadOption(bytes.NewReader(variable))
		Expect(err).ToNot(HaveOccurred())
		Expect(option.Description).To(Equal("elemental-shim"))
		Expect(option.FilePath).To(ContainSubstring("test.efi"))
		Expect(option.FilePath.String()).To(ContainSubstring(`\EFI\test.efi`))
	})
	It("Does not duplicate if an entry exists", func() {
		// We need to pass the relative path because bootmanager works on real paths
		grub = bootloader.NewGrub(cfg)
		err := grub.CreateEntry("test.efi", relativeTo, efivars)
		Expect(err).ToNot(HaveOccurred())
		vars, _ := efivars.ListVariables()
		// Only one entry should have been created
		// Second one is the BootOrder!
		Expect(len(vars)).To(Equal(2))
		// Load the options and check that its correct
		variable, _, err := efivars.GetVariable(vars[0].GUID, "Boot0000")
		option, err := efi.ReadLoadOption(bytes.NewReader(variable))
		Expect(err).ToNot(HaveOccurred())
		Expect(option.Description).To(Equal("elemental-shim"))
		Expect(option.FilePath).To(ContainSubstring("test.efi"))
		Expect(option.FilePath.String()).To(ContainSubstring(`\EFI\test.efi`))
		// And here we go again
		err = grub.CreateEntry("test.efi", relativeTo, efivars)
		// Reload vars!
		vars, _ = efivars.ListVariables()
		Expect(err).ToNot(HaveOccurred())
		Expect(len(vars)).To(Equal(2))
	})
	It("Creates a new one if the path changes", func() {
		err := fs.WriteFile("/EFI/test1.efi", []byte(""), constants.FilePerm)
		Expect(err).ToNot(HaveOccurred())
		// We need to pass the relative path because bootmanager works on real paths
		grub = bootloader.NewGrub(cfg)
		err = grub.CreateEntry("test.efi", relativeTo, efivars)
		Expect(err).ToNot(HaveOccurred())
		vars, _ := efivars.ListVariables()
		// Only one entry should have been created
		// Second one is the BootOrder!
		Expect(len(vars)).To(Equal(2))
		// Load the options and check that its correct
		variable, _, err := efivars.GetVariable(vars[0].GUID, "Boot0000")
		option, err := efi.ReadLoadOption(bytes.NewReader(variable))
		Expect(err).ToNot(HaveOccurred())
		Expect(option.Description).To(Equal("elemental-shim"))
		Expect(option.FilePath).To(ContainSubstring("test.efi"))
		Expect(option.FilePath.String()).To(ContainSubstring(`\EFI\test.efi`))

		// And here we go again
		err = grub.CreateEntry("test1.efi", relativeTo, efivars)
		Expect(err).ToNot(HaveOccurred())
		// Reload vars!
		vars, _ = efivars.ListVariables()
		Expect(len(vars)).To(Equal(3))
		// As this is the second entry generated its name is Boot0001
		variable, _, err = efivars.GetVariable(vars[0].GUID, "Boot0001")
		option, err = efi.ReadLoadOption(bytes.NewReader(variable))
		Expect(err).ToNot(HaveOccurred())
		Expect(option.Description).To(Equal("elemental-shim"))
		Expect(option.FilePath).To(ContainSubstring("test1.efi"))
		Expect(option.FilePath.String()).To(ContainSubstring(`\EFI\test1.efi`))
	})

	It("Sets default grub menu entry name from the os-release file", func() {
		grub = bootloader.NewGrub(cfg)
		Expect(grub.SetDefaultEntry(bootDir, rootDir, "")).To(Succeed())
		Expect(runner.CmdsMatch([][]string{
			{"grub2-editenv", filepath.Join(bootDir, constants.GrubOEMEnv), "set", "default_menu_entry=some-name"},
		})).To(BeNil())
	})

	It("Sets default grub menu entry name from the os-release file despite providing a default value", func() {
		grub = bootloader.NewGrub(cfg)
		Expect(grub.SetDefaultEntry(bootDir, rootDir, "this.is.ignored")).To(Succeed())
		Expect(runner.CmdsMatch([][]string{
			{"grub2-editenv", filepath.Join(bootDir, constants.GrubOEMEnv), "set", "default_menu_entry=some-name"},
		})).To(BeNil())
	})

	It("Sets default grub menu entry name to the given value if other value in os-release file is found", func() {
		Expect(fs.Remove(filepath.Join(rootDir, "/etc/os-release"))).To(Succeed())
		grub = bootloader.NewGrub(cfg)
		Expect(grub.SetDefaultEntry(bootDir, rootDir, "given-value")).To(Succeed())
		Expect(runner.CmdsMatch([][]string{
			{"grub2-editenv", filepath.Join(bootDir, constants.GrubOEMEnv), "set", "default_menu_entry=given-value"},
		})).To(BeNil())
	})

	It("Does nothing if no value is provided and the os-release file does not contain any", func() {
		Expect(fs.Remove(filepath.Join(rootDir, "/etc/os-release"))).To(Succeed())
		grub = bootloader.NewGrub(cfg)
		Expect(grub.SetDefaultEntry(bootDir, rootDir, "")).To(Succeed())
		Expect(runner.CmdsMatch([][]string{})).To(BeNil())
	})

	AfterEach(func() {
		cleanup()
	})
})
