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

package snapshotter_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	conf "github.com/rancher/elemental-toolkit/pkg/config"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1mock "github.com/rancher/elemental-toolkit/pkg/mocks"
	"github.com/rancher/elemental-toolkit/pkg/snapshotter"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
	"github.com/twpayne/go-vfs/v4"
	"github.com/twpayne/go-vfs/v4/vfst"
)

var _ = Describe("Btrfs", Label("snapshotter", " btrfs"), func() {
	var cfg v1.Config
	var runner *v1mock.FakeRunner
	var fs vfs.FS
	var logger v1.Logger
	var mounter *v1mock.FakeMounter
	var cleanup func()
	var bootloader *v1mock.FakeBootloader
	var memLog *bytes.Buffer
	var snapCfg v1.SnapshotterConfig
	var rootDir string

	BeforeEach(func() {
		rootDir = "/some/root"
		runner = v1mock.NewFakeRunner()
		mounter = v1mock.NewFakeMounter()
		bootloader = &v1mock.FakeBootloader{}
		memLog = bytes.NewBuffer(nil)
		logger = v1.NewBufferLogger(memLog)
		logger.SetLevel(v1.DebugLevel())

		var err error
		fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{})
		Expect(err).Should(BeNil())

		cfg = *conf.NewConfig(
			conf.WithFs(fs),
			conf.WithRunner(runner),
			conf.WithLogger(logger),
			conf.WithMounter(mounter),
			conf.WithPlatform("linux/amd64"),
		)
		snapCfg = v1.SnapshotterConfig{
			Type:     constants.BtrfsSnapshotterType,
			MaxSnaps: 4,
		}

		Expect(utils.MkdirAll(fs, rootDir, constants.DirPerm)).To(Succeed())
	})

	AfterEach(func() {
		cleanup()
	})

	It("creates a new Btrfs snapshotter instance", func() {
		Expect(snapshotter.NewSnapshotter(cfg, snapCfg, bootloader)).Error().NotTo(HaveOccurred())

		snapCfg.Type = "invalid"
		Expect(snapshotter.NewSnapshotter(cfg, snapCfg, bootloader)).Error().To(HaveOccurred())

		snapCfg.Type = constants.BtrfsSnapshotterType
		snapCfg.Config = map[string]string{"nonsense": "setup"}
		Expect(snapshotter.NewSnapshotter(cfg, snapCfg, bootloader)).Error().To(HaveOccurred())
	})

	Describe("Running transaction", func() {
		var b v1.Snapshotter
		var err error

		BeforeEach(func() {
			b, err = snapshotter.NewSnapshotter(cfg, snapCfg, bootloader)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Running transaction on a clean install", func() {
			BeforeEach(func() {
				runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
					switch cmd {
					case "findmnt":
						return []byte("/dev/sda"), nil
					default:
						return []byte{}, nil
					}
				}

				Expect(b.InitSnapshotter(rootDir)).To(Succeed())
				Expect(runner.MatchMilestones([][]string{
					{"btrfs", "subvolume", "list"},
					{"btrfs", "quota", "enable"},
					{"btrfs", "subvolume", "create"},
					{"btrfs", "subvolume", "create"},
					{"btrfs", "qgroup", "create"},
				})).To(Succeed())
			})

			Describe("Closing a transaction on a clean install", func() {
				var snap *v1.Snapshot
				BeforeEach(func() {
					snap, err = b.StartTransaction()
					Expect(err).NotTo(HaveOccurred())
					Expect(snap.InProgress).To(BeTrue())
					Expect(runner.MatchMilestones([][]string{
						{"btrfs", "subvolume", "create", "/some/root/.snapshots/1/snapshot"},
					})).To(Succeed())

					defaultTmpl := filepath.Join(snap.Path, "/etc/snapper/config-templates/default")
					Expect(utils.MkdirAll(fs, filepath.Dir(defaultTmpl), constants.DirPerm)).To(Succeed())
					Expect(fs.WriteFile(defaultTmpl, []byte{}, constants.FilePerm)).To(Succeed())

					snapperSysconfig := filepath.Join(snap.Path, "/etc/sysconfig/snapper")
					Expect(utils.MkdirAll(fs, filepath.Dir(snapperSysconfig), constants.DirPerm)).To(Succeed())
					Expect(fs.WriteFile(snapperSysconfig, []byte{}, constants.FilePerm)).To(Succeed())

					snapperCfg := filepath.Join(snap.Path, "/etc/snapper/configs")
					Expect(utils.MkdirAll(fs, snapperCfg, constants.DirPerm)).To(Succeed())
				})

				It("successfully closes a transaction on a clean install", func() {
					runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
						fullCmd := strings.Join(append([]string{cmd}, args...), " ")
						if strings.HasPrefix(fullCmd, "btrfs subvolume list") {
							return []byte("ID 259 gen 13453 top level 259 path @/.snapshots/1/snapshot\n"), nil
						}
						return []byte{}, nil
					}

					err = b.CloseTransaction(snap)
					Expect(err).NotTo(HaveOccurred())
					Expect(runner.MatchMilestones([][]string{
						{"btrfs", "subvolume", "set-default"},
					})).To(Succeed())
				})

				Describe("failures closing a transaction on a clean install", func() {
					var failCmd string
					BeforeEach(func() {
						runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
							fullCmd := strings.Join(append([]string{cmd}, args...), " ")
							if strings.HasPrefix(fullCmd, failCmd) {
								return []byte{}, fmt.Errorf("command '%s' failed", failCmd)
							} else if strings.HasPrefix(fullCmd, "btrfs subvolume list") {
								return []byte("ID 259 gen 13453 top level 259 path @/.snapshots/1/snapshot\n"), nil
							}
							return []byte{}, nil
						}
					})

					It("fails on missing snapper config", func() {
						failCmd = "nofailure"
						Expect(fs.Remove(filepath.Join(snap.Path, "/etc/snapper/config-templates/default"))).To(Succeed())
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring("failed to find"))
					})

					It("fails setting a ro subvolume", func() {
						failCmd = "btrfs property set"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
					})

					It("fails listing subvolumes", func() {
						failCmd = "btrfs subvolume list"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
					})

					It("fails setting default subvolume", func() {
						failCmd = "btrfs subvolume set-default"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
					})
				})
			})

			It("fails to start a transaction on a fresh install", func() {
				runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
					fullCmd := strings.Join(append([]string{cmd}, args...), " ")
					if strings.HasPrefix(fullCmd, "btrfs subvolume create /some/root/.snapshots/1/snapshot") {
						return []byte{}, fmt.Errorf("failed creating subvolume")
					}
					return []byte{}, nil
				}
				_, err = b.StartTransaction()

				Expect(err.Error()).To(ContainSubstring("failed creating subvolume"))
			})
		})

		Describe("failures to initate a snapshotter on a clean install", func() {
			var failCmd string
			BeforeEach(func() {
				runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
					fullCmd := strings.Join(append([]string{cmd}, args...), " ")
					switch {
					case strings.HasPrefix(fullCmd, failCmd):
						return []byte{}, fmt.Errorf("command '%s' failed", failCmd)
					case cmd == "findmnt":
						return []byte("/dev/sda"), nil
					default:
						return []byte{}, nil
					}
				}
			})

			It("fails to find root device", func() {
				failCmd = "findmnt"
				err = b.InitSnapshotter(rootDir)
				Expect(err.Error()).To(ContainSubstring(failCmd))
			})

			It("fails to to list subvolumes", func() {
				failCmd = "btrfs subvolume list"
				err = b.InitSnapshotter(rootDir)
				Expect(err.Error()).To(ContainSubstring(failCmd))
			})

			It("fails to enable btrfs quota", func() {
				failCmd = "btrfs quota enable"
				err = b.InitSnapshotter(rootDir)
				Expect(err.Error()).To(ContainSubstring(failCmd))
			})

			It("fails to create subvolume", func() {
				failCmd = "btrfs subvolume create"
				err = b.InitSnapshotter(rootDir)
				Expect(err.Error()).To(ContainSubstring(failCmd))
			})

			It("fails to create quota group", func() {
				failCmd = "btrfs qgroup create"
				err = b.InitSnapshotter(rootDir)
				Expect(err.Error()).To(ContainSubstring(failCmd))
			})
		})

		Describe("Running transaction on a recovery system", func() {
			BeforeEach(func() {
				defaultVol := "ID 259 gen 13453 top level 258 path @/.snapshots/1/snapshot\n"
				volumesList := "ID 257 gen 13451 top level 3 path @\n"
				volumesList += "ID 258 gen 13452 top level 257 path @/.snapshots\n"
				volumesList += defaultVol
				runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
					fullCmd := strings.Join(append([]string{cmd}, args...), " ")
					switch {
					case strings.HasPrefix(fullCmd, "btrfs subvolume list"):
						return []byte(volumesList), nil
					case strings.HasPrefix(fullCmd, "btrfs subvolume get-default"):
						return []byte(defaultVol), nil
					case cmd == "findmnt":
						return []byte("/dev/sda[/@/.snapshots/1/snapshot]"), nil
					default:
						return []byte{}, nil
					}
				}

				Expect(b.InitSnapshotter(rootDir)).To(Succeed())
				Expect(runner.MatchMilestones([][]string{
					{"btrfs", "subvolume", "list"},
					{"btrfs", "subvolume", "get-default"},
				})).To(Succeed())
			})

			Describe("Closing a transaction on a recovery system", func() {
				var snap *v1.Snapshot
				BeforeEach(func() {
					runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
						fullCmd := strings.Join(append([]string{cmd}, args...), " ")
						if strings.HasPrefix(fullCmd, "snapper --no-dbus --root /some/root/.snapshots/1/snapshot create") {
							return []byte("2\n"), nil
						}
						return []byte{}, nil
					}

					snap, err = b.StartTransaction()
					Expect(err).NotTo(HaveOccurred())
					Expect(snap.InProgress).To(BeTrue())
					Expect(runner.MatchMilestones([][]string{
						{"snapper", "--no-dbus", "--root", "/some/root/.snapshots/1/snapshot", "create"},
					})).To(Succeed())

					defaultTmpl := filepath.Join(snap.Path, "/etc/snapper/config-templates/default")
					Expect(utils.MkdirAll(fs, filepath.Dir(defaultTmpl), constants.DirPerm)).To(Succeed())
					Expect(fs.WriteFile(defaultTmpl, []byte{}, constants.FilePerm)).To(Succeed())

					snapperSysconfig := filepath.Join(snap.Path, "/etc/sysconfig/snapper")
					Expect(utils.MkdirAll(fs, filepath.Dir(snapperSysconfig), constants.DirPerm)).To(Succeed())
					Expect(fs.WriteFile(snapperSysconfig, []byte{}, constants.FilePerm)).To(Succeed())

					snapperCfg := filepath.Join(snap.Path, "/etc/snapper/configs")
					Expect(utils.MkdirAll(fs, snapperCfg, constants.DirPerm)).To(Succeed())
				})

				It("successfully closes a transaction on a recovery system", func() {
					runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
						fullCmd := strings.Join(append([]string{cmd}, args...), " ")
						if strings.HasPrefix(fullCmd, "snapper --csvout list") {
							return []byte("1,yes,no\n2,no,no\n"), nil
						} else if strings.HasPrefix(fullCmd, "btrfs subvolume list") {
							return []byte("ID 260 gen 13453 top level 259 path @/.snapshots/2/snapshot\n"), nil
						}
						return []byte{}, nil
					}

					Expect(b.CloseTransaction(snap)).NotTo(HaveOccurred())
					Expect(runner.MatchMilestones([][]string{
						{"snapper", "--no-dbus", "--root", "/some/root/.snapshots/1/snapshot", "cleanup"},
					})).To(Succeed())
				})

				Describe("close transaction failures on a recovery system", func() {
					var failCmd string
					BeforeEach(func() {
						runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
							fullCmd := strings.Join(append([]string{cmd}, args...), " ")
							if strings.HasPrefix(fullCmd, failCmd) {
								return []byte{}, fmt.Errorf("command '%s' failed", failCmd)
							} else if strings.HasPrefix(fullCmd, "snapper --no-dbus --root /some/root/.snapshots/1/snapshot --csvout list") {
								return []byte("1,yes,no\n2,no,no\n"), nil
							} else if strings.HasPrefix(fullCmd, "btrfs subvolume list") {
								return []byte("ID 260 gen 13453 top level 259 path @/.snapshots/2/snapshot\n"), nil
							}
							return []byte{}, nil
						}
					})

					It("fails syncing", func() {
						failCmd = "rsync"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
						Expect(runner.MatchMilestones([][]string{
							{"snapper", "--no-dbus", "--root", "/some/root/.snapshots/1/snapshot", "delete"},
						})).To(Succeed())
					})

					It("fails on snapper modify", func() {
						failCmd = "snapper --no-dbus --root /some/root/.snapshots/1/snapshot modify"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
						Expect(runner.MatchMilestones([][]string{
							{"snapper", "--no-dbus", "--root", "/some/root/.snapshots/1/snapshot", "delete"},
						})).To(Succeed())
					})

					It("fails setting snapshot read only", func() {
						failCmd = "btrfs property set"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
						Expect(runner.MatchMilestones([][]string{
							{"snapper", "--no-dbus", "--root", "/some/root/.snapshots/1/snapshot", "delete"},
						})).To(Succeed())
					})

					It("fails setting default", func() {
						failCmd = "btrfs subvolume set-default"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
						Expect(runner.MatchMilestones([][]string{
							{"snapper", "--no-dbus", "--root", "/some/root/.snapshots/1/snapshot", "delete"},
						})).To(Succeed())
					})
				})
			})

			It("fails to start a transaction on a recovery system", func() {
				runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
					fullCmd := strings.Join(append([]string{cmd}, args...), " ")
					if strings.HasPrefix(fullCmd, "snapper --no-dbus --root /some/root/.snapshots/1/snapshot create") {
						return []byte{}, fmt.Errorf("failed creating snapshot")
					}
					return []byte{}, nil
				}
				_, err = b.StartTransaction()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed creating snapshot"))
			})

		})

		Describe("failures to init an snapshotter on a recovery system", func() {
			var failCmd string
			BeforeEach(func() {
				defaultVol := "ID 259 gen 13453 top level 258 path @/.snapshots/1/snapshot\n"
				volumesList := "ID 257 gen 13451 top level 3 path @\n"
				volumesList += "ID 258 gen 13452 top level 257 path @/.snapshots\n"
				volumesList += defaultVol
				runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
					fullCmd := strings.Join(append([]string{cmd}, args...), " ")
					switch {
					case strings.HasPrefix(fullCmd, failCmd):
						return []byte{}, fmt.Errorf("command '%s' failed", failCmd)
					case strings.HasPrefix(fullCmd, "btrfs subvolume list"):
						return []byte(volumesList), nil
					case strings.HasPrefix(fullCmd, "btrfs subvolume get-default"):
						return []byte(defaultVol), nil
					case cmd == "findmnt":
						return []byte("/dev/sda[/@/.snapshots/1/snapshot]"), nil
					default:
						return []byte{}, nil
					}
				}
			})

			It("fails to get default subvolume", func() {
				failCmd = "btrfs subvolume get-default"
				err = b.InitSnapshotter(rootDir)
				Expect(err.Error()).To(ContainSubstring(failCmd))
			})

			It("fails to mount root", func() {
				failCmd = "nofail"
				mounter.ErrorOnMount = true
				err = b.InitSnapshotter(rootDir)
				Expect(err.Error()).To(ContainSubstring("mount"))
			})

			It("fails to umount default subvolume", func() {
				failCmd = "nofail"
				mounter.ErrorOnUnmount = true
				err = b.InitSnapshotter(rootDir)
				Expect(err.Error()).To(ContainSubstring("unmount"))
			})
		})

		Describe("Running transaction on an active system", func() {
			BeforeEach(func() {
				defaultVol := "ID 259 gen 13453 top level 258 path @/.snapshots/1/snapshot\n"
				volumesList := "ID 257 gen 13451 top level 3 path @\n"
				volumesList += "ID 258 gen 13452 top level 257 path @/.snapshots\n"
				volumesList += defaultVol

				Expect(utils.MkdirAll(fs, constants.RunElementalDir, constants.DirPerm)).To(Succeed())
				Expect(fs.WriteFile(constants.ActiveMode, []byte("1"), constants.FilePerm)).To(Succeed())

				runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
					fullCmd := strings.Join(append([]string{cmd}, args...), " ")
					switch {
					case strings.HasPrefix(fullCmd, "btrfs subvolume list"):
						return []byte(volumesList), nil
					case strings.HasPrefix(fullCmd, "btrfs subvolume get-default"):
						return []byte(defaultVol), nil
					case cmd == "findmnt":
						return []byte("/dev/sda[/@/.snapshots/1/snapshot]"), nil
					default:
						return []byte{}, nil
					}
				}

				Expect(b.InitSnapshotter(rootDir)).To(Succeed())
				Expect(runner.MatchMilestones([][]string{
					{"btrfs", "subvolume", "list"},
					{"btrfs", "subvolume", "get-default"},
				})).To(Succeed())
			})

			Describe("Closing a transaction on an active system", func() {
				var snap *v1.Snapshot
				BeforeEach(func() {
					runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
						fullCmd := strings.Join(append([]string{cmd}, args...), " ")
						if strings.HasPrefix(fullCmd, "snapper create --from") {
							return []byte("2\n"), nil
						}
						return []byte{}, nil
					}

					snap, err = b.StartTransaction()
					Expect(err).NotTo(HaveOccurred())
					Expect(snap.InProgress).To(BeTrue())
					Expect(runner.MatchMilestones([][]string{
						{"snapper", "create", "--from"},
					})).To(Succeed())

					defaultTmpl := filepath.Join(snap.Path, "/etc/snapper/config-templates/default")
					Expect(utils.MkdirAll(fs, filepath.Dir(defaultTmpl), constants.DirPerm)).To(Succeed())
					Expect(fs.WriteFile(defaultTmpl, []byte{}, constants.FilePerm)).To(Succeed())

					snapperSysconfig := filepath.Join(snap.Path, "/etc/sysconfig/snapper")
					Expect(utils.MkdirAll(fs, filepath.Dir(snapperSysconfig), constants.DirPerm)).To(Succeed())
					Expect(fs.WriteFile(snapperSysconfig, []byte{}, constants.FilePerm)).To(Succeed())

					snapperCfg := filepath.Join(snap.Path, "/etc/snapper/configs")
					Expect(utils.MkdirAll(fs, snapperCfg, constants.DirPerm)).To(Succeed())
				})

				It("successfully closes a transaction on an active system", func() {
					runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
						fullCmd := strings.Join(append([]string{cmd}, args...), " ")
						if strings.HasPrefix(fullCmd, "snapper --csvout list") {
							return []byte("1,no,yes\n2,yes,no\n"), nil
						} else if strings.HasPrefix(fullCmd, "btrfs subvolume list") {
							return []byte("ID 260 gen 13453 top level 259 path @/.snapshots/2/snapshot\n"), nil
						}
						return []byte{}, nil
					}

					Expect(b.CloseTransaction(snap)).NotTo(HaveOccurred())
					Expect(runner.MatchMilestones([][]string{
						{"snapper", "cleanup"},
					})).To(Succeed())
				})

				Describe("close transaction failures on an active system", func() {
					var failCmd string
					BeforeEach(func() {
						runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
							fullCmd := strings.Join(append([]string{cmd}, args...), " ")
							if strings.HasPrefix(fullCmd, failCmd) {
								return []byte{}, fmt.Errorf("command '%s' failed", failCmd)
							} else if strings.HasPrefix(fullCmd, "snapper --csvout list") {
								return []byte("1,no,yes\n2,yes,no\n"), nil
							} else if strings.HasPrefix(fullCmd, "btrfs subvolume list") {
								return []byte("ID 260 gen 13453 top level 259 path @/.snapshots/2/snapshot\n"), nil
							}
							return []byte{}, nil
						}
					})

					It("fails syncing", func() {
						failCmd = "rsync"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
						Expect(runner.MatchMilestones([][]string{{"snapper", "delete"}})).To(Succeed())
					})

					It("fails on snapper modify", func() {
						failCmd = "snapper modify"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
						Expect(runner.MatchMilestones([][]string{{"snapper", "delete"}})).To(Succeed())
					})

					It("fails setting snapshot read only", func() {
						failCmd = "btrfs property set"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
						Expect(runner.MatchMilestones([][]string{{"snapper", "delete"}})).To(Succeed())
					})

					It("fails setting default", func() {
						failCmd = "btrfs subvolume set-default"
						err = b.CloseTransaction(snap)
						Expect(err.Error()).To(ContainSubstring(failCmd))
						Expect(runner.MatchMilestones([][]string{{"snapper", "delete"}})).To(Succeed())
					})
				})
			})

			It("fails to start a transaction on an active system", func() {
				runner.SideEffect = func(cmd string, args ...string) ([]byte, error) {
					fullCmd := strings.Join(append([]string{cmd}, args...), " ")
					if strings.HasPrefix(fullCmd, "snapper create --from") {
						return []byte{}, fmt.Errorf("failed creating snapshot")
					}
					return []byte{}, nil
				}
				_, err = b.StartTransaction()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed creating snapshot"))
			})
		})
	})
})
