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
	"archive/tar"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rancher/elemental-toolkit/v2/pkg/systemd"
	"github.com/rancher/elemental-toolkit/v2/pkg/types"
	"github.com/rancher/elemental-toolkit/v2/pkg/utils"
)

// Generate a tarball for each feature in ./embedded and put them in
// ./generated.
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
	FeatureBootAssessment        = "boot-assessment"
	FeatureAutologin             = "autologin"
	FeatureArmFirmware           = "arm-firmware"
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
		FeatureBootAssessment,
		FeatureAutologin,
		FeatureArmFirmware,
	}

	Default = []string{
		FeatureElementalRootfs,
		FeatureElementalSysroot,
		FeatureGrubConfig,
		FeatureGrubDefaultBootargs,
		FeatureElementalSetup,
		FeatureDracutConfig,
		FeatureCloudConfigDefaults,
		FeatureCloudConfigEssentials,
		FeatureBootAssessment,
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

func (f *Feature) Install(log types.Logger, destFs types.FS, runner types.Runner) error {
	path := filepath.Join(embeddedRoot, fmt.Sprintf("%s.tar.gz", f.Name))
	tar, err := files.Open(path)
	if err != nil {
		log.Errorf("Error opening '%s': %s", path, err.Error())
		return err
	}

	err = extractTarGzip(log, tar, destFs, f.Name)
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
			return features, fmt.Errorf("Feature immutable-rootfs no longer supported, please use 'elemental-rootfs' instead")
		case FeatureElementalRootfs:
			if slices.Contains(names, FeatureImmutableRootfs) {
				return features, fmt.Errorf("conflicting features: %s and %s", FeatureElementalRootfs, FeatureImmutableRootfs)
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
		case FeatureBootAssessment:
			units := []*systemd.Unit{
				systemd.NewUnit("elemental-boot-assessment.service"),
			}
			features = append(features, New(name, units))
		case FeatureAutologin:
			features = append(features, New(name, nil))
		case FeatureArmFirmware:
			features = append(features, New(name, nil))
		default:
			notFound = append(notFound, name)
		}
	}

	if len(notFound) != 0 {
		return features, fmt.Errorf("unknown features: %s", strings.Join(notFound, ", "))
	}

	return features, nil
}

func extractTarGzip(log types.Logger, tarFile io.Reader, destFs types.FS, featureName string) error {
	gzipReader, err := gzip.NewReader(tarFile)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)

	for {
		header, err := reader.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		log.Debugf("Extracting %s for feature %s", header.Name, featureName)
		trimmed := strings.TrimPrefix(header.Name, featureName)
		target := filepath.Join("/", trimmed)

		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := destFs.Stat(target); err != nil {
				if err := utils.MkdirAll(destFs, target, fs.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			destFile, err := destFs.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				log.Errorf("Error opening file '%s': %s", target, err.Error())
				return err
			}

			written, err := io.Copy(destFile, reader)
			if err != nil {
				log.Errorf("Error copying file '%s': %s", target, err.Error())
				return err
			}

			log.Debugf("Wrote %d bytes to %s", written, target)

			_ = destFile.Close()

		}
	}
}
