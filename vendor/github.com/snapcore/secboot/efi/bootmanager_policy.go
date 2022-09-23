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

	"github.com/canonical/go-efilib"
	"github.com/canonical/go-tpm2"
	"github.com/canonical/tcglog-parser"

	"golang.org/x/xerrors"

	secboot_tpm2 "github.com/snapcore/secboot/tpm2"
)

// computePeImageDigest computes a hash of a PE image in accordance with the "Windows Authenticode Portable Executable Signature
// Format" specification. This function interprets the byte stream of the raw headers in some places, the layout of which are
// defined in the "PE Format" specification (https://docs.microsoft.com/en-us/windows/win32/debug/pe-format)
func computePeImageDigest(alg tpm2.HashAlgorithmId, image Image) (tpm2.Digest, error) {
	r, err := image.Open()
	if err != nil {
		return nil, xerrors.Errorf("cannot open image: %w", err)
	}
	defer r.Close()

	return efi.ComputePeImageDigest(alg.GetHash(), r, r.Size())
}

type bootManagerCodePolicyGenBranch struct {
	profile  *secboot_tpm2.PCRProtectionProfile
	branches []*secboot_tpm2.PCRProtectionProfile
}

func (n *bootManagerCodePolicyGenBranch) branch() *bootManagerCodePolicyGenBranch {
	b := &bootManagerCodePolicyGenBranch{profile: secboot_tpm2.NewPCRProtectionProfile()}
	n.branches = append(n.branches, b.profile)
	return b
}

// bmLoadEventAndBranch binds together a ImageLoadEvent and the branch that the event needs to be applied to.
type bmLoadEventAndBranch struct {
	event  *ImageLoadEvent
	branch *bootManagerCodePolicyGenBranch
}

// BootManagerProfileParams provide the arguments to AddBootManagerProfile.
type BootManagerProfileParams struct {
	// PCRAlgorithm is the algorithm for which to compute PCR digests for. TPMs compliant with the "TCG PC Client Platform TPM Profile
	// (PTP) Specification" Level 00, Revision 01.03 v22, May 22 2017 are required to support tpm2.HashAlgorithmSHA1 and
	// tpm2.HashAlgorithmSHA256. Support for other digest algorithms is optional.
	PCRAlgorithm tpm2.HashAlgorithmId

	// LoadSequences is a list of EFI image load sequences for which to compute PCR digests for.
	LoadSequences []*ImageLoadEvent

	// Environment is an optional parameter that allows the caller to provide
	// a custom EFI environment. If not set, the host's normal environment will
	// be used
	Environment HostEnvironment
}

