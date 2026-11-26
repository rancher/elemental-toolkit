// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

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

func checkWIN_CERTIFICATE(cert *WIN_CERTIFICATE, expectedType uint16) error {
	if cert.Length < uint32(binary.Size(*cert)) {
		return fmt.Errorf("invalid WIN_CERTIFICATE.Length (%d) (too small)", cert.Length)
	}
	// Make sure the remaining bytes after hdr.Length won't overflow
	// an int on 32-bit platforms. It never can on 64-bit platforms
	// because int is int64 and we are dealing with uint32. On 32-bit
	// platforms, int is int32 and therefore the uint32 length can
	// overflow an int.
	if cert.Length-uint32(binary.Size(*cert)) > math.MaxInt32 {
		return fmt.Errorf("invalid WIN_CERTIFICATE.Length (%d) (too large)", cert.Length)
	}
	if cert.Revision != 0x0200 {
		return fmt.Errorf("unexpected WIN_CERTIFICATE.Revision (%#x)", cert.Revision)
	}
	if cert.CertificateType != expectedType {
		return fmt.Errorf("unexpected WIN_CERTIFICATE.CertificateType (%#x)", cert.CertificateType)
	}
	return nil
}

type WIN_CERTIFICATE_EFI_PKCS1_15 struct {
	Hdr           WIN_CERTIFICATE
	HashAlgorithm EFI_GUID
	Signature     [256]byte
}

func Read_WIN_CERTIFICATE_EFI_PKCS1_15(r io.Reader) (out *WIN_CERTIFICATE_EFI_PKCS1_15, err error) {
	out = new(WIN_CERTIFICATE_EFI_PKCS1_15)
	if err := binary.Read(r, binary.LittleEndian, &out.Hdr); err != nil {
		return nil, ioerr.PassRawEOF("cannot read WIN_CERTIFICATE_EFI_PKCS1_15.Hdr: %w", err)
	}
	if err := checkWIN_CERTIFICATE(&out.Hdr, WIN_CERT_TYPE_EFI_PKCS115); err != nil {
		return nil, fmt.Errorf("cannot check WIN_CERTIFICATE_EFI_PKCS1_15.Hdr: %w", err)
	}

	lr := &io.LimitedReader{
		R: r,
		N: int64(out.Hdr.Length) - int64(binary.Size(out.Hdr)),
	}
	if _, err := io.ReadFull(lr, out.HashAlgorithm[:]); err != nil {
		return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS1_15.HashAlgorithm: %w", err)
	}
	if _, err := io.ReadFull(lr, out.Signature[:]); err != nil {
		return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS1_15.Signature: %w", err)
	}
	if lr.N > 0 {
		return nil, fmt.Errorf("invalid WIN_CERTIFICATE_PKCS1_15.Hdr.Length (%d) (too large)", out.Hdr.Length)
	}

	return out, nil
}

type WIN_CERTIFICATE_UEFI_GUID struct {
	Hdr      WIN_CERTIFICATE
	CertType EFI_GUID
	CertData []byte
}

func Read_WIN_CERTIFICATE_UEFI_GUID(r io.Reader) (out *WIN_CERTIFICATE_UEFI_GUID, err error) {
	out = new(WIN_CERTIFICATE_UEFI_GUID)
	if err := binary.Read(r, binary.LittleEndian, &out.Hdr); err != nil {
		return nil, ioerr.PassRawEOF("cannot read WIN_CERTIFICATE_UEFI_GUID.Hdr: %w", err)
	}
	if err := checkWIN_CERTIFICATE(&out.Hdr, WIN_CERT_TYPE_EFI_GUID); err != nil {
		return nil, fmt.Errorf("cannot check WIN_CERTIFICATE_UEFI_GUID.Hdr: %w", err)
	}

	lr := &io.LimitedReader{
		R: r,
		N: int64(out.Hdr.Length) - int64(binary.Size(out.Hdr)),
	}
	if _, err := io.ReadFull(lr, out.CertType[:]); err != nil {
		return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_UEFI_GUID.CertType: %w", err)
	}
	out.CertData = make([]byte, int(lr.N)) // The remaining bytes are the CertData
	if _, err := io.ReadFull(lr, out.CertData); err != nil {
		return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_UEFI_GUID.CertData: %w", err)
	}

	return out, nil
}

type WIN_CERTIFICATE_EFI_PKCS struct {
	Hdr      WIN_CERTIFICATE
	CertData []byte
}

func Read_WIN_CERTIFICATE_EFI_PKCS(r io.Reader) (*WIN_CERTIFICATE_EFI_PKCS, error) {
	out := new(WIN_CERTIFICATE_EFI_PKCS)
	if err := binary.Read(r, binary.LittleEndian, &out.Hdr); err != nil {
		return nil, ioerr.PassRawEOF("cannot read WIN_CERTIFICATE_EFI_PKCS.Hdr: %w", err)
	}
	if err := checkWIN_CERTIFICATE(&out.Hdr, WIN_CERT_TYPE_PKCS_SIGNED_DATA); err != nil {
		return nil, fmt.Errorf("cannot check WIN_CERTIFICATE_UEFI_GUID.Hdr: %w", err)
	}

	lr := &io.LimitedReader{
		R: r,
		N: int64(out.Hdr.Length) - int64(binary.Size(out.Hdr)),
	}
	out.CertData = make([]byte, int(lr.N))
	if _, err := io.ReadFull(lr, out.CertData); err != nil {
		return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS.CertData: %w", err)
	}

	return out, nil
}
