// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"strings"

	"github.com/canonical/go-efilib/internal/uefi"
	"github.com/canonical/go-efilib/mbr"
)

var (
	ErrCRCCheck         = errors.New("CRC check failed")                       // the partition table header or partition entry CRC check failed
	ErrNoProtectiveMBR  = errors.New("no protective master boot record found") // no protective MBR record was found
	ErrStandardMBRFound = errors.New("a MBR was found")                        // a standard MBR was found

	// ErrInvalidBackupPartitionTableLocation may be returned from
	// ReadPartitionTable when called with the BackupPartitionTable
	// role if the partition table isn't located at the end of the
	// device. Note that the function will still return a valid table
	// in this case.
	ErrInvalidBackupPartitionTableLocation = errors.New("backup partition table not located at end of device")

	// UnusedPartitionType is the type GUID of an unused partition entry.
	UnusedPartitionType GUID
)

type InvalidGPTHeaderError string

func (e InvalidGPTHeaderError) Error() string {
	return "invalid GPT header: " + string(e)
}

// PartitionTableHeader correponds to the EFI_PARTITION_TABLE_HEADER type.
type PartitionTableHeader struct {
	HeaderSize               uint32
	MyLBA                    LBA
	AlternateLBA             LBA
	FirstUsableLBA           LBA
	LastUsableLBA            LBA
	DiskGUID                 GUID
	PartitionEntryLBA        LBA
	NumberOfPartitionEntries uint32
	SizeOfPartitionEntry     uint32
	PartitionEntryArrayCRC32 uint32
}

// ReadPartitionTableHeader reads a EFI_PARTITION_TABLE_HEADER from the supplied io.Reader.
// If the header signature or revision is incorrect, an error will be returned. If
// checkCrc is true and the header has an invalid CRC, an error will be returned.
// If checkCrc is false, then a CRC check is not performed.
func ReadPartitionTableHeader(r io.Reader, checkCrc bool) (*PartitionTableHeader, error) {
	hdr, crc, err := uefi.Read_EFI_PARTITION_TABLE_HEADER(r)
	if err != nil {
		return nil, err
	}
	if hdr.Hdr.Signature != uefi.EFI_PTAB_HEADER_ID {
		return nil, InvalidGPTHeaderError("invalid signature")
	}
	if hdr.Hdr.Revision != 0x10000 {
		return nil, InvalidGPTHeaderError("unexpected revision")
	}
	if checkCrc && hdr.Hdr.CRC != crc {
		return nil, ErrCRCCheck
	}

	return &PartitionTableHeader{
		HeaderSize:               hdr.Hdr.HeaderSize,
		MyLBA:                    LBA(hdr.MyLBA),
		AlternateLBA:             LBA(hdr.AlternateLBA),
		FirstUsableLBA:           LBA(hdr.FirstUsableLBA),
		LastUsableLBA:            LBA(hdr.LastUsableLBA),
		DiskGUID:                 GUID(hdr.DiskGUID),
		PartitionEntryLBA:        LBA(hdr.PartitionEntryLBA),
		NumberOfPartitionEntries: hdr.NumberOfPartitionEntries,
		SizeOfPartitionEntry:     hdr.SizeOfPartitionEntry,
		PartitionEntryArrayCRC32: hdr.PartitionEntryArrayCRC32}, nil
}

// Write serializes this PartitionTableHeader to w. The CRC field is
// computed automatically.
func (h *PartitionTableHeader) Write(w io.Writer) error {
	hdr := uefi.EFI_PARTITION_TABLE_HEADER{
		Hdr: uefi.EFI_TABLE_HEADER{
			Signature:  uefi.EFI_PTAB_HEADER_ID,
			Revision:   0x10000,
			HeaderSize: h.HeaderSize},
		MyLBA:                    uefi.EFI_LBA(h.MyLBA),
		AlternateLBA:             uefi.EFI_LBA(h.AlternateLBA),
		FirstUsableLBA:           uefi.EFI_LBA(h.FirstUsableLBA),
		LastUsableLBA:            uefi.EFI_LBA(h.LastUsableLBA),
		DiskGUID:                 uefi.EFI_GUID(h.DiskGUID),
		PartitionEntryLBA:        uefi.EFI_LBA(h.PartitionEntryLBA),
		NumberOfPartitionEntries: h.NumberOfPartitionEntries,
		SizeOfPartitionEntry:     h.SizeOfPartitionEntry,
		PartitionEntryArrayCRC32: h.PartitionEntryArrayCRC32}

	hdrSize := binary.Size(hdr)
	if h.HeaderSize < uint32(hdrSize) {
		return errors.New("invalid HeaderSize")
	}

	reserved := make([]byte, int(h.HeaderSize)-hdrSize)

	crc := crc32.NewIEEE()
	binary.Write(crc, binary.LittleEndian, &hdr)
	crc.Write(reserved)

	hdr.Hdr.CRC = crc.Sum32()

	if err := binary.Write(w, binary.LittleEndian, &hdr); err != nil {
		return err
	}
	_, err := w.Write(reserved)
	return err
}

