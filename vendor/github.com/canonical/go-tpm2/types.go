// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

import (
	"errors"

	"github.com/canonical/go-tpm2/mu"
)

// TPMManufacturer corresponds to the TPM manufacturer and is returned when querying the value PropertyManufacturer with
// TPMContext.GetCapabilityTPMProperties
type TPMManufacturer uint32

const (
	TPMManufacturerAMD  TPMManufacturer = 0x414D4400 // AMD
	TPMManufacturerATML TPMManufacturer = 0x41544D4C // Atmel
	TPMManufacturerBRCM TPMManufacturer = 0x4252434D // Broadcom
	TPMManufacturerHPE  TPMManufacturer = 0x48504500 // HPE
	TPMManufacturerIBM  TPMManufacturer = 0x49424d00 // IBM
	TPMManufacturerIFX  TPMManufacturer = 0x49465800 // Infineon
	TPMManufacturerINTC TPMManufacturer = 0x494E5443 // Intel
	TPMManufacturerLEN  TPMManufacturer = 0x4C454E00 // Lenovo
	TPMManufacturerMSFT TPMManufacturer = 0x4D534654 // Microsoft
	TPMManufacturerNSM  TPMManufacturer = 0x4E534D20 // National Semiconductor
	TPMManufacturerNTZ  TPMManufacturer = 0x4E545A00 // Nationz
	TPMManufacturerNTC  TPMManufacturer = 0x4E544300 // Nuvoton Technology
	TPMManufacturerQCOM TPMManufacturer = 0x51434F4D // Qualcomm
	TPMManufacturerSMSC TPMManufacturer = 0x534D5343 // SMSC
	TPMManufacturerSTM  TPMManufacturer = 0x53544D20 // ST Microelectronics
	TPMManufacturerSMSN TPMManufacturer = 0x534D534E // Samsung
	TPMManufacturerSNS  TPMManufacturer = 0x534E5300 // Sinosun
	TPMManufacturerTXN  TPMManufacturer = 0x54584E00 // Texas Instruments
	TPMManufacturerWEC  TPMManufacturer = 0x57454300 // Winbond
	TPMManufacturerROCC TPMManufacturer = 0x524F4343 // Fuzhou Rockchip
	TPMManufacturerGOOG TPMManufacturer = 0x474F4F47 // Google
)

// PCRValues contains a collection of PCR values, keyed by HashAlgorithmId and PCR index.
type PCRValues map[HashAlgorithmId]map[int]Digest

// SelectionList computes a list of PCR selections corresponding to this set of PCR values.
func (v PCRValues) SelectionList() PCRSelectionList {
	var out PCRSelectionList
	for h := range v {
		s := PCRSelection{Hash: h}
		for p := range v[h] {
			s.Select = append(s.Select, p)
		}
		out = append(out, s)
	}
	return out.Sort()
}

// ToListAndSelection converts this set of PCR values to a list of PCR selections and list of PCR
// values, in a form that can be serialized.
func (v PCRValues) ToListAndSelection() (pcrs PCRSelectionList, digests DigestList) {
	pcrs = v.SelectionList()
	for _, p := range pcrs {
		for _, s := range p.Select {
			digests = append(digests, v[p.Hash][s])
		}
	}
	return
}

// SetValuesFromListAndSelection sets PCR values from the supplied list of PCR selections and list
// of values.
func (v PCRValues) SetValuesFromListAndSelection(pcrs PCRSelectionList, digests DigestList) (int, error) {
	// Copy the selections so that each selection is ordered correctly
	mu.MustCopyValue(&pcrs, pcrs)

	i := 0
	for _, p := range pcrs {
		if _, ok := v[p.Hash]; !ok {
			v[p.Hash] = make(map[int]Digest)
		}
		for _, s := range p.Select {
			if len(digests) == 0 {
				return 0, errors.New("insufficient digests")
			}
			d := digests[0]
			digests = digests[1:]
			if len(d) != p.Hash.Size() {
				return 0, errors.New("incorrect digest size")
			}
			v[p.Hash][s] = d
			i++
		}
	}
	return i, nil
}

// SetValue sets the PCR value for the specified PCR and PCR bank.
func (v PCRValues) SetValue(alg HashAlgorithmId, pcr int, digest Digest) {
	if _, ok := v[alg]; !ok {
		v[alg] = make(map[int]Digest)
	}
	v[alg][pcr] = digest
}

// PublicTemplate exists to allow either Public or PublicDerived structures
// to be used as the template value for TPMContext.CreateLoaded.
type PublicTemplate interface {
	ToTemplate() (Template, error)
}
