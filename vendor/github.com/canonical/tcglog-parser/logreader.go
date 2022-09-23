// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"io"

	"github.com/canonical/go-tpm2"
)

// LogOptions allows the behaviour of Log to be controlled.
type LogOptions struct {
	EnableGrub           bool     // Enable support for interpreting events recorded by GRUB
	EnableSystemdEFIStub bool     // Enable support for interpreting events recorded by systemd's EFI linux loader stub
	SystemdEFIStubPCR    PCRIndex // Specify the PCR that systemd's EFI linux loader stub measures to
}

func fixupSpecIdEvent(event *Event, algorithms AlgorithmIdList) {
	for _, alg := range algorithms {
		if alg == tpm2.HashAlgorithmSHA1 {
			continue
		}

		if _, ok := event.Digests[alg]; ok {
			continue
		}

		event.Digests[alg] = make(Digest, alg.Size())
	}
}

type PlatformType int

const (
	PlatformTypeUnknown PlatformType = iota
	PlatformTypeBIOS
	PlatformTypeEFI
)

// Spec corresponds to the TCG specification that an event log conforms to.
type Spec struct {
	PlatformType PlatformType
	Major        uint8
	Minor        uint8
	Errata       uint8
}

// IsBIOS indicates that a log conforms to "TCG PC Client Specific Implementation Specification
// for Conventional BIOS".
// See https://www.trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientImplementation_1-21_1_00.pdf
func (s Spec) IsBIOS() bool { return s.PlatformType == PlatformTypeBIOS }

// IsEFI_1_2 indicates that a log conforms to "TCG EFI Platform Specification For TPM Family 1.1 or
// 1.2".
// See https://trustedcomputinggroup.org/wp-content/uploads/TCG_EFI_Platform_1_22_Final_-v15.pdf
func (s Spec) IsEFI_1_2() bool {
	return s.PlatformType == PlatformTypeEFI && s.Major == 1 && s.Minor == 2
}

// IsEFI_2 indicates that a log conforms to "TCG PC Client Platform Firmware Profile Specification"
// See https://trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientSpecPlat_TPM_2p0_1p04_pub.pdf
func (s Spec) IsEFI_2() bool {
	return s.PlatformType == PlatformTypeEFI && s.Major == 2
}

// Log corresponds to a parsed event log.
type Log struct {
	Spec       Spec            // The specification to which this log conforms
	Algorithms AlgorithmIdList // The digest algorithms that appear in the log
	Events     []*Event        // The list of events in the log
}

// ReadLog reads an event log read from r using the supplied options. The log must
// be in the format defined in one of the PC Client Platform Firmware Profile
// specifications. If an error occurs during parsing, this may return an incomplete
// list of events with the error.
func ReadLog(r io.Reader, options *LogOptions) (*Log, error) {
	event, err := ReadEvent(r, options)
	if err != nil {
		return nil, err
	}

	var spec Spec
	var digestSizes []EFISpecIdEventAlgorithmSize

	switch d := event.Data.(type) {
	case *SpecIdEvent00:
		spec = Spec{
			PlatformType: PlatformTypeBIOS,
			Major:        d.SpecVersionMajor,
			Minor:        d.SpecVersionMinor,
			Errata:       d.SpecErrata}
	case *SpecIdEvent02:
		spec = Spec{
			PlatformType: PlatformTypeEFI,
			Major:        d.SpecVersionMajor,
			Minor:        d.SpecVersionMinor,
			Errata:       d.SpecErrata}
	case *SpecIdEvent03:
		spec = Spec{
			PlatformType: PlatformTypeEFI,
			Major:        d.SpecVersionMajor,
			Minor:        d.SpecVersionMinor,
			Errata:       d.SpecErrata}
		digestSizes = d.DigestSizes
	}

	var algorithms AlgorithmIdList

	if spec.IsEFI_2() {
		for _, s := range digestSizes {
			if s.AlgorithmId.IsValid() {
				algorithms = append(algorithms, s.AlgorithmId)
			}
		}
	} else {
		algorithms = AlgorithmIdList{tpm2.HashAlgorithmSHA1}
	}

	if spec.IsEFI_2() {
		fixupSpecIdEvent(event, algorithms)
	}

	log := &Log{Spec: spec, Algorithms: algorithms, Events: []*Event{event}}

	for {
		var event *Event
		var err error
		if spec.IsEFI_2() {
			event, err = ReadEventCryptoAgile(r, digestSizes, options)
		} else {
			event, err = ReadEvent(r, options)
		}

		switch {
		case err == io.EOF:
			return log, nil
		case err != nil:
			return log, err
		default:
			log.Events = append(log.Events, event)
		}
	}
}