func (h *PartitionTableHeader) String() string {
	return fmt.Sprintf(`EFI_PARTITION_TABLE_HEADER {
	MyLBA: %#x,
	AlternateLBA: %#x,
	FirstUsableLBA: %#x,
	LastUsableLBA: %#x,
	DiskGUID: %v,
	PartitionEntryLBA: %#x,
	NumberOfPartitionEntries: %d,
	SizeOfPartitionEntry: %#x,
	PartitionEntryArrayCRC32: %#08x,
}`, h.MyLBA, h.AlternateLBA, h.FirstUsableLBA, h.LastUsableLBA, h.DiskGUID, h.PartitionEntryLBA,
		h.NumberOfPartitionEntries, h.SizeOfPartitionEntry, h.PartitionEntryArrayCRC32)
}

// PartitionEntry corresponds to the EFI_PARTITION_ENTRY type.
type PartitionEntry struct {
	PartitionTypeGUID   GUID
	UniquePartitionGUID GUID
	StartingLBA         LBA
	EndingLBA           LBA
	Attributes          uint64
	PartitionName       string
}

// ReadPartitionEntry reads a single EFI_PARTITION_ENTRY from r.
func ReadPartitionEntry(r io.Reader) (*PartitionEntry, error) {
	var e uefi.EFI_PARTITION_ENTRY
	if err := binary.Read(r, binary.LittleEndian, &e); err != nil {
		return nil, err
	}

	return &PartitionEntry{
		PartitionTypeGUID:   GUID(e.PartitionTypeGUID),
		UniquePartitionGUID: GUID(e.UniquePartitionGUID),
		StartingLBA:         LBA(e.StartingLBA),
		EndingLBA:           LBA(e.EndingLBA),
		Attributes:          e.Attributes,
		PartitionName:       ConvertUTF16ToUTF8(e.PartitionName[:])}, nil
}

// String implements [fmt.Stringer].
func (e *PartitionEntry) String() string {
	return fmt.Sprintf(`EFI_PARTITION_ENTRY {
	PartitionTypeGUID: %s,
	UniquePartitionGUID: %s,
	StartingLBA: 0x%x,
	EndingLBA: %#x,
	Attributes: %#016x,
	PartitionName: %q,
}`, e.PartitionTypeGUID, e.UniquePartitionGUID, e.StartingLBA, e.EndingLBA, e.Attributes, e.PartitionName)
}

// Write serializes this PartitionEntry to w. Note that it doesn't write
// any bytes beyond the end of the EFI_PARTITION_ENTRY structure, so if the
// caller is writing several entries and the partition table header defines
// an entry size of greater than 128 bytes, the caller is responsible for
// inserting the 0 padding bytes.
func (e *PartitionEntry) Write(w io.Writer) error {
	entry := uefi.EFI_PARTITION_ENTRY{
		PartitionTypeGUID:   uefi.EFI_GUID(e.PartitionTypeGUID),
		UniquePartitionGUID: uefi.EFI_GUID(e.UniquePartitionGUID),
		StartingLBA:         uefi.EFI_LBA(e.StartingLBA),
		EndingLBA:           uefi.EFI_LBA(e.EndingLBA),
		Attributes:          e.Attributes}

	partitionName := ConvertUTF8ToUTF16(e.PartitionName)
	if len(partitionName) > len(entry.PartitionName) {
		return errors.New("PartitionName is too long")
	}
	copy(entry.PartitionName[:], partitionName)

	return binary.Write(w, binary.LittleEndian, &entry)
}

func readPartitionEntries(r io.Reader, num, sz, expectedCrc uint32, checkCrc bool) (out []*PartitionEntry, err error) {
	crc := crc32.NewIEEE()
	r2 := io.TeeReader(r, crc)

	var buf bytes.Buffer
	for i := uint32(0); i < num; i++ {
		buf.Reset()

		if _, err := io.CopyN(&buf, r2, int64(sz)); err != nil {
			switch {
			case err == io.EOF && i == 0:
				return nil, err
			case err == io.EOF:
				err = io.ErrUnexpectedEOF
			}
			return nil, fmt.Errorf("cannot read entry %d: %w", i, err)
		}

		e, err := ReadPartitionEntry(&buf)
		if err != nil {
			return nil, err
		}

		out = append(out, e)
	}

	if checkCrc && crc.Sum32() != expectedCrc {
		return nil, ErrCRCCheck
	}

	return out, nil
}

