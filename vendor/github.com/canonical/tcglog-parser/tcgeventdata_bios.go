// Copyright 2019-2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/canonical/tcglog-parser/internal/ioerr"
)

type rawSpecIdEvent00Hdr struct {
	PlatformClass    uint32
	SpecVersionMinor uint8
	SpecVersionMajor uint8
	SpecErrata       uint8
	Reserved         uint8
	VendorInfoSize   uint8
}

// SpecIdEvent00 corresponds to the TCG_PCClientSpecIdEventStruct type and is the
// event data for a Specification ID Version EV_NO_ACTION event for BIOS platforms.
type SpecIdEvent00 struct {
	rawEventData
	PlatformClass    uint32
	SpecVersionMinor uint8
	SpecVersionMajor uint8
	SpecErrata       uint8
	VendorInfo       []byte
}

func (e *SpecIdEvent00) String() string {
	return fmt.Sprintf("PCClientSpecIdEvent{ platformClass=%d, specVersionMinor=%d, specVersionMajor=%d, specErrata=%d }",
		e.PlatformClass, e.SpecVersionMinor, e.SpecVersionMajor, e.SpecErrata)
}

func (e *SpecIdEvent00) Write(w io.Writer) error {
	vendorInfoSize := len(e.VendorInfo)
	if vendorInfoSize > math.MaxUint8 {
		return errors.New("VendorInfo too large")
	}

	var signature [16]byte
	copy(signature[:], []byte("Spec ID Event00"))
	if _, err := w.Write(signature[:]); err != nil {
		return err
	}

	spec := rawSpecIdEvent00Hdr{
		PlatformClass:    e.PlatformClass,
		SpecVersionMinor: e.SpecVersionMinor,
		SpecVersionMajor: e.SpecVersionMajor,
		SpecErrata:       e.SpecErrata,
		VendorInfoSize:   uint8(vendorInfoSize)}
	if err := binary.Write(w, binary.LittleEndian, &spec); err != nil {
		return err
	}

	_, err := w.Write(e.VendorInfo)
	return err
}

// https://trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientImplementation_1-21_1_00.pdf
//  (section 11.3.4.1 "Specification Event")
func decodeSpecIdEvent00(data []byte, r io.Reader) (out *SpecIdEvent00, err error) {
	var spec rawSpecIdEvent00Hdr
	if err := binary.Read(r, binary.LittleEndian, &spec); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	out = &SpecIdEvent00{
		rawEventData:     data,
		PlatformClass:    spec.PlatformClass,
		SpecVersionMinor: spec.SpecVersionMinor,
		SpecVersionMajor: spec.SpecVersionMajor,
		SpecErrata:       spec.SpecErrata,
		VendorInfo:       make([]byte, spec.VendorInfoSize)}
	if _, err := io.ReadFull(r, out.VendorInfo); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return out, nil
}
