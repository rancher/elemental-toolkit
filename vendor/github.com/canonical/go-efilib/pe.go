// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"crypto"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"

	"golang.org/x/xerrors"

	"github.com/canonical/go-efilib/internal/ioerr"
	"github.com/canonical/go-efilib/internal/pe1.14"
)

const (
	certTableIndex = 4 // Index of the Certificate Table entry in the Data Directory of a PE image optional header
)

type eofIsUnexpectedReaderAt struct {
	r io.ReaderAt
}

func (r *eofIsUnexpectedReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	n, err = r.r.ReadAt(p, off)
	return n, ioerr.EOFIsUnexpected(err)
}

// ComputePeImageDigest computes the digest of the supplied PE image in accordance with the
// Authenticode specification, using the specified digest algorithm.
func ComputePeImageDigest(alg crypto.Hash, r io.ReaderAt, sz int64) ([]byte, error) {
	var dosheader [96]byte
	if n, err := r.ReadAt(dosheader[0:], 0); err != nil {
		if n > 0 && err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	var coffHeaderOffset int64
	if dosheader[0] == 'M' && dosheader[1] == 'Z' {
		signoff := int64(binary.LittleEndian.Uint32(dosheader[0x3c:]))
		var sign [4]byte
		r.ReadAt(sign[:], signoff)
		if !(sign[0] == 'P' && sign[1] == 'E' && sign[2] == 0 && sign[3] == 0) {
			return nil, fmt.Errorf("invalid PE COFF file signature: %v", sign)
		}
		coffHeaderOffset = signoff + 4
	}

	p, err := pe.NewFile(r)
	if err != nil {
		return nil, xerrors.Errorf("cannot decode PE binary: %w", err)
	}

	var isPe32Plus bool
	var sizeOfHeaders int64
	var dd []pe.DataDirectory
	switch oh := p.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		sizeOfHeaders = int64(oh.SizeOfHeaders)
		dd = oh.DataDirectory[0:oh.NumberOfRvaAndSizes]
	case *pe.OptionalHeader64:
		isPe32Plus = true
		sizeOfHeaders = int64(oh.SizeOfHeaders)
		dd = oh.DataDirectory[0:oh.NumberOfRvaAndSizes]
	default:
		return nil, errors.New("PE binary doesn't contain an optional header")
	}

	// 1) Load the image header in to memory.
	hr := io.NewSectionReader(&eofIsUnexpectedReaderAt{r}, 0, sizeOfHeaders)

	// 2) Initialize a hash algorithm context.
	h := alg.New()

	// 3) Hash the image header from its base to immediately before the start of the checksum address in the optional header.
	// This includes the DOS header, 4-byte PE signature, COFF header, and the first 64 bytes of the optional header.
	b := make([]byte, int(coffHeaderOffset)+binary.Size(p.FileHeader)+64)
	if _, err := io.ReadFull(hr, b); err != nil {
		return nil, xerrors.Errorf("cannot read from image to start to checksum: %w", err)
	}
	h.Write(b)

	// 4) Skip over the checksum, which is a 4-byte field.
	hr.Seek(4, io.SeekCurrent)

	var certTable *pe.DataDirectory

	if len(dd) > certTableIndex {
		// 5) Hash everything from the end of the checksum field to immediately before the start of the Certificate Table entry in the
		// optional header data directory.
		// This is 60 bytes for PE32 format binaries, or 76 bytes for PE32+ format binaries.
		sz := 60
		if isPe32Plus {
			sz = 76
		}
		b = make([]byte, sz)
		if _, err := io.ReadFull(hr, b); err != nil {
			return nil, xerrors.Errorf("cannot read from checksum to certificate table data directory entry: %w", err)
		}
		h.Write(b)

		// 6) Get the Attribute Certificate Table address and size from the Certificate Table entry.
		certTable = &dd[certTableIndex]
	}

	// 7) Exclude the Certificate Table entry from the calculation and hash	everything from the end of the Certificate Table entry
	// to the end of image header, including the Section Table. The Certificate Table entry is 8 bytes long.
	if certTable != nil {
		hr.Seek(8, io.SeekCurrent)
	}

	chunkedHashAll := func(r io.Reader, h hash.Hash) error {
		b := make([]byte, 4096)
		for {
			n, err := r.Read(b)
			h.Write(b[:n])

			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}

	if err := chunkedHashAll(hr, h); err != nil {
		return nil, xerrors.Errorf("cannot hash remainder of headers and section table: %w", err)
	}

	// 8) Create a counter called sumOfBytesHashed, which is not part of the signature. Set this counter to the SizeOfHeaders field.
	sumOfBytesHashed := sizeOfHeaders

	// 9) Build a temporary table of pointers to all of the section headers in the image. Do not include any section headers in the
	// table whose Size field is zero.
	var sections []*pe.SectionHeader
	for _, section := range p.Sections {
		if section.Size == 0 {
			continue
		}
		sections = append(sections, &section.SectionHeader)
	}

	// 10) Using the Offset field in the referenced SectionHeader structure as a key, arrange the table's elements in ascending order.
	// In other words, sort the section headers in ascending order according to the disk-file offset of the sections.
	sort.Slice(sections, func(i, j int) bool { return sections[i].Offset < sections[j].Offset })

	for _, section := range sections {
		// 11) Walk through the sorted table, load the corresponding section into memory, and hash the entire section. Use the
		// Size field in the SectionHeader structure to determine the amount of data to hash.
		sr := io.NewSectionReader(&eofIsUnexpectedReaderAt{r}, int64(section.Offset), int64(section.Size))
		if err := chunkedHashAll(sr, h); err != nil {
			return nil, xerrors.Errorf("cannot hash section %s: %w", section.Name, err)
		}

		// 12) Add the section’s Size value to sumOfBytesHashed.
		sumOfBytesHashed += int64(section.Size)

		// 13) Repeat steps 11 and 12 for all of the sections in the sorted table.
	}

	// 14) Create a value called fileSize, which is not part of the signature. Set this value to the image’s file size. If fileSize is
	// greater than sumOfBytesHashed, the file contains extra data that must be added to the hash. This data begins at the
	// sumOfBytesHashed file offset, and its length is:
	// fileSize – (certTable.Size + sumOfBytesHashed)
	fileSize := sz

	if fileSize > sumOfBytesHashed {
		var certSize int64
		if certTable != nil {
			certSize = int64(certTable.Size)
		}

		if fileSize < (sumOfBytesHashed + certSize) {
			return nil, errors.New("image too short")
		}

		sr := io.NewSectionReader(&eofIsUnexpectedReaderAt{r}, sumOfBytesHashed, fileSize-sumOfBytesHashed-certSize)
		if err := chunkedHashAll(sr, h); err != nil {
			return nil, xerrors.Errorf("cannot hash extra data: %w", err)
		}
	}

	return h.Sum(nil), nil
}
