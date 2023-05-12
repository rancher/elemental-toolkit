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
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"github.com/rancher/elemental-cli/pkg/action"
	"github.com/rancher/elemental-cli/pkg/config"
	"github.com/rancher/elemental-cli/pkg/constants"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
	v1mock "github.com/rancher/elemental-cli/tests/mocks"
)

var _ = Describe("Runtime Actions", func() {
	var cfg *v1.BuildConfig
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var mounter *v1mock.ErrorMounter
	var syscall *v1mock.FakeSyscall
	var client *v1mock.FakeHTTPClient
	var cloudInit *v1mock.FakeCloudInitRunner
	var extractor *v1mock.FakeImageExtractor
	var cleanup func()
	var memLog *bytes.Buffer
	BeforeEach(func() {
		runner = v1mock.NewFakeRunner()
		syscall = &v1mock.FakeSyscall{}
		mounter = v1mock.NewErrorMounter()
		client = &v1mock.FakeHTTPClient{}
		memLog = &bytes.Buffer{}
		logger = v1.NewBufferLogger(memLog)
		logger.SetLevel(logrus.DebugLevel)
		extractor = v1mock.NewFakeImageExtractor(logger)
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
			config.WithImageExtractor(extractor),
			config.WithPlatform("linux/amd64"),
		)

	})
	AfterEach(func() {
		cleanup()
	})
	Describe("Build ISO", Label("iso"), func() {
		var iso *v1.LiveISO
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
			rootSrc, _ := v1.NewSrcFromURI("oci:elementalos:latest")
			iso.RootFS = []*v1.ImageSource{rootSrc}

			extractor.SideEffect = func(_, destination, platform string, _ bool) error {
				bootDir := filepath.Join(destination, "boot")
				logger.Debugf("Creating %s", bootDir)
				err := utils.MkdirAll(fs, bootDir, constants.DirPerm)
				if err != nil {
					return err
				}
				_, err = fs.Create(filepath.Join(bootDir, "vmlinuz"))
				if err != nil {
					return err
				}

				_, err = fs.Create(filepath.Join(bootDir, "initrd"))
				return err
			}

			buildISO := action.NewBuildISOAction(cfg, iso)
			err := buildISO.ISORun()

			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Successfully builds an ISO using self contained binaries and including overlayed files", func() {
			rootFs := []string{"dir:/overlay/dir"}
			for _, src := range rootFs {
				rootSrc, _ := v1.NewSrcFromURI(src)
				iso.RootFS = append(iso.RootFS, rootSrc)
			}

			err := utils.MkdirAll(fs, "/overlay/dir/boot", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create("/overlay/dir/boot/vmlinuz")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create("/overlay/dir/boot/initrd")
			Expect(err).ShouldNot(HaveOccurred())

			liveBoot := &v1mock.LiveBootLoaderMock{}
			buildISO := action.NewBuildISOAction(cfg, iso, action.WithLiveBoot(liveBoot))
			err = buildISO.ISORun()

			Expect(err).ShouldNot(HaveOccurred())
		})
		It("Fails on prepare EFI", func() {
			iso.BootloaderInRootFs = true

			rootSrc, _ := v1.NewSrcFromURI("oci:elementalos:latest")
			iso.RootFS = append(iso.RootFS, rootSrc)

			liveBoot := &v1mock.LiveBootLoaderMock{ErrorEFI: true}
			buildISO := action.NewBuildISOAction(cfg, iso, action.WithLiveBoot(liveBoot))
			err := buildISO.ISORun()

			Expect(err).Should(HaveOccurred())
		})
		It("Fails on prepare ISO", func() {
			iso.BootloaderInRootFs = true

			rootSrc, _ := v1.NewSrcFromURI("channel:system/elemental")
			iso.RootFS = append(iso.RootFS, rootSrc)

			liveBoot := &v1mock.LiveBootLoaderMock{ErrorISO: true}
			buildISO := action.NewBuildISOAction(cfg, iso, action.WithLiveBoot(liveBoot))
			err := buildISO.ISORun()

			Expect(err).Should(HaveOccurred())
		})
		It("Fails if kernel or initrd is not found in rootfs", func() {
			rootSrc, _ := v1.NewSrcFromURI("dir:/local/dir")
			iso.RootFS = []*v1.ImageSource{rootSrc}

			err := utils.MkdirAll(fs, "/local/dir/boot", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())

			By("fails without kernel")
			buildISO := action.NewBuildISOAction(cfg, iso)
			err = buildISO.ISORun()
			Expect(err).Should(HaveOccurred())

			By("fails without initrd")
			_, err = fs.Create("/local/dir/boot/vmlinuz")
			Expect(err).ShouldNot(HaveOccurred())
			buildISO = action.NewBuildISOAction(cfg, iso)
			err = buildISO.ISORun()
			Expect(err).Should(HaveOccurred())
		})
		It("Fails installing rootfs sources", func() {
			rootSrc, _ := v1.NewSrcFromURI("channel:system/elemental")
			iso.RootFS = []*v1.ImageSource{rootSrc}

			buildISO := action.NewBuildISOAction(cfg, iso)
			err := buildISO.ISORun()
			Expect(err).Should(HaveOccurred())
		})
		It("Fails installing uefi sources", func() {
			rootSrc, _ := v1.NewSrcFromURI("docker:elemental:latest")
			iso.RootFS = []*v1.ImageSource{rootSrc}
			uefiSrc, _ := v1.NewSrcFromURI("channel:live/efi")
			iso.UEFI = []*v1.ImageSource{uefiSrc}

			buildISO := action.NewBuildISOAction(cfg, iso)
			err := buildISO.ISORun()
			Expect(err).Should(HaveOccurred())
		})
		It("Fails installing image sources", func() {
			rootSrc, _ := v1.NewSrcFromURI("docker:elemental:latest")
			iso.RootFS = []*v1.ImageSource{rootSrc}
			uefiSrc, _ := v1.NewSrcFromURI("docker:registry.suse.com/custom-uefi:v0.1")
			iso.UEFI = []*v1.ImageSource{uefiSrc}

			buildISO := action.NewBuildISOAction(cfg, iso)
			err := buildISO.ISORun()
			Expect(err).Should(HaveOccurred())
		})
		It("Fails on ISO filesystem creation", func() {
			rootSrc, _ := v1.NewSrcFromURI("dir:/local/dir")
			iso.RootFS = []*v1.ImageSource{rootSrc}

			err := utils.MkdirAll(fs, "/local/dir/boot", constants.DirPerm)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create("/local/dir/boot/vmlinuz")
			Expect(err).ShouldNot(HaveOccurred())
			_, err = fs.Create("/local/dir/boot/initrd")
			Expect(err).ShouldNot(HaveOccurred())

			runner.SideEffect = func(command string, args ...string) ([]byte, error) {
				if command == "xorriso" {
					return []byte{}, errors.New("Burn ISO error")
				}
				return []byte{}, nil
			}

			buildISO := action.NewBuildISOAction(cfg, iso)
			err = buildISO.ISORun()

			Expect(err).Should(HaveOccurred())
		})
	})
})
