// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

import (
	"encoding/binary"
	"io"
	"io/ioutil"

	"github.com/canonical/go-efilib/internal/ioerr"
)

const (
	LOAD_OPTION_ACTIVE          = 0x00000001
	LOAD_OPTION_FORCE_RECONNECT = 0x00000002
	LOAD_OPTION_HIDDEN          = 0x00000008
	LOAD_OPTION_CATEGORY        = 0x00001f00
	LOAD_OPTION_CATEGORY_BOOT   = 0x00000000
	LOAD_OPTION_CATEGORY_APP    = 0x00000100
)

type EFI_LOAD_OPTION struct {
	Attributes         uint32
	FilePathListLength uint16
	Description        []uint16
	FilePathList       []byte
	OptionalData       []byte
}

func (o *EFI_LOAD_OPTION) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, o.Attributes); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, o.FilePathListLength); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, o.Description); err != nil {
		return err
	}
	if _, err := w.Write(o.FilePathList); err != nil {
		return err
	}
	if _, err := w.Write(o.OptionalData); err != nil {
		return err
	}
	return nil
}

func Read_EFI_LOAD_OPTION(r io.Reader) (out *EFI_LOAD_OPTION, err error) {
	out = &EFI_LOAD_OPTION{}
	if err := binary.Read(r, binary.LittleEndian, &out.Attributes); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &out.FilePathListLength); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	for i := 0; ; i++ {
		var c uint16
		if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
			return nil, ioerr.EOFIsUnexpected(err)
		}
		out.Description = append(out.Description, c)
		if c == 0 {
			break
		}
	}

	out.FilePathList = make([]byte, out.FilePathListLength)
	if _, err := io.ReadFull(r, out.FilePathList); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	optionalData, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	out.OptionalData = optionalData

	return out, nil
}
