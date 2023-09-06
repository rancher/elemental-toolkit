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
	"github.com/rancher/elemental-toolkit/pkg/elemental"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

func RunMount(cfg *v1.RunConfig, spec *v1.MountSpec) error {
	cfg.Logger.Info("Running mount command")

	e := elemental.NewElemental(&cfg.Config)

	err := e.MountPartitions(spec.Partitions.PartitionsByMountPoint(false))
	if err != nil {
		return err
	}

	if err := e.MountImage(spec.Image); err != nil {
		cfg.Logger.Errorf("Error mounting image %s: %s", spec.Image.File, err.Error())
		return err
	}

	for _, path := range spec.RwPaths {
		cfg.Logger.Debugf("Mounting path %s into %s", path, spec.Sysroot)
		if err := MountRwPath(cfg, spec.Sysroot, path); err != nil {
			cfg.Logger.Errorf("Error mounting path %s: %s", path, err.Error())
			return err
		}
	}

	if err := WriteFstab(cfg, spec); err != nil {
		cfg.Logger.Errorf("Error writing new fstab: %s", err.Error())
		return err
	}

	cfg.Logger.Info("Mount command finished successfully")
	return nil
}

func MountRwPath(cfg *v1.RunConfig, sysroot, path string) error {
	cfg.Logger.Debugf("Mounting Path")

	lower := filepath.Join(sysroot, path)
	if err := utils.MkdirAll(cfg.Config.Fs, lower, constants.DirPerm); err != nil {
		cfg.Logger.Errorf("Error creating directory %s: %s", path, err.Error())
		return err
	}

	trimmed := strings.TrimPrefix(path, "/")
	pathName := strings.ReplaceAll(trimmed, "/", "-")
	upper := fmt.Sprintf("%s/%s/upper", constants.OverlayDir, pathName)
	if err := utils.MkdirAll(cfg.Config.Fs, upper, constants.DirPerm); err != nil {
		cfg.Logger.Errorf("Error creating upperdir %s: %s", upper, err.Error())
		return err
	}

	work := fmt.Sprintf("%s/%s/work", constants.OverlayDir, pathName)
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

	data := fmt.Sprintf("%s\t/\tauto\tro\t0 0\n", spec.Image.LoopDevice)

	for _, part := range spec.Partitions.PartitionsByMountPoint(false) {
		data = data + fmt.Sprintf("%s\t%s\t%s\t%s\n", part.Path, part.MountPoint, part.FS, "")
	}

	for _, rw := range spec.RwPaths {
		trimmed := strings.TrimPrefix(rw, "/")
		pathName := strings.ReplaceAll(trimmed, "/", "-")
		upper := fmt.Sprintf("%s/%s/upper", constants.OverlayDir, pathName)
		work := fmt.Sprintf("%s/%s/work", constants.OverlayDir, pathName)

		options := []string{"defaults"}
		options = append(options, fmt.Sprintf("lowerdir=%s", rw))
		options = append(options, fmt.Sprintf("upperdir=%s", upper))
		options = append(options, fmt.Sprintf("workdir=%s", work))
		options = append(options, fmt.Sprintf("x-systemd.requires-mounts-for=%s", constants.OverlayDir))
		data = data + fmt.Sprintf("%s\t%s\t%s\t%s\n", "overlay", rw, "overlay", strings.Join(options, ","))
	}

	return cfg.Config.Fs.WriteFile(filepath.Join(spec.Sysroot, "/etc/fstab"), []byte(data), 0644)
}
