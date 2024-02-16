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

package features

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rancher/elemental-toolkit/pkg/constants"
	"github.com/rancher/elemental-toolkit/pkg/systemd"
	v1 "github.com/rancher/elemental-toolkit/pkg/types/v1"
	"github.com/rancher/elemental-toolkit/pkg/utils"
)

//go:generate go run generate-tarballs.go ./embedded ./generated

//go:embed all:generated
var files embed.FS

const (
	embeddedRoot = "generated"

	FeatureImmutableRootfs       = "immutable-rootfs"
	FeatureElementalRootfs       = "elemental-rootfs"
	FeatureElementalSysroot      = "elemental-sysroot"
	FeatureGrubConfig            = "grub-config"
	FeatureGrubDefaultBootargs   = "grub-default-bootargs"
	FeatureElementalSetup        = "elemental-setup"
	FeatureDracutConfig          = "dracut-config"
	FeatureCloudConfigDefaults   = "cloud-config-defaults"
	FeatureCloudConfigEssentials = "cloud-config-essentials"
)

var (
	All = []string{
		FeatureElementalRootfs,
		FeatureElementalSysroot,
		FeatureGrubConfig,
		FeatureGrubDefaultBootargs,
		FeatureElementalSetup,
		FeatureDracutConfig,
		FeatureCloudConfigDefaults,
		FeatureCloudConfigEssentials,
	}
)

type Feature struct {
	Name  string
	Units []*systemd.Unit
}

func New(name string, units []*systemd.Unit) *Feature {
	return &Feature{
		name,
		units,
	}
}

func (f *Feature) Install(log v1.Logger, destFs v1.FS, runner v1.Runner) error {
	featurePath := filepath.Join(embeddedRoot, f.Name)
	err := fs.WalkDir(files, featurePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Errorf("Error accessing embedded file '%s': %s", path, err.Error())
			return err
		}

		if d.IsDir() {
			log.Debugf("Skipping dir %s", path)
			return nil
		}

		targetPath, err := filepath.Rel(featurePath, path)
		if err != nil {
			log.Errorf("Could not calculate relative path for file '%s': %s", path, err.Error())
			return err
		}
		targetPath = filepath.Join("/", targetPath)

		if err := utils.MkdirAll(destFs, filepath.Dir(targetPath), constants.DirPerm); err != nil {
			log.Errorf("Error mkdir: %s", err.Error())
			return err
		}

		content, err := files.ReadFile(path)
		if err != nil {
			log.Errorf("Error reading embedded file '%s': %s", path, err.Error())
			return err
		}

		log.Debugf("Writing file '%s' to '%s'", path, targetPath)
		return destFs.WriteFile(targetPath, content, 0755)
	})
	if err != nil {
		log.Errorf("Error walking files for feature %s: %s", f.Name, err.Error())
		return err
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

func Get(names []string) ([]*Feature, error) {
	if len(names) == 0 {
		return []*Feature{}, nil
	}

	features := []*Feature{}
	notFound := []string{}

	for _, name := range names {
		switch name {
		case FeatureCloudConfigDefaults:
			features = append(features, New(name, nil))
		case FeatureCloudConfigEssentials:
			features = append(features, New(name, nil))
		case FeatureImmutableRootfs:
			if slices.Contains(names, FeatureElementalRootfs) {
				return features, fmt.Errorf("Conflicting features: %s and %s", FeatureImmutableRootfs, FeatureElementalRootfs)
			}

			features = append(features, New(name, nil))
		case FeatureElementalRootfs:
			if slices.Contains(names, FeatureImmutableRootfs) {
				return features, fmt.Errorf("Conflicting features: %s and %s", FeatureElementalRootfs, FeatureImmutableRootfs)
			}

			units := []*systemd.Unit{
				systemd.NewUnit("elemental-rootfs.service"),
			}

			features = append(features, New(name, units))
		case FeatureElementalSysroot:
			features = append(features, New(name, nil))
		case FeatureDracutConfig:
			features = append(features, New(name, nil))
		case FeatureGrubConfig:
			features = append(features, New(name, nil))
		case FeatureGrubDefaultBootargs:
			features = append(features, New(name, nil))
		case FeatureElementalSetup:
			units := []*systemd.Unit{
				systemd.NewUnit("elemental-setup-reconcile.service"),
				systemd.NewUnit("elemental-setup-reconcile.timer"),
				systemd.NewUnit("elemental-setup-boot.service"),
				systemd.NewUnit("elemental-setup-network.service"),
				systemd.NewUnit("elemental-setup-fs.service"),
				systemd.NewUnit("elemental-setup-initramfs.service"),
				systemd.NewUnit("elemental-setup-rootfs.service"),
			}
			features = append(features, New(name, units))
		default:
			notFound = append(notFound, name)
		}
	}

	if len(notFound) != 0 {
		return features, fmt.Errorf("Unknown features: %s", strings.Join(notFound, ", "))
	}

	return features, nil
}