// AddBootManagerProfile adds the UEFI boot manager code and boot attempts profile to the provided PCR protection profile, in order
// to generate a PCR policy that restricts access to a sealed key to a specific set of binaries started from the UEFI boot manager and
// which are measured to PCR 4. Events that are measured to this PCR are detailed in section 2.3.4.5 of the "TCG PC Client Platform
// Firmware Profile Specification".
//
// If the firmware supports executing system preparation applications before the transition to "OS present", events corresponding to
// the launch of these applications will be measured to PCR 4. If the event log indicates that any system preparation applications
// were executed during the current boot, this function will automatically include these binaries in the generated PCR profile. Note
// that it is not possible to pre-compute PCR values for system preparation applications using this function, and so it is not
// possible to update these in a way that is atomic (if any of them are changed, a new PCR profile can only be generated after
// performing a reboot).
//
// The sequences of binaries for which to generate a PCR profile for is supplied via the LoadSequences field of params. Note that
// this function does not use the Source field of EFIImageLoadEvent. Each bootloader stage in each load sequence must perform a
// measurement of any subsequent stage to PCR 4 in the same format as the events measured by the UEFI boot manager.
//
// Section 2.3.4.5 of the "TCG PC Client Platform Firmware Profile Specification" specifies that EFI applications that load additional
// pre-OS environment code must measure this to PCR 4 using the EV_COMPACT_HASH event type. This function does not support EFI
// applications that load additional pre-OS environment code that isn't otherwise authenticated via the secure boot mechanism,
// and will generate PCR profiles that aren't correct for applications that do this.
//
// If the EV_OMIT_BOOT_DEVICE_EVENTS is not recorded to PCR 4, the platform firmware will perform meaurements of all boot attempts,
// even if they fail. The generated PCR policy will not be satisfied if the platform firmware performs boot attempts that fail,
// even if the successful boot attempt is of a sequence of binaries included in this PCR profile.
func AddBootManagerProfile(profile *secboot_tpm2.PCRProtectionProfile, params *BootManagerProfileParams) error {
	env := params.Environment
	if env == nil {
		env = defaultEnv
	}

	// Load event log
	log, err := env.ReadEventLog()
	if err != nil {
		return xerrors.Errorf("cannot parse TCG event log: %w", err)
	}

	if !log.Algorithms.Contains(params.PCRAlgorithm) {
		return errors.New("cannot compute secure boot policy digests: the TCG event log does not have the requested algorithm")
	}

	profile.AddPCRValue(params.PCRAlgorithm, bootManagerCodePCR, make(tpm2.Digest, params.PCRAlgorithm.Size()))

	// Replay the event log until we see the transition from "pre-OS" to "OS-present". The event log may contain measurements
	// for system preparation applications, and spec-compliant firmware should measure a EV_EFI_ACTION “Calling EFI Application
	// from Boot Option” event before the EV_SEPARATOR event, but not all firmware does this.
	for _, event := range log.Events {
		if event.PCRIndex != bootManagerCodePCR {
			continue
		}

		profile.ExtendPCR(params.PCRAlgorithm, bootManagerCodePCR, tpm2.Digest(event.Digests[params.PCRAlgorithm]))
		if event.EventType == tcglog.EventTypeSeparator {
			break
		}
	}

	root := bootManagerCodePolicyGenBranch{profile: profile}
	allBranches := []*bootManagerCodePolicyGenBranch{&root}

	var loadEvents []*bmLoadEventAndBranch
	var nextLoadEvents []*bmLoadEventAndBranch

	if len(params.LoadSequences) == 1 {
		loadEvents = append(loadEvents, &bmLoadEventAndBranch{event: params.LoadSequences[0], branch: &root})
	} else {
		for _, e := range params.LoadSequences {
			branch := root.branch()
			allBranches = append(allBranches, branch)
			loadEvents = append(loadEvents, &bmLoadEventAndBranch{event: e, branch: branch})
		}
	}

	for len(loadEvents) > 0 {
		e := loadEvents[0]
		loadEvents = loadEvents[1:]

		digest, err := computePeImageDigest(params.PCRAlgorithm, e.event.Image)
		if err != nil {
			return err
		}
		e.branch.profile.ExtendPCR(params.PCRAlgorithm, bootManagerCodePCR, digest)

		if len(e.event.Next) == 1 {
			nextLoadEvents = append(nextLoadEvents, &bmLoadEventAndBranch{event: e.event.Next[0], branch: e.branch})
		} else {
			for _, n := range e.event.Next {
				branch := e.branch.branch()
				nextLoadEvents = append(nextLoadEvents, &bmLoadEventAndBranch{event: n, branch: branch})
				allBranches = append(allBranches, branch)
			}
		}

		if len(loadEvents) == 0 {
			loadEvents = nextLoadEvents
			nextLoadEvents = nil
		}
	}

	// Iterate over all of the branch points starting from the root and creates a tree of
	// sub-profiles with AddProfileOR. The ordering doesn't matter here, because each subprofile
	// is already complete
	for _, b := range allBranches {
		if len(b.branches) == 0 {
			continue
		}
		b.profile.AddProfileOR(b.branches...)
	}

	return nil
}
