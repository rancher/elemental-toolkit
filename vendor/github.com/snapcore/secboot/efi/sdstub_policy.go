// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package efi

import (
	"errors"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/tcglog-parser"

	secboot_tpm2 "github.com/snapcore/secboot/tpm2"
)

// SystemdStubProfileParams provides the parameters to AddSystemdStubProfile.
type SystemdStubProfileParams struct {
	// PCRAlgorithm is the algorithm for which to compute PCR digests for. TPMs compliant with the "TCG PC Client Platform TPM Profile
	// (PTP) Specification" Level 00, Revision 01.03 v22, May 22 2017 are required to support tpm2.HashAlgorithmSHA1 and
	// tpm2.HashAlgorithmSHA256. Support for other digest algorithms is optional.
	PCRAlgorithm tpm2.HashAlgorithmId

	// PCRIndex is the PCR that the systemd EFI stub measures the kernel commandline to.
	PCRIndex int

	// KernelCmdlines is the set of kernel commandlines to add to the PCR profile.
	KernelCmdlines []string
}

// AddSystemdStubProfile adds the systemd EFI linux loader stub profile to the PCR protection profile, in order to generate a
// PCR policy that restricts access to a key to a defined set of kernel commandlines when booting a linux kernel using the systemd
// EFI stub.
//
// The PCR index that the EFI stub measures the kernel commandline too can be specified via the PCRIndex field of params.
//
// The set of kernel commandlines to add to the PCRProtectionProfile is specified via the KernelCmdlines field of params.
func AddSystemdStubProfile(profile *secboot_tpm2.PCRProtectionProfile, params *SystemdStubProfileParams) error {
	if params.PCRIndex < 0 {
		return errors.New("invalid PCR index")
	}
	if len(params.KernelCmdlines) == 0 {
		return errors.New("no kernel commandlines specified")
	}

	var subProfiles []*secboot_tpm2.PCRProtectionProfile
	for _, cmdline := range params.KernelCmdlines {
		digest := tcglog.ComputeSystemdEFIStubCommandlineDigest(params.PCRAlgorithm.GetHash(), cmdline)
		subProfiles = append(subProfiles, secboot_tpm2.NewPCRProtectionProfile().ExtendPCR(params.PCRAlgorithm, params.PCRIndex, digest))
	}

	profile.AddProfileOR(subProfiles...)
	return nil
}
