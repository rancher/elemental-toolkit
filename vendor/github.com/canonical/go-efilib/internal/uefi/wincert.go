// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/canonical/go-efilib/internal/ioerr"
)

const (
	WIN_CERT_TYPE_PKCS_SIGNED_DATA = 0x0002
	WIN_CERT_TYPE_EFI_PKCS115      = 0x0ef0
	WIN_CERT_TYPE_EFI_GUID         = 0x0ef1
)

type WIN_CERTIFICATE struct {
	Length          uint32
	Revision        uint16
	CertificateType uint16
}

type WIN_CERTIFICATE_EFI_PKCS1_15 struct {
	Hdr           WIN_CERTIFICATE
	HashAlgorithm EFI_GUID
	Signature     []byte
}

type WIN_CERTIFICATE_UEFI_GUID struct {
	Hdr      WIN_CERTIFICATE
	CertType EFI_GUID
	CertData []byte
}

func Read_WIN_CERTIFICATE_UEFI_GUID(r io.Reader) (out *WIN_CERTIFICATE_UEFI_GUID, err error) {
	out = &WIN_CERTIFICATE_UEFI_GUID{}
	if err := binary.Read(r, binary.LittleEndian, &out.Hdr); err != nil {
		return nil, err
	}
	if out.Hdr.Revision != 0x0200 {
		return nil, errors.New("unexpected Hdr.Revision")
	}
	if out.Hdr.CertificateType != WIN_CERT_TYPE_EFI_GUID {
		return nil, errors.New("unexpected Hdr.CertificateType")
	}

	if _, err := io.ReadFull(r, out.CertType[:]); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	out.CertData = make([]byte, int(out.Hdr.Length)-binary.Size(out.Hdr)-binary.Size(out.CertType))
	if _, err := io.ReadFull(r, out.CertData); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return out, nil
}

type WIN_CERTIFICATE_EFI_PKCS struct {
	Hdr      WIN_CERTIFICATE
	CertData []byte
}
