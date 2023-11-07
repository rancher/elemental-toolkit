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

package action

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/rancher/elemental-toolkit/pkg/bootloader"
	cnst "github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	elementalError "github.com/rancher/elemental-toolkit/pkg/error"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

type InstallAction struct {
	cfg        *v1.RunConfig
	spec       *v1.InstallSpec
	bootloader v1.Bootloader
}

type InstallActionOption func(i *InstallAction) error

func WithInstallBootloader(bootloader v1.Bootloader) func(i *InstallAction) error {
	return func(i *InstallAction) error {
		i.bootloader = bootloader
		return nil
	}
}

func NewInstallAction(cfg *v1.RunConfig, spec *v1.InstallSpec, opts ...InstallActionOption) *InstallAction {
	i := &InstallAction{cfg: cfg, spec: spec}

	for _, o := range opts {
		err := o(i)
		if err != nil {
			cfg.Logger.Errorf("error applying config option: %s", err.Error())
			return nil
		}
	}

	if i.bootloader == nil {
		i.bootloader = bootloader.NewGrub(&cfg.Config, bootloader.WithGrubDisableBootEntry(i.spec.DisableBootEntry))
	}

	return i
}

func (i *InstallAction) installHook(hook string) error {
	return Hook(&i.cfg.Config, hook, i.cfg.Strict, i.cfg.CloudInitPaths...)
}

func (i *InstallAction) installChrootHook(hook string, root string) error {
	extraMounts := map[string]string{}
	persistent := i.spec.Partitions.Persistent
	if persistent != nil && persistent.MountPoint != "" {
		extraMounts[persistent.MountPoint] = cnst.UsrLocalPath
	}
	oem := i.spec.Partitions.OEM
	if oem != nil && oem.MountPoint != "" {
		extraMounts[oem.MountPoint] = cnst.OEMPath
	}
	return ChrootHook(&i.cfg.Config, hook, i.cfg.Strict, root, extraMounts, i.cfg.CloudInitPaths...)
}

