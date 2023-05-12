// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package mbr

import (
	"encoding/binary"
	"errors"
	"io"
)

const mbrSignature = 0xaa55

var ErrInvalidSignature = errors.New("invalid master boot record signature")

// Address is a CHS address.
type Address [3]uint8

func (a Address) Head() uint8 {
	return a[0]
}

func (a Address) Sector() uint8 {
	return a[1] & 0x3f
}

func (a Address) Cylinder() uint16 {
	c := uint16(a[2])
	c |= uint16(a[1]&0xc0) << 2
	return c
}

// PartitionEntry corresponds to a partition entry from a MBR.
type PartitionEntry struct {
	BootIndicator   uint8
	StartAddress    Address
	Type            uint8
	EndAddress      Address
	StartingLBA     uint32
	NumberOfSectors uint32
}

// Record corresponds to a MBR.
type Record struct {
	BootstrapCode   [440]byte
	UniqueSignature uint32
	Partitions      [4]PartitionEntry
}

type record struct {
	BootstrapCode   [440]byte
	UniqueSignature uint32
	Unknown         [2]uint8
	Partitions      [4]PartitionEntry
	Signature       uint16
}

// ReadRecord reads a MBR from r. It returns ErrInvalidSignature if the
// MBR has an invalid signature.
func ReadRecord(r io.Reader) (*Record, error) {
	var rec record
	if err := binary.Read(r, binary.LittleEndian, &rec); err != nil {
		return nil, err
	}
	if rec.Signature != mbrSignature {
		return nil, ErrInvalidSignature
	}
	return &Record{BootstrapCode: rec.BootstrapCode,
		UniqueSignature: rec.UniqueSignature,
		Partitions:      rec.Partitions}, nil
}
