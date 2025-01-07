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

package action

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/rancher/elemental-toolkit/v2/pkg/bootloader"
	cnst "github.com/rancher/elemental-toolkit/v2/pkg/constants"
	"github.com/rancher/elemental-toolkit/v2/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/v2/pkg/error"
	"github.com/rancher/elemental-toolkit/v2/pkg/snapshotter"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

type InstallAction struct {
	cfg         *types.RunConfig
	spec        *types.InstallSpec
	bootloader  types.Bootloader
	snapshotter types.Snapshotter
	snapshot    *types.Snapshot
}

type InstallActionOption func(i *InstallAction) error

func WithInstallBootloader(bootloader types.Bootloader) func(i *InstallAction) error {
	return func(i *InstallAction) error {
		i.bootloader = bootloader
		return nil
	}
}

func NewInstallAction(cfg *types.RunConfig, spec *types.InstallSpec, opts ...InstallActionOption) (*InstallAction, error) {
	var err error

	i := &InstallAction{cfg: cfg, spec: spec}

	for _, o := range opts {
		err = o(i)
		if err != nil {
			cfg.Logger.Errorf("error applying config option: %s", err.Error())
			return nil, err
		}
	}

	if i.bootloader == nil {
		i.bootloader = bootloader.NewGrub(&cfg.Config,
			bootloader.WithGrubDisableBootEntry(i.spec.DisableBootEntry),
			bootloader.WithGrubAutoDisableBootEntry(),
		)
	}

	if i.snapshotter == nil {
		i.snapshotter, err = snapshotter.NewSnapshotter(cfg.Config, cfg.Snapshotter, i.bootloader)
	}

	if i.cfg.Snapshotter.Type == cnst.BtrfsSnapshotterType {
		if spec.Partitions.State.FS != cnst.Btrfs {
			cfg.Logger.Warning("Btrfs snapshotter type, forcing btrfs filesystem on state partition")
			spec.Partitions.State.FS = cnst.Btrfs
		}
	}

	return i, err
}

// installHook runs the given hook without chroot. Moreover if the hook is 'after-install'
// it appends defiled cloud init paths rooted to the deployed root. This way any
// 'after-install' hook provided by the deployed system image is also taken into account.
func (i *InstallAction) installHook(hook string) error {
	cIPaths := i.cfg.CloudInitPaths
	if hook == cnst.AfterInstallHook {
		cIPaths = append(cIPaths, utils.PreAppendRoot(cnst.WorkingImgDir, i.cfg.CloudInitPaths...)...)
	}
	return Hook(&i.cfg.Config, hook, i.cfg.Strict, cIPaths...)
}

func (i *InstallAction) installChrootHook(hook string, root string) error {
	extraMounts := map[string]string{}
	persistent := i.spec.Partitions.Persistent
	if persistent != nil && persistent.MountPoint != "" {
		extraMounts[persistent.MountPoint] = cnst.PersistentPath
	}
	oem := i.spec.Partitions.OEM
	if oem != nil && oem.MountPoint != "" {
		extraMounts[oem.MountPoint] = cnst.OEMPath
	}
	efi := i.spec.Partitions.Boot
	if efi != nil && efi.MountPoint != "" {
		extraMounts[efi.MountPoint] = cnst.BootDir
	}
	return ChrootHook(&i.cfg.Config, hook, i.cfg.Strict, root, extraMounts, i.cfg.CloudInitPaths...)
}

