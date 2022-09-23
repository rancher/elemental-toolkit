// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/canonical/go-efilib/internal/ioerr"
	"github.com/canonical/go-efilib/internal/uefi"
)

// VariableAuthentication correspond to the EFI_VARIABLE_AUTHENTICATION type and is provided as a header when updating a variable with
// the EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS attribute set.
type VariableAuthentication struct {
	MonotonicCount uint64
	AuthInfo       WinCertificateGUID
}

func (a *VariableAuthentication) Write(w io.Writer) error {
	desc := uefi.EFI_VARIABLE_AUTHENTICATION{
		MonotonicCount: a.MonotonicCount,
		AuthInfo:       *a.AuthInfo.toUefiType()}
	return binary.Write(w, binary.LittleEndian, desc)
}

// ReadVariableAuthentication decodes a header for updating a variable with the EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS attribute
// set.
func ReadVariableAuthentication(r io.Reader) (*VariableAuthentication, error) {
	desc, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION(r)
	if err != nil {
		return nil, err
	}

	return &VariableAuthentication{
		MonotonicCount: desc.MonotonicCount,
		AuthInfo:       *newWinCertificateGUID(&desc.AuthInfo)}, nil
}

// VariableAuthentication2 correspond to the EFI_VARIABLE_AUTHENTICATION_2 type and is provided as a header when updating a variable
// with the EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS attribute set.
type VariableAuthentication2 struct {
	TimeStamp time.Time
	AuthInfo  WinCertificateGUID
}

func (a *VariableAuthentication2) Write(w io.Writer) error {
	desc := uefi.EFI_VARIABLE_AUTHENTICATION_2{
		TimeStamp: *uefi.New_EFI_TIME(a.TimeStamp),
		AuthInfo:  *a.AuthInfo.toUefiType()}
	return binary.Write(w, binary.LittleEndian, desc)
}

// ReadTimeBasedVariableAuthentication decodes the header for updating a variable with the
// EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS attribute set.
func ReadTimeBasedVariableAuthentication(r io.Reader) (*VariableAuthentication2, error) {
	desc, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION_2(r)
	if err != nil {
		return nil, err
	}

	return &VariableAuthentication2{
		TimeStamp: desc.TimeStamp.GoTime(),
		AuthInfo:  *newWinCertificateGUID(&desc.AuthInfo)}, nil
}

// VariableAuthentication3 represents the header for updating a variable with the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS
// attribute set.
type VariableAuthentication3 interface{}

// VariableAuthentication3Timestamp corresponds to the header for updating a variable with the
// EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set, and a type of EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE.
type VariableAuthentication3Timestamp struct {
	TimeStamp   time.Time
	NewCert     *WinCertificateGUID
	SigningCert WinCertificateGUID
}

func (a *VariableAuthentication3Timestamp) Write(w io.Writer) error {
	var buf bytes.Buffer

	t := uefi.New_EFI_TIME(a.TimeStamp)
	if err := binary.Write(&buf, binary.LittleEndian, &t); err != nil {
		panic(err)
	}

	if a.NewCert != nil {
		if err := binary.Write(&buf, binary.LittleEndian, a.NewCert.toUefiType()); err != nil {
			panic(err)
		}
	}

	if err := binary.Write(&buf, binary.LittleEndian, a.SigningCert.toUefiType()); err != nil {
		panic(err)
	}

	hdr := uefi.EFI_VARIABLE_AUTHENTICATION_3{
		Version: 1,
		Type:    uefi.EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE}
	hdr.MetadataSize = uint32(binary.Size(hdr) + buf.Len())
	if a.NewCert != nil {
		hdr.Flags = 1
	}
	if err := binary.Write(w, binary.LittleEndian, &hdr); err != nil {
		return err
	}

	_, err := buf.WriteTo(w)
	return err
}

// VariableAuthentication3Nonce corresponds to the header for updating a variable with the
// EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set, and a type of EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE.
type VariableAuthentication3Nonce struct {
	Nonce       []byte
	NewCert     *WinCertificateGUID
	SigningCert WinCertificateGUID
}

func (a *VariableAuthentication3Nonce) Write(w io.Writer) error {
	var buf bytes.Buffer

	n := uefi.EFI_VARIABLE_AUTHENTICATION_3_NONCE{
		NonceSize: uint32(len(a.Nonce)),
		Nonce:     a.Nonce}
	if err := binary.Write(&buf, binary.LittleEndian, &n); err != nil {
		panic(err)
	}

	if a.NewCert != nil {
		if err := binary.Write(&buf, binary.LittleEndian, a.NewCert.toUefiType()); err != nil {
			panic(err)
		}
	}

	if err := binary.Write(&buf, binary.LittleEndian, a.SigningCert.toUefiType()); err != nil {
		panic(err)
	}

	hdr := uefi.EFI_VARIABLE_AUTHENTICATION_3{
		Version: 1,
		Type:    uefi.EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE}
	hdr.MetadataSize = uint32(binary.Size(hdr) + buf.Len())
	if a.NewCert != nil {
		hdr.Flags = 1
	}
	if err := binary.Write(w, binary.LittleEndian, &hdr); err != nil {
		return err
	}

	_, err := buf.WriteTo(w)
	return err
}

