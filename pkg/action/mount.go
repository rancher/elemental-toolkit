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

	"github.com/joho/godotenv"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

func RunMount(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	cfg.Logger.Info("Running mount command")

	e := elemental.NewElemental(&cfg.Config)

	cfg.Logger.Debug("Fscking partitions")
	if err := RunFsck(cfg, spec, e); err != nil {
		cfg.Logger.Errorf("Error fscking partitions: %s", err.Error())
		return err
	}

	cfg.Logger.Debug("Mounting partitions")
	if err := e.MountPartitions(spec.Partitions.PartitionsByMountPoint(false)); err != nil {
		cfg.Logger.Errorf("Error mounting partitions: %s", err.Error())
		return err
	}

	cfg.Logger.Debugf("Mounting image %s", spec.Image.File)
	if err := e.MountImage(spec.Image); err != nil {
		cfg.Logger.Errorf("Error mounting image %s: %s", spec.Image.File, err.Error())
		return err
	}

	if spec.RunCloudInit {
		cfg.Logger.Debug("Running rootfs cloud-init stage")
		err := utils.RunStage(&cfg.Config, "rootfs", cfg.Strict, cfg.CloudInitPaths...)
		if err != nil {
			cfg.Logger.Errorf("Error running rootfs stage: %s", err.Error())
			return err
		}
	} else {
		cfg.Logger.Debug("Skipping cloud-init rootfs stage")
	}

	if err := applyLayoutConfig(cfg, spec); err != nil {
		cfg.Logger.Errorf("Error reading env vars: %s", err.Error())
		return err
	}

	cfg.Logger.Debugf("Mounting overlays")
	if err := MountOverlay(cfg, spec.Sysroot, spec.Overlay); err != nil {
		cfg.Logger.Errorf("Error mounting image %s: %s", spec.Image.File, err.Error())
		return err
	}

	cfg.Logger.Debugf("Mounting persistent directories")
	if err := MountPersistent(cfg, spec.Sysroot, spec.Persistent); err != nil {
		cfg.Logger.Errorf("Error mounting image %s: %s", spec.Image.File, err.Error())
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

func applyLayoutConfig(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	// Read the OVERLAY, RW_PATHS, PERSISTENT_STATE_PATHS and PERSISTENT_STATE_BIND and overwrite the MountSpec
	files := []string{"/run/cos/cos-layout.env", "/run/elemental/layout.env"}

	for _, file := range files {
		cfg.Logger.Debugf("Parsing env vars from file '%s'", file)
		env, err := godotenv.Read(file)
		if err != nil {
			cfg.Logger.Warnf("Failed reading file %s: %s", file, err.Error())
			continue
		}

		if overlay, exists := env["OVERLAY"]; exists {
			cfg.Logger.Debug("Found OVERLAY env var")

			split := strings.SplitN(overlay, ":", 2)

			if split[0] == "tmpfs" && len(split) == 2 {
				spec.Overlay.Size = split[1]
			}
		}

		if rwPaths, exists := env["RW_PATHS"]; exists {
			cfg.Logger.Debug("Found RW_PATHS env var")
			spec.Overlay.Paths = strings.Fields(rwPaths)
		}

		if paths, exists := env["PERSISTENT_STATE_PATHS"]; exists {
			cfg.Logger.Debug("Found PERSISTENT_STATE_PATHS env var")
			spec.Persistent.Paths = strings.Fields(paths)
		}

		if _, exists := env["PERSISTENT_STATE_BIND"]; exists {
			cfg.Logger.Debug("Found PERSISTENT_STATE_BIND env var")
			spec.Persistent.Mode = constants.BindMode
		}
	}

	return nil
}

func RunFsck(cfg *v1.RunConfig, spec *v1.MountSpec, e *elemental.Elemental) error {
	if !spec.RunFsck {
		cfg.Logger.Debug("Skipping fsck")
		return nil
	}

	allParts, err := utils.GetAllPartitions()
	if err != nil {
		cfg.Logger.Errorf("Error getting all partitions: %s", err.Error())
		return err
	}

	return e.FsckPartitions(allParts)
}

func MountOverlay(cfg *v1.RunConfig, sysroot string, overlay v1.OverlayMounts) error {
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
	pathName := strings.ReplaceAll(trimmed, "/", "-") + ".overlay"
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

func WriteFstab(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	if !spec.WriteFstab {
		cfg.Logger.Debug("Skipping writing fstab")
		return nil
	}

	data := fstab(spec.Image.LoopDevice, "/", "auto", []string{"ro"})
	data = data + fstab("tmpfs", constants.OverlayDir, "tmpfs", []string{"defaults", fmt.Sprintf("size=%s", spec.Overlay.Size)})

	for _, part := range spec.Partitions.PartitionsByMountPoint(false) {
		data = data + fstab(part.Path, part.MountPoint, "auto", []string{"defaults"})
	}

	for _, rw := range spec.Overlay.Paths {
		trimmed := strings.TrimPrefix(rw, "/")
		pathName := strings.ReplaceAll(trimmed, "/", "-") + ".overlay"
		upper := fmt.Sprintf("%s/%s/upper", constants.OverlayDir, pathName)
		work := fmt.Sprintf("%s/%s/work", constants.OverlayDir, pathName)

		options := []string{"defaults"}
		options = append(options, fmt.Sprintf("lowerdir=%s", rw))
		options = append(options, fmt.Sprintf("upperdir=%s", upper))
		options = append(options, fmt.Sprintf("workdir=%s", work))
		options = append(options, fmt.Sprintf("x-systemd.requires-mounts-for=%s", constants.OverlayDir))
		data = data + fstab("overlay", rw, "overlay", options)
	}

	// /usr/local/.state/var-lib-NetworkManager.bind /var/lib/NetworkManager none defaults,bind 0 0
	for _, path := range spec.Persistent.Paths {
		if spec.Persistent.Mode == constants.OverlayMode {
			trimmed := strings.TrimPrefix(path, "/")
			pathName := strings.ReplaceAll(trimmed, "/", "-") + ".overlay"
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