func (i *InstallAction) createInstallStateYaml() error {
	if i.spec.Partitions.State == nil || i.spec.Partitions.Recovery == nil {
		return fmt.Errorf("undefined state or recovery partition")
	}

	if i.snapshot == nil {
		return fmt.Errorf("undefined installed snapshot")
	}

	date := time.Now().Format(time.RFC3339)

	installState := &types.InstallState{
		Date:        date,
		Snapshotter: i.cfg.Snapshotter,
		Partitions: map[string]*types.PartitionState{
			cnst.StatePartName: {
				FSLabel: i.spec.Partitions.State.FilesystemLabel,
				Snapshots: map[int]*types.SystemState{
					i.snapshot.ID: {
						Source:     i.spec.System,
						Digest:     i.spec.System.GetDigest(),
						Active:     true,
						Labels:     i.spec.SnapshotLabels,
						Date:       date,
						FromAction: cnst.ActionInstall,
					},
				},
			},
			cnst.RecoveryPartName: {
				FSLabel: i.spec.Partitions.Recovery.FilesystemLabel,
				RecoveryImage: &types.SystemState{
					Source:     i.spec.RecoverySystem.Source,
					Digest:     i.spec.RecoverySystem.Source.GetDigest(),
					Label:      i.spec.RecoverySystem.Label,
					FS:         i.spec.RecoverySystem.FS,
					Labels:     i.spec.SnapshotLabels,
					Date:       date,
					FromAction: cnst.ActionInstall,
				},
			},
		},
	}

	if i.spec.Partitions.OEM != nil {
		installState.Partitions[cnst.OEMPartName] = &types.PartitionState{
			FSLabel: i.spec.Partitions.OEM.FilesystemLabel,
		}
	}
	if i.spec.Partitions.Persistent != nil {
		installState.Partitions[cnst.PersistentPartName] = &types.PartitionState{
			FSLabel: i.spec.Partitions.Persistent.FilesystemLabel,
		}
	}
	if i.spec.Partitions.Boot != nil {
		installState.Partitions[cnst.BootPartName] = &types.PartitionState{
			FSLabel: i.spec.Partitions.Boot.FilesystemLabel,
		}
	}

	return i.cfg.WriteInstallState(
		installState,
		filepath.Join(i.spec.Partitions.State.MountPoint, cnst.InstallStateFile),
		filepath.Join(i.spec.Partitions.Recovery.MountPoint, cnst.InstallStateFile),
	)
}

// InstallRun will install the system from a given configuration
func (i InstallAction) Run() (err error) {
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	// Set installation sources from a downloaded ISO
	if i.spec.Iso != "" {
		isoSrc, isoCleaner, err := elemental.SourceFormISO(i.cfg.Config, i.spec.Iso)
		cleanup.Push(isoCleaner)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.Unknown)
		}
		i.spec.System = isoSrc
	}

	// Partition and format device if needed
	err = i.prepareDevice()
	if err != nil {
		return err
	}

	err = elemental.MountPartitions(i.cfg.Config, i.spec.Partitions.PartitionsByMountPoint(false), "rw")
	if err != nil {
		i.cfg.Logger.Errorf("failed mounting partitions")
		return elementalError.NewFromError(err, elementalError.MountPartitions)
	}
	cleanup.Push(func() error {
		return elemental.UnmountPartitions(i.cfg.Config, i.spec.Partitions.PartitionsByMountPoint(true))
	})

	err = i.snapshotter.InitSnapshotter(i.spec.Partitions.State, i.spec.Partitions.Boot.MountPoint)
	if err != nil {
		i.cfg.Logger.Errorf("failed initializing snapshotter")
		return elementalError.NewFromError(err, elementalError.SnapshotterInit)
	}

	// Before install hook happens after partitioning but before the image OS is applied
	err = i.installHook(cnst.BeforeInstallHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookBeforeInstall)
	}

	// Starting snapshotter transaction
	i.cfg.Logger.Info("Starting snapshotter transaction")
	i.snapshot, err = i.snapshotter.StartTransaction()
	if err != nil {
		i.cfg.Logger.Errorf("failed to start snapshotter transaction")
		return elementalError.NewFromError(err, elementalError.SnapshotterStart)
	}
	cleanup.PushErrorOnly(func() error { return i.snapshotter.CloseTransactionOnError(i.snapshot) })

	// Deploy system image
	err = elemental.MirrorRoot(i.cfg.Config, i.snapshot.WorkDir, i.spec.System)
	if err != nil {
		i.cfg.Logger.Errorf("failed deploying source: %s", i.spec.System.String())
		return elementalError.NewFromError(err, elementalError.DumpSource)
	}

	// Fine tune the dumped tree
	i.cfg.Logger.Info("Fine tune the dumped root tree")
	err = i.refineDeployment()
	if err != nil {
		i.cfg.Logger.Error("failed refining system root tree")
		return err
	}

	// Closing snapshotter transaction
	i.cfg.Logger.Info("Closing snapshotter transaction")
	err = i.snapshotter.CloseTransaction(i.snapshot)
	if err != nil {
		i.cfg.Logger.Errorf("failed closing snapshot transaction: %v", err)
		return err
	}

	// Install recovery
	recoveryBootDir := filepath.Join(i.spec.Partitions.Recovery.MountPoint, "boot")
	err = utils.MkdirAll(i.cfg.Fs, recoveryBootDir, cnst.DirPerm)
	if err != nil {
		i.cfg.Logger.Errorf("failed creating recovery boot dir: %v", err)
		return err
	}

	recoverySystem := i.spec.RecoverySystem
	i.cfg.Logger.Info("Deploying recovery system")
	if recoverySystem.Source.String() == i.spec.System.String() {
		// Reuse already deployed root-tree from active snapshot
		recoverySystem.Source, err = i.snapshotter.SnapshotToImageSource(i.snapshot)
		if err != nil {
			return err
		}
		recoverySystem.Source.SetDigest(i.spec.System.GetDigest())
	}
	err = elemental.DeployRecoverySystem(i.cfg.Config, &recoverySystem)
	if err != nil {
		i.cfg.Logger.Errorf("Failed deploying recovery image: %v", err)
		return elementalError.NewFromError(err, elementalError.DeployImage)
	}

	err = i.installHook(cnst.PostInstallHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookPostInstall)
	}

	// Add state.yaml file on state and recovery partitions
	i.cfg.Logger.Info("Creating installation state files")
	err = i.createInstallStateYaml()
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateFile)
	}

	// Do not reboot/poweroff on cleanup errors
	err = cleanup.Cleanup(err)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.Cleanup)
	}

	// If we want to eject the cd, create the required executable so the cd is ejected at shutdown
	if i.cfg.EjectCD && utils.BootedFrom(i.cfg.Runner, "cdroot") {
		i.cfg.Logger.Infof("Writing eject script")
		err = i.cfg.Fs.WriteFile("/usr/lib/systemd/system-shutdown/eject", []byte(cnst.EjectScript), 0744)
		if err != nil {
			i.cfg.Logger.Warnf("Could not write eject script, cdrom wont be ejected automatically: %s", err)
		}
	}

	return PowerAction(i.cfg)
}

