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
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/canonical/go-efilib/internal/ioerr"
	"github.com/canonical/go-efilib/internal/uefi"
)

// VariableAuthentication corresponds to the EFI_VARIABLE_AUTHENTICATION
// type and is used to authenticate updates to variables with the
// EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS attribute set.
type VariableAuthentication struct {
	MonotonicCount uint64
	AuthInfo       WinCertificateGUID
}

// ReadVariableAuthentication decodes an authentication header for updating
// a variable with the EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS attribute set.
func ReadVariableAuthentication(r io.Reader) (*VariableAuthentication, error) {
	desc, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION(r)
	if err != nil {
		return nil, err
	}

	sig, err := newWinCertificateGUID(&desc.AuthInfo)
	if err != nil {
		return nil, err
	}

	return &VariableAuthentication{
		MonotonicCount: desc.MonotonicCount,
		AuthInfo:       sig}, nil
}

// VariableAuthentication2 corresponds to the EFI_VARIABLE_AUTHENTICATION_2
// type and is used to authenticate updates to variables with the
// EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS attribute set.
type VariableAuthentication2 struct {
	TimeStamp time.Time
	AuthInfo  WinCertificateGUID
}

// ReadTimeBasedVariableAuthentication decodes an authentication header
// for updating a variable with the EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS
// attribute set.
func ReadTimeBasedVariableAuthentication(r io.Reader) (*VariableAuthentication2, error) {
	desc, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION_2(r)
	if err != nil {
		return nil, err
	}

	sig, err := newWinCertificateGUID(&desc.AuthInfo)
	if err != nil {
		return nil, err
	}

	return &VariableAuthentication2{
		TimeStamp: desc.TimeStamp.GoTime(),
		AuthInfo:  sig}, nil
}

// VariableAuthentication3Type describes the type of [VariableAuthentication3].
type VariableAuthentication3Type int

const (
	// VariableAuthentication3TimestampType indicates that a
	// VariableAuthentication3 is a timestamp based enhanced authentication
	// and is implemented by the *VariableAuthentication3Timestamp type.
	VariableAuthentication3TimestampType VariableAuthentication3Type = uefi.EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE

	// VariableAuthentication3iNonceType indicates that a
	// VariableAuthentication3 is a nonce based enhanced authentication
	// and is implemented by the *VariableAuthentication3Nonce type.
	VariableAuthentication3NonceType VariableAuthentication3Type = uefi.EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE
)

// VariableAuthentication3 is used to authenticate updates to variables
// with the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set.
type VariableAuthentication3 interface {
	Type() VariableAuthentication3Type
	NewCert() WinCertificateGUID
	SigningCert() WinCertificateGUID
}

type variableAuthentication3 struct {
	newCert     WinCertificateGUID
	signingCert WinCertificateGUID
}

func (a *variableAuthentication3) NewCert() WinCertificateGUID {
	return a.newCert
}

func (a *variableAuthentication3) SigningCert() WinCertificateGUID {
	return a.signingCert
}

// VariableAuthentication3Timestamp is used to authenticate updates to
// variables with the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute
// set, and a type of EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE.
type VariableAuthentication3Timestamp struct {
	Timestamp time.Time
	variableAuthentication3
}

func (a *VariableAuthentication3Timestamp) Type() VariableAuthentication3Type {
	return VariableAuthentication3TimestampType
}

// VariableAuthentication3Nonce is used to authenticate updates to
// variables with the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute
// set, and a type of EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE.
type VariableAuthentication3Nonce struct {
	Nonce []byte
	variableAuthentication3
}

func (a *VariableAuthentication3Nonce) Type() VariableAuthentication3Type {
	return VariableAuthentication3NonceType
}

// ReadEnhancedVariableAuthentication decodes the authentication header for
// updating variables with the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS
// attribute set.
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

		sig, err := newWinCertificateGUID(signingCert)
		if err != nil {
			return nil, fmt.Errorf("cannot decode signature: %w", err)
		}

		out := &VariableAuthentication3Timestamp{
			Timestamp:               t.GoTime(),
			variableAuthentication3: variableAuthentication3{signingCert: sig}}
		if newCert != nil {
			sig, err := newWinCertificateGUID(newCert)
			if err != nil {
				return nil, fmt.Errorf("cannot decode new authority signature: %w", err)
			}
			out.newCert = sig
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

		sig, err := newWinCertificateGUID(signingCert)
		if err != nil {
			return nil, fmt.Errorf("cannot decode signature: %w", err)
		}

		out := &VariableAuthentication3Nonce{
			Nonce:                   n.Nonce,
			variableAuthentication3: variableAuthentication3{signingCert: sig}}
		if newCert != nil {
			sig, err := newWinCertificateGUID(newCert)
			if err != nil {
				return nil, fmt.Errorf("cannot decode new authority signature: %w", err)
			}
			out.newCert = sig
		}
		return out, nil
	default:
		return nil, errors.New("unexpected type")
	}
}

