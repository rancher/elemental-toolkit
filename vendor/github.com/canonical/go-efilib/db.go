// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"crypto"
	_ "crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/canonical/go-efilib/internal/uefi"
)

// SignatureData corresponds to the EFI_SIGNATURE_DATA type.
type SignatureData struct {
	Owner GUID
	Data  []byte
}

func (d *SignatureData) toUefiType() *uefi.EFI_SIGNATURE_DATA {
	return &uefi.EFI_SIGNATURE_DATA{
		SignatureOwner: uefi.EFI_GUID(d.Owner),
		SignatureData:  d.Data}
}

// Write serializes this signature data to w.
func (d *SignatureData) Write(w io.Writer) error {
	return d.toUefiType().Write(w)
}

// Equal determines whether other is equal to this SignatureData
func (d *SignatureData) Equal(other *SignatureData) bool {
	if d.Owner != other.Owner {
		return false
	}
	return bytes.Equal(d.Data, other.Data)
}

// SignatureList corresponds to the EFI_SIGNATURE_LIST type.
type SignatureList struct {
	Type       GUID
	Header     []byte
	Signatures []*SignatureData
}

func (l *SignatureList) toUefiType() (out *uefi.EFI_SIGNATURE_LIST, err error) {
	out = &uefi.EFI_SIGNATURE_LIST{
		SignatureType:       uefi.EFI_GUID(l.Type),
		SignatureHeaderSize: uint32(len(l.Header)),
		SignatureHeader:     l.Header}

	for i, s := range l.Signatures {
		sig := s.toUefiType()

		sz := uint32(binary.Size(sig.SignatureOwner) + len(sig.SignatureData))
		if i == 0 {
			out.SignatureSize = sz
		}
		if sz != out.SignatureSize {
			// EFI_SIGNATURE_LIST cannot contain EFI_SIGNATURE_DATA entries with different
			// sizes - they must go in their own list.
			return nil, fmt.Errorf("signature %d contains the wrong size", i)
		}

		out.Signatures = append(out.Signatures, *sig)
	}

	out.SignatureListSize = uefi.ESLHeaderSize + out.SignatureHeaderSize + (out.SignatureSize * uint32(len(out.Signatures)))
	return out, nil
}

func (l *SignatureList) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, `EFI_SIGNATURE_LIST {
	SignatureType: %v,
	SignatureHeader: %x,
	Signatures: [`, l.Type, l.Header)
	for _, d := range l.Signatures {
		fmt.Fprintf(&b, `
		EFI_SIGNATURE_DATA {
			SignatureOwner: %v,
			Details: {`, d.Owner)
		switch l.Type {
		case CertSHA1Guid, CertSHA256Guid, CertSHA224Guid, CertSHA384Guid, CertSHA512Guid:
			fmt.Fprintf(&b, " Hash: %x },", d.Data)
		case CertX509Guid:
			cert, err := x509.ParseCertificate(d.Data)
			switch {
			case err != nil:
				fmt.Fprintf(&b, " Cannot parse certificate: %v },", err)
			default:
				h := crypto.SHA256.New()
				h.Write(cert.Raw)
				fmt.Fprintf(&b, `
				Subject: %v
				Issuer: %v
				SHA256 fingerprint: %x
			},`, cert.Subject, cert.Issuer, h.Sum(nil))
			}
		default:
			b.WriteString(" <unrecognized type> ")
		}
		b.WriteString("\n\t\t},")
	}
	b.WriteString("\n\t],\n}")

	return b.String()
}

// Write serializes this signature list to w.
func (l *SignatureList) Write(w io.Writer) error {
	list, err := l.toUefiType()
	if err != nil {
		return err
	}
	return list.Write(w)
}

// ReadSignatureList decodes a single EFI_SIGNATURE_LIST from r.
func ReadSignatureList(r io.Reader) (*SignatureList, error) {
	l, err := uefi.Read_EFI_SIGNATURE_LIST(r)
	if err != nil {
		return nil, err
	}

	list := &SignatureList{Type: GUID(l.SignatureType), Header: l.SignatureHeader}

	for _, s := range l.Signatures {
		list.Signatures = append(list.Signatures, &SignatureData{Owner: GUID(s.SignatureOwner), Data: s.SignatureData})
	}

	return list, nil
}

// SignatureDatabase corresponds to a list of EFI_SIGNATURE_LIST structures.
type SignatureDatabase []*SignatureList

func (db SignatureDatabase) String() string {
	var b strings.Builder
	b.WriteString("{")
	for _, l := range db {
		fmt.Fprintf(&b, "\n\t%s,", indent(l, 1))
	}
	b.WriteString("\n}")
	return b.String()
}

// Bytes returns the serialized form of this signature database.
func (db SignatureDatabase) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := db.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write serializes this signature database to w.
func (db SignatureDatabase) Write(w io.Writer) error {
	for i, l := range db {
		if err := l.Write(w); err != nil {
			return fmt.Errorf("cannot encode signature list %d: %w", i, err)
		}
	}
	return nil
}

// ReadSignatureDatabase decodes a list of EFI_SIGNATURE_LIST structures from r.
func ReadSignatureDatabase(r io.Reader) (SignatureDatabase, error) {
	var db SignatureDatabase
	for i := 0; ; i++ {
		l, err := ReadSignatureList(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("cannot read EFI_SIGNATURE_LIST %d: %w", i, err)
		}
		db = append(db, l)
	}

	return db, nil
}