func (i *InstallAction) prepareDevice() error {
	if i.spec.NoFormat {
		if elemental.CheckActiveDeployment(i.cfg.Config) && !i.spec.Force {
			return elementalError.New("use `force` flag to run an installation over the current running deployment", elementalError.AlreadyInstalled)
		}
	} else {
		// Deactivate any active volume on target
		err := elemental.DeactivateDevices(i.cfg.Config)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.DeactivatingDevices)
		}
		// Partition device
		err = elemental.PartitionAndFormatDevice(i.cfg.Config, i.spec)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.PartitioningDevice)
		}
	}
	return nil
}

func (i *InstallAction) refineDeployment() error { //nolint:dupl
	// Copy cloud-init if any
	err := elemental.CopyCloudConfig(i.cfg.Config, i.spec.Partitions.GetConfigStorage(), i.spec.CloudInit)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyFile)
	}
	// Install grub
	err = i.bootloader.Install(
		i.snapshot.WorkDir,
		i.spec.Partitions.Boot.MountPoint,
	)
	if err != nil {
		i.cfg.Logger.Errorf("failed installing grub: %v", err)
		return elementalError.NewFromError(err, elementalError.InstallGrub)
	}

	err = i.installChrootHook(cnst.AfterInstallChrootHook, cnst.WorkingImgDir)
	if err != nil {
		i.cfg.Logger.Errorf("failed after-install-chroot hook: %v", err)
		return elementalError.NewFromError(err, elementalError.HookAfterInstallChroot)
	}
	err = i.installHook(cnst.AfterInstallHook)
	if err != nil {
		i.cfg.Logger.Errorf("failed after-install hook: %v", err)
		return elementalError.NewFromError(err, elementalError.HookAfterInstall)
	}

	grubVars := i.spec.GetGrubLabels()
	err = i.bootloader.SetPersistentVariables(
		filepath.Join(i.spec.Partitions.Boot.MountPoint, cnst.GrubOEMEnv),
		grubVars,
	)
	if err != nil {
		i.cfg.Logger.Errorf("failed setting GRUB labels: %v", err)
		return elementalError.NewFromError(err, elementalError.SetGrubVariables)
	}

	// Installation rebrand (only grub for now)
	err = i.bootloader.SetDefaultEntry(
		i.spec.Partitions.Boot.MountPoint,
		cnst.WorkingImgDir,
		i.spec.GrubDefEntry,
	)
	if err != nil {
		i.cfg.Logger.Errorf("failed setting defaut GRUB entry: %v", err)
		return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
	}
	return nil
}
