// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/canonical/go-efilib/internal/ioerr"
	"github.com/canonical/go-efilib/internal/uefi"
)

// WinCertificate is an interface type corresponding to implementations of WIN_CERTIFICATE.
type WinCertificate interface {
	Write(w io.Writer) error // Encode this certificate to the supplied io.Writer
}

// WinCertificatePKCS1v15 corresponds to the WIN_CERTIFICATE_EFI_PKCS1_15 type.
type WinCertificatePKCS1v15 struct {
	HashAlgorithm GUID
	Signature     []byte
}

func (c *WinCertificatePKCS1v15) Write(w io.Writer) error {
	cert := uefi.WIN_CERTIFICATE_EFI_PKCS1_15{
		HashAlgorithm: uefi.EFI_GUID(c.HashAlgorithm),
		Signature:     c.Signature}
	cert.Hdr = uefi.WIN_CERTIFICATE{
		Length:          uint32(binary.Size(cert.Hdr) + binary.Size(cert.HashAlgorithm) + len(c.Signature)),
		Revision:        0x0200,
		CertificateType: uefi.WIN_CERT_TYPE_EFI_PKCS115}
	return binary.Write(w, binary.LittleEndian, &cert)
}

// WinCertificateGUID corresponds to the WIN_CERTIFICATE_UEFI_GUID type.
type WinCertificateGUID struct {
	Type GUID
	Data []byte
}

func (c *WinCertificateGUID) Write(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, c.toUefiType())
}

func (c *WinCertificateGUID) toUefiType() *uefi.WIN_CERTIFICATE_UEFI_GUID {
	cert := &uefi.WIN_CERTIFICATE_UEFI_GUID{
		CertType: uefi.EFI_GUID(c.Type),
		CertData: c.Data}
	cert.Hdr = uefi.WIN_CERTIFICATE{
		Length:          uint32(binary.Size(cert.Hdr) + binary.Size(cert.CertType) + len(c.Data)),
		Revision:        0x0200,
		CertificateType: uefi.WIN_CERT_TYPE_EFI_GUID}
	return cert
}

func newWinCertificateGUID(cert *uefi.WIN_CERTIFICATE_UEFI_GUID) *WinCertificateGUID {
	return &WinCertificateGUID{Type: GUID(cert.CertType), Data: cert.CertData}
}

// WinCertificateAuthenticode corresponds to an Authenticode signature.
type WinCertificateAuthenticode []byte

func (c WinCertificateAuthenticode) Write(w io.Writer) error {
	cert := uefi.WIN_CERTIFICATE_EFI_PKCS{CertData: c}
	cert.Hdr = uefi.WIN_CERTIFICATE{
		Length:          uint32(binary.Size(cert.Hdr) + len(c)),
		Revision:        0x0200,
		CertificateType: uefi.WIN_CERT_TYPE_PKCS_SIGNED_DATA}
	return binary.Write(w, binary.LittleEndian, &cert)
}

// ReadWinCertificate decodes a signature (something that is confusingly represented by types with "certificate" in the name in both
// the UEFI and PE/COFF specifications) from the supplied io.Reader and returns a WinCertificate of the appropriate type. The type
// returned is dependent on the data, and will be one of *WinCertificateAuthenticode, *WinCertificatePKCS1_15 or *WinCertificateGUID.
func ReadWinCertificate(r io.Reader) (WinCertificate, error) {
	var hdr uefi.WIN_CERTIFICATE
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, err
	}
	if hdr.Revision != 0x0200 {
		return nil, errors.New("unexpected revision")
	}

	switch hdr.CertificateType {
	case uefi.WIN_CERT_TYPE_PKCS_SIGNED_DATA:
		cert := uefi.WIN_CERTIFICATE_EFI_PKCS{Hdr: hdr}
		cert.CertData = make([]byte, int(cert.Hdr.Length)-binary.Size(cert.Hdr))
		if _, err := io.ReadFull(r, cert.CertData); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS: %w", err)
		}
		return WinCertificateAuthenticode(cert.CertData), nil
	case uefi.WIN_CERT_TYPE_EFI_PKCS115:
		cert := uefi.WIN_CERTIFICATE_EFI_PKCS1_15{Hdr: hdr}
		cert.Signature = make([]byte, int(cert.Hdr.Length)-binary.Size(cert.Hdr)-binary.Size(cert.HashAlgorithm))
		if _, err := io.ReadFull(r, cert.HashAlgorithm[:]); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS1_15: %w", err)
		}
		if _, err := io.ReadFull(r, cert.Signature); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_EFI_PKCS1_15: %w", err)
		}
		return &WinCertificatePKCS1v15{HashAlgorithm: GUID(cert.HashAlgorithm), Signature: cert.Signature}, nil
	case uefi.WIN_CERT_TYPE_EFI_GUID:
		cert := uefi.WIN_CERTIFICATE_UEFI_GUID{Hdr: hdr}
		cert.CertData = make([]byte, int(cert.Hdr.Length)-binary.Size(cert.Hdr)-binary.Size(cert.CertType))
		if _, err := io.ReadFull(r, cert.CertType[:]); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_UEFI_GUID: %w", err)
		}
		if _, err := io.ReadFull(r, cert.CertData); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read WIN_CERTIFICATE_UEFI_GUID: %w", err)
		}
		return newWinCertificateGUID(&cert), nil
	default:
		return nil, errors.New("unexpected type")
	}
}