// ReadEnhancedVariableAuthentication decodes the header for updating a variable with the
// EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set.
func ReadEnhancedVariableAuthentication(r io.Reader) (VariableAuthentication3, error) {
	var hdr uefi.EFI_VARIABLE_AUTHENTICATION_3
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, err
	}
	if hdr.Version != 1 {
		return nil, errors.New("unexpected version")
	}

	lr := io.LimitReader(r, int64(hdr.MetadataSize)-int64(binary.Size(hdr)))

	switch hdr.Type {
	case uefi.EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE:
		var t uefi.EFI_TIME
		if err := binary.Read(lr, binary.LittleEndian, &t); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read timestamp authentication: %w", err)
		}

		var newCert *uefi.WIN_CERTIFICATE_UEFI_GUID
		if hdr.Flags&1 > 0 {
			cert, err := uefi.Read_WIN_CERTIFICATE_UEFI_GUID(r)
			if err != nil {
				return nil, ioerr.EOFIsUnexpected("cannot read timestamp authentication: %w", err)
			}
			newCert = cert
		}

		signingCert, err := uefi.Read_WIN_CERTIFICATE_UEFI_GUID(r)
		if err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read timestamp authentication: %w", err)
		}

		out := &VariableAuthentication3Timestamp{
			TimeStamp:   t.GoTime(),
			SigningCert: *newWinCertificateGUID(signingCert)}
		if newCert != nil {
			out.NewCert = newWinCertificateGUID(newCert)
		}
		return out, nil
	case uefi.EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE:
		n, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION_3_NONCE(r)
		if err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read nonce authentication: %w", err)
		}

		var newCert *uefi.WIN_CERTIFICATE_UEFI_GUID
		if hdr.Flags&1 > 0 {
			cert, err := uefi.Read_WIN_CERTIFICATE_UEFI_GUID(r)
			if err != nil {
				return nil, ioerr.EOFIsUnexpected("cannot read nonce authentication: %w", err)
			}
			newCert = cert
		}

		signingCert, err := uefi.Read_WIN_CERTIFICATE_UEFI_GUID(r)
		if err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read nonce authentication: %w", err)
		}

		out := &VariableAuthentication3Nonce{
			Nonce:       n.Nonce,
			SigningCert: *newWinCertificateGUID(signingCert)}
		if newCert != nil {
			out.NewCert = newWinCertificateGUID(newCert)
		}
		return out, nil
	default:
		return nil, errors.New("unexpected type")
	}
}

// VariableAuthentication3Descriptor corresponds to the authentication descriptor provided when reading the payload of a variable
// with the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set.
type VariableAuthentication3Descriptor interface{}

const (
	VariableAuthentication3CertIDSHA256 = uefi.EFI_VARIABLE_AUTHENTICATION_3_CERT_ID_SHA256
)

type VariableAuthentication3CertId struct {
	Type uint8
	Id   []byte
}

func newVariableAuthentication3CertId(id *uefi.EFI_VARIABLE_AUTHENTICATION_3_CERT_ID) *VariableAuthentication3CertId {
	return &VariableAuthentication3CertId{
		Type: id.Type,
		Id:   id.Id}
}

// VariableAuthentication3TimestampDescriptor corresponds to the authentication descriptor provided when reading the payload of a
// variable with the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set, and a type of
// EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE.
type VariableAuthentication3TimestampDescriptor struct {
	TimeStamp time.Time
	VariableAuthentication3CertId
}

// VariableAuthentication3NonceDescriptor corresponds to the authentication descriptor provided when reading the payload of a
// variable with the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set, and a type of
// EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE.
type VariableAuthentication3NonceDescriptor struct {
	Nonce []byte
	VariableAuthentication3CertId
}

// ReadEnhancedAuthenticationDescriptor decodes the enhanced authentication descriptor from the supplied io.Reader. The supplied
// reader will typically read from the payload area of a variable with the EFI_VARIABLE_ENHANCED_AUTHENTICATION_ACCESS attribute
// set.
func ReadEnhancedAuthenticationDescriptor(r io.Reader) (VariableAuthentication3Descriptor, error) {
	var hdr uefi.EFI_VARIABLE_AUTHENTICATION_3
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, err
	}
	if hdr.Version != 1 {
		return nil, errors.New("unexpected version")
	}

	lr := io.LimitReader(r, int64(hdr.MetadataSize)-int64(binary.Size(hdr)))

	switch hdr.Type {
	case uefi.EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE:
		var t uefi.EFI_TIME
		if err := binary.Read(lr, binary.LittleEndian, &t); err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read timestamp descriptor: %w", err)
		}

		id, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION_3_CERT_ID(r)
		if err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read timestamp descriptor: %w", err)
		}

		return &VariableAuthentication3TimestampDescriptor{
			TimeStamp:                     t.GoTime(),
			VariableAuthentication3CertId: *newVariableAuthentication3CertId(id)}, nil
	case uefi.EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE:
		n, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION_3_NONCE(r)
		if err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read nonce descriptor: %w", err)
		}

		id, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION_3_CERT_ID(r)
		if err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read nonce descriptor: %w", err)
		}

		return &VariableAuthentication3NonceDescriptor{
			Nonce:                         n.Nonce,
			VariableAuthentication3CertId: *newVariableAuthentication3CertId(id)}, nil
	default:
		return nil, errors.New("unexpected type")
	}
}
