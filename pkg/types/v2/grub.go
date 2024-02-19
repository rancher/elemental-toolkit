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

package v1

import (
	"github.com/rancher/elemental-toolkit/v2/pkg/constants"
)

func (i InstallSpec) GetGrubLabels() map[string]string {
	grubEnv := map[string]string{
		"state_label":    i.Partitions.State.FilesystemLabel,
		"recovery_label": i.Partitions.Recovery.FilesystemLabel,
		"oem_label":      i.Partitions.OEM.FilesystemLabel,
	}

	if i.Partitions.Persistent != nil {
		grubEnv["persistent_label"] = i.Partitions.Persistent.FilesystemLabel
	}

	return grubEnv
}

func (u UpgradeSpec) GetGrubLabels() map[string]string {
	grubVars := map[string]string{
		"state_label":    u.Partitions.State.FilesystemLabel,
		"recovery_label": u.Partitions.Recovery.FilesystemLabel,
		"oem_label":      u.Partitions.OEM.FilesystemLabel,
	}

	if u.Partitions.Persistent != nil {
		grubVars["persistent_label"] = u.Partitions.Persistent.FilesystemLabel
	}

	return grubVars
}

func (r ResetSpec) GetGrubLabels() map[string]string {
	grubVars := map[string]string{
		"state_label":    r.Partitions.State.FilesystemLabel,
		"recovery_label": r.Partitions.Recovery.FilesystemLabel,
		"oem_label":      r.Partitions.OEM.FilesystemLabel,
	}

	if r.State != nil {
		if recoveryPart, ok := r.State.Partitions[constants.RecoveryPartName]; ok {
			grubVars["recovery_label"] = recoveryPart.FSLabel
			grubVars["system_label"] = recoveryPart.RecoveryImage.Label
		}
	}

	if r.Partitions.Persistent != nil {
		grubVars["persistent_label"] = r.Partitions.Persistent.FilesystemLabel
	}

	return grubVars
}

func (d DiskSpec) GetGrubLabels() map[string]string {
	return map[string]string{
		"efi_label":        d.Partitions.EFI.FilesystemLabel,
		"oem_label":        d.Partitions.OEM.FilesystemLabel,
		"recovery_label":   d.Partitions.Recovery.FilesystemLabel,
		"state_label":      d.Partitions.State.FilesystemLabel,
		"persistent_label": d.Partitions.Persistent.FilesystemLabel,
	}
}
