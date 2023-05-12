// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

import (
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/canonical/go-efilib/internal/ioerr"
)

const ESLHeaderSize = 28

type EFI_SIGNATURE_DATA struct {
	SignatureOwner EFI_GUID
	SignatureData  []byte
}

func (d *EFI_SIGNATURE_DATA) Write(w io.Writer) error {
	if _, err := w.Write(d.SignatureOwner[:]); err != nil {
		return err
	}
	if _, err := w.Write(d.SignatureData); err != nil {
		return err
	}
	return nil
}

type EFI_SIGNATURE_LIST struct {
	SignatureType       EFI_GUID
	SignatureListSize   uint32
	SignatureHeaderSize uint32
	SignatureSize       uint32

	SignatureHeader []byte
	Signatures      []EFI_SIGNATURE_DATA
}

func (l *EFI_SIGNATURE_LIST) Write(w io.Writer) error {
	if _, err := w.Write(l.SignatureType[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, l.SignatureListSize); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, l.SignatureHeaderSize); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, l.SignatureSize); err != nil {
		return err
	}

	if _, err := w.Write(l.SignatureHeader); err != nil {
		return err
	}

	for _, s := range l.Signatures {
		if err := s.Write(w); err != nil {
			return err
		}
	}

	return nil
}

func Read_EFI_SIGNATURE_LIST(r io.Reader) (out *EFI_SIGNATURE_LIST, err error) {
	out = &EFI_SIGNATURE_LIST{}
	if err := binary.Read(r, binary.LittleEndian, &out.SignatureType); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &out.SignatureListSize); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	if err := binary.Read(r, binary.LittleEndian, &out.SignatureHeaderSize); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	if err := binary.Read(r, binary.LittleEndian, &out.SignatureSize); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	if out.SignatureHeaderSize > math.MaxUint32-ESLHeaderSize {
		return nil, errors.New("signature header size too large")
	}
	if out.SignatureHeaderSize+ESLHeaderSize > out.SignatureListSize {
		return nil, errors.New("inconsistent size fields: total signatures payload size underflows")
	}
	signaturesSize := out.SignatureListSize - out.SignatureHeaderSize - ESLHeaderSize

	if out.SignatureSize < uint32(binary.Size(EFI_GUID{})) {
		return nil, errors.New("invalid SignatureSize")
	}
	if signaturesSize%out.SignatureSize != 0 {
		return nil, errors.New("inconsistent size fields: total signatures payload size not a multiple of the individual signature size")
	}
	numOfSignatures := int(signaturesSize / out.SignatureSize)

	out.SignatureHeader = make([]byte, out.SignatureHeaderSize)
	if _, err := io.ReadFull(r, out.SignatureHeader); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	for i := 0; i < numOfSignatures; i++ {
		var s EFI_SIGNATURE_DATA
		if _, err := io.ReadFull(r, s.SignatureOwner[:]); err != nil {
			return nil, ioerr.EOFIsUnexpected(err)
		}

		s.SignatureData = make([]byte, int(out.SignatureSize)-binary.Size(s.SignatureOwner))
		if _, err := io.ReadFull(r, s.SignatureData); err != nil {
			return nil, ioerr.EOFIsUnexpected(err)
		}

		out.Signatures = append(out.Signatures, s)
	}

	return out, nil
}
