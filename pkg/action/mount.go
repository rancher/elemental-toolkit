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
	"strings"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const overlaySuffix = ".overlay"

func RunMount(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	cfg.Logger.Info("Running mount command")

	cfg.Logger.Debugf("Mounting ephemeral directories")
	if err := MountEphemeral(cfg, spec.Sysroot, spec.Ephemeral); err != nil {
		cfg.Logger.Errorf("Error mounting overlays: %s", err.Error())
		return err
	}

	cfg.Logger.Debugf("Mounting persistent directories")
	if err := MountPersistent(cfg, spec.Sysroot, spec.Persistent); err != nil {
		cfg.Logger.Errorf("Error mounting persistent overlays: %s", err.Error())
		return err
	}

	cfg.Logger.Debugf("Writing fstab")
	if err := WriteFstab(cfg, spec); err != nil {
		cfg.Logger.Errorf("Error writing new fstab: %s", err.Error())
		return err
	}

	cfg.Logger.Info("Mount command finished successfully")
	return nil
}

func MountEphemeral(cfg *v1.RunConfig, sysroot string, overlay v1.EphemeralMounts) error {
	if err := utils.MkdirAll(cfg.Config.Fs, constants.OverlayDir, constants.DirPerm); err != nil {
		cfg.Logger.Errorf("Error creating directory %s: %s", constants.OverlayDir, err.Error())
		return err
	}

	var (
		overlaySource string
		overlayFS     string
		overlayOpts   []string
	)

	switch overlay.Type {
	case constants.Tmpfs:
		overlaySource = constants.Tmpfs
		overlayFS = constants.Tmpfs
		overlayOpts = []string{"defaults", fmt.Sprintf("size=%s", overlay.Size)}
	case constants.Block:
		overlaySource = overlay.Device
		overlayFS = constants.Autofs
		overlayOpts = []string{"defaults"}
	default:
		return fmt.Errorf("unknown overlay type '%s'", overlay.Type)
	}

	if err := cfg.Mounter.Mount(overlaySource, constants.OverlayDir, overlayFS, overlayOpts); err != nil {
		cfg.Logger.Errorf("Error mounting overlay: %s", err.Error())
		return err
	}

	for _, path := range overlay.Paths {
		cfg.Logger.Debugf("Mounting path %s into %s", path, sysroot)
		if err := MountOverlayPath(cfg, sysroot, constants.OverlayDir, path); err != nil {
			cfg.Logger.Errorf("Error mounting path %s: %s", path, err.Error())
			return err
		}
	}

	return nil
}

func MountPersistent(cfg *v1.RunConfig, sysroot string, persistent v1.PersistentMounts) error {
	mountFunc := MountOverlayPath
	if persistent.Mode == "bind" {
		mountFunc = MountBindPath
	}

	for _, path := range persistent.Paths {
		cfg.Logger.Debugf("Mounting path %s into %s", path, sysroot)

		if err := mountFunc(cfg, sysroot, constants.PersistentStateDir, path); err != nil {
			cfg.Logger.Errorf("Error mounting path %s: %s", path, err.Error())
			return err
		}
	}

	return nil
}

type MountFunc func(cfg *v1.RunConfig, sysroot, overlayDir, path string) error

func MountBindPath(cfg *v1.RunConfig, sysroot, overlayDir, path string) error {
	cfg.Logger.Debugf("Mounting bind path %s", path)

	base := filepath.Join(sysroot, path)
	if err := utils.MkdirAll(cfg.Config.Fs, base, constants.DirPerm); err != nil {
		cfg.Logger.Errorf("Error creating directory %s: %s", path, err.Error())
		return err
	}

	trimmed := strings.TrimPrefix(path, "/")
	pathName := strings.ReplaceAll(trimmed, "/", "-") + ".bind"
	stateDir := fmt.Sprintf("%s/%s", overlayDir, pathName)
	if err := utils.MkdirAll(cfg.Config.Fs, stateDir, constants.DirPerm); err != nil {
		cfg.Logger.Errorf("Error creating upperdir %s: %s", stateDir, err.Error())
		return err
	}

	if err := utils.SyncData(cfg.Logger, cfg.Runner, cfg.Fs, base, stateDir); err != nil {
		cfg.Logger.Errorf("Error shuffling data: %s", err.Error())
		return err
	}

	if err := cfg.Mounter.Mount(stateDir, base, "none", []string{"defaults", "bind"}); err != nil {
		cfg.Logger.Errorf("Error mounting overlay: %s", err.Error())
		return err
	}

	return nil
}