// ReadPartitionEntries reads the specified number of EFI_PARTITION_ENTRY structures
// of the specified size from the supplied io.Reader. The number and size are typically
// defined by the partition table header.
func ReadPartitionEntries(r io.Reader, num, sz uint32) ([]*PartitionEntry, error) {
	return readPartitionEntries(r, num, sz, 0, false)
}

var emptyPartitionType GUID

// PartitionTableRole describes the role of a partition table.
type PartitionTableRole int

const (
	PrimaryPartitionTable PartitionTableRole = iota
	BackupPartitionTable
)

// PartitionTable describes a complete GUID partition table.
type PartitionTable struct {
	Hdr     *PartitionTableHeader
	Entries []*PartitionEntry
}

// String implements [fmt.Stringer].
func (t *PartitionTable) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, `GPT {
	Hdr: %s,
	Entries: [`, indent(t.Hdr, 1))
	for i, entry := range t.Entries {
		if entry.PartitionTypeGUID == UnusedPartitionType {
			continue
		}
		fmt.Fprintf(&b, "\n\t\t%d: %s,", i, indent(entry, 2))
	}
	b.WriteString("\n\t],\n}")
	return b.String()
}

func readPartitionTable(r io.ReadSeeker, blockSz, offset int64, whence int, checkCrc bool) (*PartitionTable, error) {
	if _, err := r.Seek(offset, whence); err != nil {
		return nil, err
	}

	hdr, err := ReadPartitionTableHeader(r, checkCrc)
	switch {
	case err == io.EOF:
		return nil, io.ErrUnexpectedEOF
	case err != nil:
		return nil, err
	}

	if _, err := r.Seek(int64(hdr.PartitionEntryLBA)*blockSz, io.SeekStart); err != nil {
		return nil, err
	}

	entries, err := readPartitionEntries(r, hdr.NumberOfPartitionEntries, hdr.SizeOfPartitionEntry, hdr.PartitionEntryArrayCRC32, checkCrc)
	switch {
	case err == io.EOF:
		return nil, io.ErrUnexpectedEOF
	case err != nil:
		return nil, err
	}

	return &PartitionTable{hdr, entries}, nil
}

// ReadPartitionTable reads a complete GUID partition table from the supplied
// io.Reader. The total size and logical block size of the device must be
// supplied - the logical block size is 512 bytes for a file, but must be
// obtained from the kernel for a block device.
//
// This function expects the device to have a valid protective MBR.
//
// If role is PrimaryPartitionTable, this will read the primary partition
// table that is located immediately after the protective MBR. If role is
// BackupPartitionTable, this will read the backup partition table that is
// located at the end of the device.
//
// If checkCrc is true and either CRC check fails for the requested table, an
// error will be returned. Setting checkCrc to false disables the CRC checks.
//
// Note that whilst this function checks the integrity of the header and
// partition table entries, it does not check the contents of the partition
// table entries.
//
// If role is BackupPartitionTable and the backup table is not located at
// the end of the device, this will return ErrInvalidBackupPartitionTableLocation
// along with the valid table.
func ReadPartitionTable(r io.ReaderAt, totalSz, blockSz int64, role PartitionTableRole, checkCrc bool) (*PartitionTable, error) {
	r2 := io.NewSectionReader(r, 0, totalSz)

	record, err := mbr.ReadRecord(r2)
	switch {
	case errors.Is(err, mbr.ErrInvalidSignature):
		return nil, ErrNoProtectiveMBR
	case err != nil:
		return nil, err
	case !record.IsProtectiveMBR():
		return nil, ErrStandardMBRFound
	}

	switch role {
	case PrimaryPartitionTable:
		return readPartitionTable(r2, blockSz, blockSz, io.SeekStart, checkCrc)
	case BackupPartitionTable:
		var offset int64
		var whence int

		primary, primaryErr := readPartitionTable(r2, blockSz, blockSz, io.SeekStart, checkCrc)
		if primaryErr != nil {
			offset = -blockSz
			whence = io.SeekEnd
		} else {
			offset = int64(primary.Hdr.AlternateLBA) * blockSz
			whence = io.SeekStart
		}

		backup, err := readPartitionTable(r2, blockSz, offset, whence, checkCrc)
		if err != nil {
			return nil, err
		}

		if primaryErr == nil && offset != totalSz-blockSz {
			return backup, ErrInvalidBackupPartitionTableLocation
		}
		return backup, nil

	default:
		panic("invalid role")
	}
}
