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

package action

import (
	"bufio"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/rancher/elemental-toolkit/pkg/constants"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

const overlaySuffix = ".overlay"
const labelPref = "LABEL="
const partLabelPref = "PARTLABEL="
const uuidPref = "UUID="
const devPref = "/dev/"
const diskBy = "/dev/disk/by-"
const diskByLabel = diskBy + "label"
const diskByPartLabel = diskBy + "partlabel"
const diskByUUID = diskBy + "uuid"
const runPath = "/run"

func RunMount(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	var fstabData string
	var err error

	cfg.Logger.Info("Running mount command")

	if spec.WriteFstab {
		cfg.Logger.Debug("Generating inital sysroot fstab lines")
		fstabData, err = InitialFstabData(cfg.Runner, spec.Sysroot)
		if err != nil {
			cfg.Logger.Errorf("Error mounting volumes: %s", err.Error())
			return err
		}

	}

	cfg.Logger.Debug("Mounting volumes")
	if err = MountVolumes(cfg, spec); err != nil {
		cfg.Logger.Errorf("Error mounting volumes: %s", err.Error())
		return err
	}

	cfg.Logger.Debugf("Mounting ephemeral directories")
	if err = MountEphemeral(cfg, spec.Sysroot, spec.Ephemeral); err != nil {
		cfg.Logger.Errorf("Error mounting overlays: %s", err.Error())
		return err
	}

	cfg.Logger.Debugf("Mounting persistent directories")
	if err = MountPersistent(cfg, spec); err != nil {
		cfg.Logger.Errorf("Error mounting persistent overlays: %s", err.Error())
		return err
	}

	cfg.Logger.Debugf("Writing fstab")
	if err = WriteFstab(cfg, spec, fstabData); err != nil {
		cfg.Logger.Errorf("Error writing new fstab: %s", err.Error())
		return err
	}

	cfg.Logger.Info("Mount command finished successfully")
	return nil
}

func MountVolumes(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	var errs error

	volumes := map[string]*v1.VolumeMount{}
	keys := []string{}
	if spec.HasPersistent() {
		volumes[spec.Persistent.Volume.Mountpoint] = &spec.Persistent.Volume
		keys = append(keys, spec.Persistent.Volume.Mountpoint)
	}

	for _, v := range spec.Volumes {
		volumes[v.Mountpoint] = v
		keys = append(keys, v.Mountpoint)
	}

	sort.Strings(keys)

	for _, k := range keys {
		var dev string
		switch {
		case strings.HasPrefix(volumes[k].Device, labelPref):
			dev = filepath.Join(diskByLabel, strings.TrimPrefix(volumes[k].Device, labelPref))
		case strings.HasPrefix(volumes[k].Device, partLabelPref):
			dev = filepath.Join(diskByPartLabel, strings.TrimPrefix(volumes[k].Device, partLabelPref))
		case strings.HasPrefix(volumes[k].Device, uuidPref):
			dev = filepath.Join(diskByUUID, strings.TrimPrefix(volumes[k].Device, uuidPref))
		case strings.HasPrefix(volumes[k].Device, devPref):
			dev = volumes[k].Device
		default:
			cfg.Logger.Errorf("Unknown device reference, it should be LABEL, PARTLABEL, UUID or a /dev/* path")
			errs = multierror.Append(errs, fmt.Errorf("Unkown device reference: %s", volumes[k].Device))
			continue
		}
		mountpoint := volumes[k].Mountpoint
		if !strings.HasPrefix(mountpoint, runPath) {
			mountpoint = filepath.Join(spec.Sysroot, mountpoint)
		}

		err := utils.MkdirAll(cfg.Fs, mountpoint, constants.DirPerm)
		if err != nil {
			cfg.Logger.Errorf("failed creating mountpoint %s", mountpoint)
			errs = multierror.Append(errs, err)
			continue
		}

		fstype := volumes[k].FSType
		if fstype == "" {
			fstype = "auto"
		}

		cfg.Logger.Debugf("Mounting %s to %s", dev, mountpoint)
		err = cfg.Mounter.Mount(dev, mountpoint, fstype, volumes[k].Options)
		if err != nil {
			cfg.Logger.Errorf("failed mounting device %s to %s", dev, mountpoint)
			errs = multierror.Append(errs, err)
		}
	}
	return errs
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

func MountPersistent(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	mountFunc := MountOverlayPath
	if spec.Persistent.Mode == "bind" {
		mountFunc = MountBindPath
	}

	if !spec.HasPersistent() {
		cfg.Logger.Debug("No persistent device defined, omitting persistent paths mounts")
		return nil
	}

	for _, path := range spec.Persistent.Paths {
		cfg.Logger.Debugf("Mounting path %s into %s", path, spec.Sysroot)

		target := filepath.Join(spec.Persistent.Volume.Mountpoint, constants.PersistentStateDir)
		if err := mountFunc(cfg, spec.Sysroot, target, path); err != nil {
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

func WriteFstab(cfg *v1.RunConfig, spec *v1.MountSpec, data string) error {
	var errs error

	if !spec.WriteFstab {
		cfg.Logger.Debug("Skipping writing fstab")
		return nil
	}

	for _, vol := range spec.Volumes {
		data += fstab(vol.Device, vol.Mountpoint, vol.FSType, vol.Options)
	}

	if spec.HasPersistent() {
		pVol := spec.Persistent.Volume
		data += fstab(pVol.Device, pVol.Mountpoint, pVol.FSType, pVol.Options)

		for _, path := range spec.Persistent.Paths {
			if spec.Persistent.Mode == constants.OverlayMode {
				data += overlayLine(path, filepath.Join(pVol.Mountpoint, constants.PersistentStateDir), constants.PersistentDir)
				continue
			}

			if spec.Persistent.Mode == constants.BindMode {
				trimmed := strings.TrimPrefix(path, "/")
				pathName := strings.ReplaceAll(trimmed, "/", "-") + ".bind"
				stateDir := filepath.Join(pVol.Mountpoint, constants.PersistentStateDir, pathName)

				data = data + fstab(stateDir, path, "none", []string{"defaults", "bind"})
				continue
			}
			errs = multierror.Append(errs, fmt.Errorf("Unknown persistent mode '%s'", spec.Persistent.Mode))
		}
	}

	data += fstab("tmpfs", constants.OverlayDir, "tmpfs", []string{"defaults", fmt.Sprintf("size=%s", spec.Ephemeral.Size)})
	for _, rw := range spec.Ephemeral.Paths {
		data += overlayLine(rw, constants.OverlayDir, constants.OverlayDir)
	}

	return cfg.Config.Fs.WriteFile(filepath.Join(spec.Sysroot, "/etc/fstab"), []byte(data), 0644)
}

func InitialFstabData(runner v1.Runner, sysroot string) (string, error) {
	var data string

	mounts, err := findmnt(runner, "/")
	if err != nil {
		return "", err
	}
	for _, mnt := range mounts {
		if mnt.Mountpoint == sysroot {
			data += fstab(mnt.Device, "/", mnt.FSType, mnt.Options)
		} else if strings.HasPrefix(mnt.Mountpoint, sysroot) {
			data += fstab(mnt.Device, strings.TrimPrefix(mnt.Mountpoint, sysroot), mnt.FSType, mnt.Options)
		} else if strings.HasPrefix(mnt.Mountpoint, constants.RunElementalDir) {
			data += fstab(mnt.Device, mnt.Mountpoint, mnt.FSType, mnt.Options)
		} else if mnt.Mountpoint == constants.RunningStateDir {
			data += fstab(mnt.Device, mnt.Mountpoint, mnt.FSType, mnt.Options)
		}
	}

	return data, nil
}

func fstab(device, path, fstype string, flags []string) string {
	if len(flags) == 0 {
		flags = []string{"defaults"}
	}

	if fstype == "" {
		fstype = "auto"
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t0\t0\n", device, path, fstype, strings.Join(flags, ","))
}

func findmnt(runner v1.Runner, mountpoint string) ([]*v1.VolumeMount, error) {
	mounts := []*v1.VolumeMount{}
	output, err := runner.Run("findmnt", "-Rrfno", "SOURCE,TARGET,FSTYPE,OPTIONS", mountpoint)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(string(output))))
	for scanner.Scan() {
		lineFields := strings.Fields(scanner.Text())
		if len(lineFields) != 4 {
			continue
		}
		if lineFields[2] == "btrfs" {
			r := regexp.MustCompile(`^(/[^\[\]]+)`)
			if match := r.FindStringSubmatch(lineFields[0]); match != nil {
				lineFields[0] = match[1]
			}
		}
		mounts = append(mounts, &v1.VolumeMount{
			Device:     lineFields[0],
			Mountpoint: lineFields[1],
			Options:    strings.Split(lineFields[3], ","),
			FSType:     lineFields[2],
		})
	}
	return mounts, nil
}

func overlayLine(path, upperPath, requriedMount string) string {
	trimmed := strings.TrimPrefix(path, "/")
	pathName := strings.ReplaceAll(trimmed, "/", "-") + overlaySuffix
	upper := fmt.Sprintf("%s/%s/upper", upperPath, pathName)
	work := fmt.Sprintf("%s/%s/work", upperPath, pathName)

	options := []string{"defaults"}
	options = append(options, fmt.Sprintf("lowerdir=%s", path))
	options = append(options, fmt.Sprintf("upperdir=%s", upper))
	options = append(options, fmt.Sprintf("workdir=%s", work))
	options = append(options, fmt.Sprintf("x-systemd.requires-mounts-for=%s", requriedMount))
	return fstab("overlay", path, "overlay", options)
}