func MountOverlayPath(cfg *v1.RunConfig, sysroot, overlayDir, path string) error {
	cfg.Logger.Debugf("Mounting overlay path %s", path)

	lower := filepath.Join(sysroot, path)
	if err := utils.MkdirAll(cfg.Config.Fs, lower, constants.DirPerm); err != nil {
		cfg.Logger.Errorf("Error creating directory %s: %s", path, err.Error())
		return err
	}

	trimmed := strings.TrimPrefix(path, "/")
	pathName := strings.ReplaceAll(trimmed, "/", "-") + overlaySuffix
	upper := fmt.Sprintf("%s/%s/upper", overlayDir, pathName)
	if err := utils.MkdirAll(cfg.Config.Fs, upper, constants.DirPerm); err != nil {
		cfg.Logger.Errorf("Error creating upperdir %s: %s", upper, err.Error())
		return err
	}

	work := fmt.Sprintf("%s/%s/work", overlayDir, pathName)
	if err := utils.MkdirAll(cfg.Config.Fs, work, constants.DirPerm); err != nil {
		cfg.Logger.Errorf("Error creating workdir %s: %s", work, err.Error())
		return err
	}

	cfg.Logger.Debugf("Mounting overlay %s", lower)
	options := []string{"defaults"}
	options = append(options, fmt.Sprintf("lowerdir=%s", lower))
	options = append(options, fmt.Sprintf("upperdir=%s", upper))
	options = append(options, fmt.Sprintf("workdir=%s", work))

	if err := cfg.Mounter.Mount("overlay", lower, "overlay", options); err != nil {
		cfg.Logger.Errorf("Error mounting overlay: %s", err.Error())
		return err
	}

	return nil
}

func findmnt(runner v1.Runner, mountpoint string) (string, error) {
	output, err := runner.Run("findmnt", "-fno", "SOURCE", mountpoint)
	return strings.TrimSuffix(string(output), "\n"), err
}

func WriteFstab(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	if !spec.WriteFstab {
		cfg.Logger.Debug("Skipping writing fstab")
		return nil
	}

	loop, err := findmnt(cfg.Runner, spec.Sysroot)
	if err != nil {
		return err
	}

	data := fstab(loop, "/", "ext2", []string{"ro", "relatime"})
	data = data + fstab("tmpfs", constants.OverlayDir, "tmpfs", []string{"defaults", fmt.Sprintf("size=%s", spec.Ephemeral.Size)})

	for _, part := range spec.Partitions.PartitionsByMountPoint(false) {
		if part.Path == "" {
			// Lets error out only after 10 attempts to find the device
			device, err := utils.GetDeviceByLabel(cfg.Runner, part.FilesystemLabel, 10)
			if err != nil {
				cfg.Logger.Errorf("Could not find a device with label %s", part.FilesystemLabel)
				return err
			}
			part.Path = device
		}

		data = data + fstab(part.Path, part.MountPoint, "auto", []string{"defaults"})
	}

	for _, rw := range spec.Ephemeral.Paths {
		trimmed := strings.TrimPrefix(rw, "/")
		pathName := strings.ReplaceAll(trimmed, "/", "-") + overlaySuffix
		upper := fmt.Sprintf("%s/%s/upper", constants.OverlayDir, pathName)
		work := fmt.Sprintf("%s/%s/work", constants.OverlayDir, pathName)

		options := []string{"defaults"}
		options = append(options, fmt.Sprintf("lowerdir=%s", rw))
		options = append(options, fmt.Sprintf("upperdir=%s", upper))
		options = append(options, fmt.Sprintf("workdir=%s", work))
		options = append(options, fmt.Sprintf("x-systemd.requires-mounts-for=%s", constants.OverlayDir))
		data = data + fstab("overlay", rw, "overlay", options)
	}

	for _, path := range spec.Persistent.Paths {
		if spec.Persistent.Mode == constants.OverlayMode {
			trimmed := strings.TrimPrefix(path, "/")
			pathName := strings.ReplaceAll(trimmed, "/", "-") + overlaySuffix
			upper := fmt.Sprintf("%s/%s/upper", constants.PersistentStateDir, pathName)
			work := fmt.Sprintf("%s/%s/work", constants.PersistentStateDir, pathName)

			options := []string{"defaults"}
			options = append(options, fmt.Sprintf("lowerdir=%s", path))
			options = append(options, fmt.Sprintf("upperdir=%s", upper))
			options = append(options, fmt.Sprintf("workdir=%s", work))
			options = append(options, fmt.Sprintf("x-systemd.requires-mounts-for=%s", constants.PersistentDir))
			data = data + fstab("overlay", path, "overlay", options)

			continue
		}

		if spec.Persistent.Mode == constants.BindMode {
			trimmed := strings.TrimPrefix(path, "/")
			pathName := strings.ReplaceAll(trimmed, "/", "-") + ".bind"
			stateDir := fmt.Sprintf("%s/%s", constants.PersistentStateDir, pathName)

			data = data + fstab(stateDir, path, "none", []string{"defaults", "bind"})
			continue
		}

		return fmt.Errorf("Unknown persistent mode '%s'", spec.Persistent.Mode)
	}

	return cfg.Config.Fs.WriteFile(filepath.Join(spec.Sysroot, "/etc/fstab"), []byte(data), 0644)
}

func fstab(device, path, fstype string, flags []string) string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\t0\t0\n", device, path, fstype, strings.Join(flags, ","))
}