// VariableAuthentication3CertId corresponds to the EFI_VARIABLE_AUTHENTICATION_3_CERT_ID
// type and represents the identification of an authority certificate
// associated with a variable that has the EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS
// attribute set.
type VariableAuthentication3CertId interface {
	// Matches determines whether the specified certificate matches this ID
	Matches(cert *x509.Certificate) bool
}

// VariableAuthentication3Descriptor corresponds to the authentication
// descriptor provided when reading the payload of a variable with the
// EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set.
type VariableAuthentication3Descriptor interface {
	Type() VariableAuthentication3Type
	Id() VariableAuthentication3CertId // The ID of the authority associated with the variable
}

// VariableAuthentication3CertIdSHA256 corresponds to a EFI_VARIABLE_AUTHENTICATION_3_CERT_ID
// with a type of EFI_VARIABLE_AUTHENTICATION_3_CERT_ID_SHA256 and is the
// SHA-256 digest of the TBS content of a X.509 certificate.
type VariableAuthentication3CertIdSHA256 [32]byte

func (i VariableAuthentication3CertIdSHA256) Matches(cert *x509.Certificate) bool {
	h := crypto.SHA256.New()
	h.Write(cert.RawTBSCertificate)
	return bytes.Equal(h.Sum(nil), i[:])
}

func newVariableAuthentication3CertId(id *uefi.EFI_VARIABLE_AUTHENTICATION_3_CERT_ID) (VariableAuthentication3CertId, error) {
	switch id.Type {
	case uefi.EFI_VARIABLE_AUTHENTICATION_3_CERT_ID_SHA256:
		if len(id.Id) != 32 {
			return nil, errors.New("invalid SHA256 length")
		}

		var out VariableAuthentication3CertIdSHA256
		copy(out[:], id.Id)
		return out, nil
	default:
		return nil, fmt.Errorf("unrecognized type: %d", id.Type)
	}
}

// VariableAuthentication3TimestampDescriptor corresponds to the authentication
// descriptor provided when reading the payload of a variable with the
// EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set, and a type of
// EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE.
type VariableAuthentication3TimestampDescriptor struct {
	TimeStamp time.Time
	id        VariableAuthentication3CertId
}

func (d *VariableAuthentication3TimestampDescriptor) Type() VariableAuthentication3Type {
	return VariableAuthentication3TimestampType
}

func (d *VariableAuthentication3TimestampDescriptor) Id() VariableAuthentication3CertId {
	return d.id
}

// VariableAuthentication3NonceDescriptor corresponds to the authentication
// descriptor provided when reading the payload of a variable with the
// EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS attribute set, and a type of
// EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE.
type VariableAuthentication3NonceDescriptor struct {
	Nonce []byte
	id    VariableAuthentication3CertId
}

func (d *VariableAuthentication3NonceDescriptor) Type() VariableAuthentication3Type {
	return VariableAuthentication3NonceType
}

func (d *VariableAuthentication3NonceDescriptor) Id() VariableAuthentication3CertId {
	return d.id
}

// ReadEnhancedAuthenticationDescriptor decodes the enhanced authentication
// descriptor from the supplied reader. The supplied reader will typically
// read from the payload area of a variable with the
// EFI_VARIABLE_ENHANCED_AUTHENTICATION_ACCESS attribute set.
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

		id2, err := newVariableAuthentication3CertId(id)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp descriptor ID: %w", err)
		}

		return &VariableAuthentication3TimestampDescriptor{
			TimeStamp: t.GoTime(),
			id:        id2}, nil
	case uefi.EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE:
		n, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION_3_NONCE(r)
		if err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read nonce descriptor: %w", err)
		}

		id, err := uefi.Read_EFI_VARIABLE_AUTHENTICATION_3_CERT_ID(r)
		if err != nil {
			return nil, ioerr.EOFIsUnexpected("cannot read nonce descriptor: %w", err)
		}

		id2, err := newVariableAuthentication3CertId(id)
		if err != nil {
			return nil, fmt.Errorf("invalid nonce descriptor ID: %w", err)
		}

		return &VariableAuthentication3NonceDescriptor{
			Nonce: n.Nonce,
			id:    id2}, nil
	default:
		return nil, errors.New("unexpected type")
	}
}
