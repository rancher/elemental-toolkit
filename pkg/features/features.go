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

package features

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/systemd"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

//go:embed embedded
var files embed.FS

const (
	embeddedRoot = "embedded"

	FeatureImmutableRootfs = "immutable-rootfs"
	FeatureGrubConfig      = "grub-config"
	FeatureElementalSetup  = "elemental-setup"
	FeatureDracutConfig    = "dracut-config"
	FeatureBootAssessment  = "boot-assessment"
	FeatureLiveCD          = "livecd"
	FeatureRecovery        = "recovery"
	FeatureNetwork         = "network"
	FeatureDefaultServices = "default-services"
)

var (
	All = []string{
		FeatureImmutableRootfs,
		FeatureGrubConfig,
		FeatureElementalSetup,
		FeatureDracutConfig,
		FeatureBootAssessment,
		FeatureLiveCD,
		FeatureRecovery,
		FeatureNetwork,
		FeatureDefaultServices,
	}
)

type Feature struct {
	Name        string
	Units       []*systemd.Unit
	ConfigFiles []*ConfigFile
}

type ConfigFile struct {
	Source      string
	Destination string
}

func NewConfigFile(source, destination string) *ConfigFile {
	return &ConfigFile{
		source,
		destination,
	}
}

func (c *ConfigFile) Install(srcFs embed.FS, destFs v1.FS) error {
	if err := utils.MkdirAll(destFs, filepath.Dir(c.Destination), constants.DirPerm); err != nil {
		return err
	}

	return ExtractFile(srcFs, c.Source, destFs, c.Destination)
}

func New(name string, units []*systemd.Unit, files []*ConfigFile) *Feature {
	return &Feature{
		name,
		units,
		files,
	}
}

func (f *Feature) Install(log v1.Logger, dstFs v1.FS, runner v1.Runner) error {
	for _, conf := range f.ConfigFiles {
		if err := conf.Install(files, dstFs); err != nil {
			log.Errorf("Error installing config-file '%s': %v", conf.Source, err.Error())
			return err
		}
	}

	for _, unit := range f.Units {
		log.Debugf("Enabling unit '%s'", unit.Name)
		if err := systemd.Enable(runner, unit); err != nil {
			log.Errorf("Error enabling unit '%s': %v", unit.Name, err.Error())
			return err
		}
	}

	return nil
}

func ExtractFile(srcFs embed.FS, srcPath string, dstFs v1.FS, dstPath string) error {
	content, err := srcFs.ReadFile(srcPath)
	if err != nil {
		return err
	}

	return dstFs.WriteFile(dstPath, content, 0644)
}

func Get(names []string) ([]*Feature, error) {
	if len(names) == 0 {
		return []*Feature{}, nil
	}

	features := []*Feature{}
	notFound := []string{}

	for _, name := range names {
		switch name {
		case FeatureImmutableRootfs:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "02-elemental-immutable-rootfs.conf"), "/etc/dracut.conf.d/02-elemental-immutable-rootfs.conf"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "immutable-rootfs", "module-setup.sh"), "/usr/lib/dracut/modules.d/30immutable-rootfs/module-setup.sh"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "immutable-rootfs", "elemental-fsck.sh"), "/usr/lib/dracut/modules.d/30immutable-rootfs/elemental-fsck.sh"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "immutable-rootfs", "elemental-generator.sh"), "/usr/lib/dracut/modules.d/30immutable-rootfs/elemental-generator.sh"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "immutable-rootfs", "elemental-mount-layout.sh"), "/usr/lib/dracut/modules.d/30immutable-rootfs/elemental-mount-layout.sh"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "immutable-rootfs", "parse-elemental-cmdline.sh"), "/usr/lib/dracut/modules.d/30immutable-rootfs/parse-elemental-cmdline.sh"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "immutable-rootfs", "elemental-immutable-rootfs.service"), "/usr/lib/dracut/modules.d/30immutable-rootfs/elemental-immutable-rootfs.service"))
			features = append(features, New(name, nil, configs))
		case FeatureDracutConfig:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "50-elemental-initrd.conf"), "/etc/dracut.conf.d/50-elemental-initrd.conf"))
			features = append(features, New(name, nil, configs))
		case FeatureGrubConfig:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "grub.cfg"), "/etc/cos/grub.cfg"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "bootargs.cfg"), "/etc/cos/bootargs.cfg"))
			features = append(features, New(name, nil, configs))
		case FeatureElementalSetup:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "02-elemental-setup-initramfs.conf"), "/etc/dracut.conf.d/02-elemental-setup-initramfs.conf"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "elemental-setup-reconcile.service"), "/usr/lib/systemd/system/elemental-setup-reconcile.service"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "elemental-setup-reconcile.timer"), "/usr/lib/systemd/system/elemental-setup-reconcile.timer"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "elemental-setup-boot.service"), "/usr/lib/systemd/system/elemental-setup-boot.service"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "elemental-setup-rootfs.service"), "/usr/lib/systemd/system/elemental-setup-rootfs.service"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "elemental-setup-network.service"), "/usr/lib/systemd/system/elemental-setup-network.service"))
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "elemental-setup-initramfs.service"), "/usr/lib/systemd/system/elemental-setup-initramfs.service"))

			units := []*systemd.Unit{
				systemd.NewUnit("elemental-setup-reconcile.service"),
				systemd.NewUnit("elemental-setup-reconcile.timer"),
				systemd.NewUnit("elemental-setup-boot.service"),
				systemd.NewUnit("elemental-setup-rootfs.service"),
				systemd.NewUnit("elemental-setup-network.service"),
				systemd.NewUnit("elemental-setup-initramfs.service"),
			}
			features = append(features, New(name, units, configs))
		case FeatureBootAssessment:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "08_boot_assessment.yaml"), "/system/oem/08_boot_assessment.yaml"))
			features = append(features, New(name, nil, configs))
		case FeatureLiveCD:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "07_livecd.yaml"), "/system/oem/07_livecd.yaml"))
			features = append(features, New(name, nil, configs))
		case FeatureRecovery:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "06_recovery.yaml"), "/system/oem/06_recovery.yaml"))
			features = append(features, New(name, nil, configs))
		case FeatureNetwork:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "05_network.yaml"), "/system/oem/05_network.yaml"))
			features = append(features, New(name, nil, configs))
		case FeatureDefaultServices:
			configs := []*ConfigFile{}
			configs = append(configs, NewConfigFile(filepath.Join(embeddedRoot, "01_defaults.yaml"), "/system/oem/01_defaults.yaml"))
			features = append(features, New(name, nil, configs))
		default:
			notFound = append(notFound, name)
		}
	}

	if len(notFound) != 0 {
		return features, fmt.Errorf("Unknown features: %s", strings.Join(notFound, ", "))
	}

	return features, nil
}