func (i *InstallAction) createInstallStateYaml(sysMeta, recMeta interface{}) error {
	if i.spec.Partitions.State == nil || i.spec.Partitions.Recovery == nil {
		return fmt.Errorf("undefined state or recovery partition")
	}

	// If recovery image is a copyied file from active reuse the same source and metadata
	recSource := i.spec.Recovery.Source
	if i.spec.Recovery.Source.IsFile() && i.spec.Active.File == i.spec.Recovery.Source.Value() {
		recMeta = sysMeta
		recSource = i.spec.Active.Source
	}

	installState := &v1.InstallState{
		Date: time.Now().Format(time.RFC3339),
		Partitions: map[string]*v1.PartitionState{
			cnst.StatePartName: {
				FSLabel: i.spec.Partitions.State.FilesystemLabel,
				Images: map[string]*v1.ImageState{
					cnst.ActiveImgName: {
						Source:         i.spec.Active.Source,
						SourceMetadata: sysMeta,
						Label:          i.spec.Active.Label,
						FS:             i.spec.Active.FS,
					},
					cnst.PassiveImgName: {
						Source:         i.spec.Active.Source,
						SourceMetadata: sysMeta,
						Label:          i.spec.Passive.Label,
						FS:             i.spec.Passive.FS,
					},
				},
			},
			cnst.RecoveryPartName: {
				FSLabel: i.spec.Partitions.Recovery.FilesystemLabel,
				Images: map[string]*v1.ImageState{
					cnst.RecoveryImgName: {
						Source:         recSource,
						SourceMetadata: recMeta,
						Label:          i.spec.Recovery.Label,
						FS:             i.spec.Recovery.FS,
					},
				},
			},
		},
	}
	if i.spec.Partitions.OEM != nil {
		installState.Partitions[cnst.OEMPartName] = &v1.PartitionState{
			FSLabel: i.spec.Partitions.OEM.FilesystemLabel,
		}
	}
	if i.spec.Partitions.Persistent != nil {
		installState.Partitions[cnst.PersistentPartName] = &v1.PartitionState{
			FSLabel: i.spec.Partitions.Persistent.FilesystemLabel,
		}
	}
	if i.spec.Partitions.EFI != nil {
		installState.Partitions[cnst.EfiPartName] = &v1.PartitionState{
			FSLabel: i.spec.Partitions.EFI.FilesystemLabel,
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
	e := elemental.NewElemental(&i.cfg.Config)
	cleanup := utils.NewCleanStack()
	defer func() { err = cleanup.Cleanup(err) }()

	// Set installation sources from a downloaded ISO
	if i.spec.Iso != "" {
		isoCleaner, err := e.UpdateSourceFormISO(i.spec.Iso, &i.spec.Active)
		cleanup.Push(isoCleaner)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.Unknown)
		}
	}

	// Partition and format device if needed
	err = i.prepareDevice(e)
	if err != nil {
		return err
	}

	err = e.MountPartitions(i.spec.Partitions.PartitionsByMountPoint(false))
	if err != nil {
		return elementalError.NewFromError(err, elementalError.MountPartitions)
	}
	cleanup.Push(func() error {
		return e.UnmountPartitions(i.spec.Partitions.PartitionsByMountPoint(true))
	})

	// Before install hook happens after partitioning but before the image OS is applied
	err = i.installHook(cnst.BeforeInstallHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookBeforeInstall)
	}

	// Deploy active image
	systemMeta, treeCleaner, err := e.DeployImgTree(&i.spec.Active, cnst.WorkingImgDir)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.DeployImgTree)
	}
	cleanup.Push(func() error { return treeCleaner() })

	// Copy cloud-init if any
	err = e.CopyCloudConfig(i.spec.Partitions.GetConfigStorage(), i.spec.CloudInit)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CopyFile)
	}
	// Install grub
	err = i.bootloader.Install(
		cnst.WorkingImgDir,
		i.spec.Partitions.State.MountPoint,
		i.spec.Partitions.State.FilesystemLabel,
	)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.InstallGrub)
	}

	// Relabel SELinux
	err = i.applySelinuxLabels(e)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SelinuxRelabel)
	}

	err = i.installChrootHook(cnst.AfterInstallChrootHook, cnst.WorkingImgDir)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookAfterInstallChroot)
	}
	err = i.installHook(cnst.AfterInstallHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookAfterInstall)
	}

	grubVars := i.spec.GetGrubLabels()
	err = i.bootloader.SetPersistentVariables(
		filepath.Join(i.spec.Partitions.State.MountPoint, cnst.GrubOEMEnv),
		grubVars,
	)
	if err != nil {
		i.cfg.Logger.Error("Error setting GRUB labels: %s", err)
		return elementalError.NewFromError(err, elementalError.SetGrubVariables)
	}

	// Installation rebrand (only grub for now)
	err = i.bootloader.SetDefaultEntry(
		i.spec.Partitions.State.MountPoint,
		cnst.WorkingImgDir,
		i.spec.GrubDefEntry,
	)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.SetDefaultGrubEntry)
	}

	err = e.CreateImgFromTree(cnst.WorkingImgDir, &i.spec.Active, false, treeCleaner)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.CreateImgFromTree)
	}

	// Install Recovery
	var recoveryMeta interface{}
	if i.spec.Recovery.Source.IsFile() && i.spec.Active.File == i.spec.Recovery.Source.Value() && i.spec.Active.FS == i.spec.Recovery.FS {
		// Reuse image file from active image
		err := e.CopyFileImg(&i.spec.Recovery)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.CopyFileImg)
		}
	} else {
		recoveryMeta, err = e.DeployImage(&i.spec.Recovery)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.DeployImage)
		}
	}

	// Install Passive
	err = e.CopyFileImg(&i.spec.Passive)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.DeployImage)
	}

	err = i.installHook(cnst.PostInstallHook)
	if err != nil {
		return elementalError.NewFromError(err, elementalError.HookPostInstall)
	}

	// Add state.yaml file on state and recovery partitions
	err = i.createInstallStateYaml(systemMeta, recoveryMeta)
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

// applySelinuxLabels sets SELinux extended attributes to the root-tree being installed
func (i *InstallAction) applySelinuxLabels(e *elemental.Elemental) error {
	binds := map[string]string{}
	if mnt, _ := utils.IsMounted(&i.cfg.Config, i.spec.Partitions.Persistent); mnt {
		binds[i.spec.Partitions.Persistent.MountPoint] = cnst.UsrLocalPath
	}
	if mnt, _ := utils.IsMounted(&i.cfg.Config, i.spec.Partitions.OEM); mnt {
		binds[i.spec.Partitions.OEM.MountPoint] = cnst.OEMPath
	}
	return utils.ChrootedCallback(
		&i.cfg.Config, cnst.WorkingImgDir, binds, func() error { return e.SelinuxRelabel("/", true) },
	)
}

func (i *InstallAction) prepareDevice(e *elemental.Elemental) error {
	if i.spec.NoFormat {
		// Check force flag against current device
		labels := []string{i.spec.Active.Label, i.spec.Recovery.Label}
		if e.CheckActiveDeployment(labels) && !i.spec.Force {
			return elementalError.New("use `force` flag to run an installation over the current running deployment", elementalError.AlreadyInstalled)
		}
	} else {
		// Deactivate any active volume on target
		err := e.DeactivateDevices()
		if err != nil {
			return elementalError.NewFromError(err, elementalError.DeactivatingDevices)
		}
		// Partition device
		err = e.PartitionAndFormatDevice(i.spec)
		if err != nil {
			return elementalError.NewFromError(err, elementalError.PartitioningDevice)
		}
	}
	return nil
}
