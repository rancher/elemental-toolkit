// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"

	"github.com/canonical/go-efilib/internal/ioerr"
)

const EFI_PTAB_HEADER_ID uint64 = 0x5452415020494645

type EFI_PARTITION_ENTRY struct {
	PartitionTypeGUID   EFI_GUID
	UniquePartitionGUID EFI_GUID
	StartingLBA         EFI_LBA
	EndingLBA           EFI_LBA
	Attributes          uint64
	PartitionName       [36]uint16
}

type EFI_PARTITION_TABLE_HEADER struct {
	Hdr                      EFI_TABLE_HEADER
	MyLBA                    EFI_LBA
	AlternateLBA             EFI_LBA
	FirstUsableLBA           EFI_LBA
	LastUsableLBA            EFI_LBA
	DiskGUID                 EFI_GUID
	PartitionEntryLBA        EFI_LBA
	NumberOfPartitionEntries uint32
	SizeOfPartitionEntry     uint32
	PartitionEntryArrayCRC32 uint32
}

func Read_EFI_PARTITION_TABLE_HEADER(r io.Reader) (out *EFI_PARTITION_TABLE_HEADER, crc uint32, err error) {
	var hdr EFI_TABLE_HEADER
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, 0, err
	}
	if hdr.HeaderSize < uint32(binary.Size(hdr)) {
		return nil, 0, errors.New("invalid header size")
	}

	origCrc := hdr.CRC
	hdr.CRC = 0

	b := new(bytes.Buffer)
	if err := binary.Write(b, binary.LittleEndian, &hdr); err != nil {
		return nil, 0, err
	}

	if _, err := io.CopyN(b, r, int64(hdr.HeaderSize-uint32(binary.Size(hdr)))); err != nil {
		return nil, 0, ioerr.EOFIsUnexpected(err)
	}

	crc = crc32.ChecksumIEEE(b.Bytes())

	out = &EFI_PARTITION_TABLE_HEADER{}
	if err := binary.Read(b, binary.LittleEndian, out); err != nil {
		return nil, 0, err
	}
	out.Hdr.CRC = origCrc

	return out, crc, nil
}
