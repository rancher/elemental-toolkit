// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

import (
	"encoding/binary"
	"io"

	"github.com/canonical/go-efilib/internal/ioerr"
)

const (
	EFI_VARIABLE_AUTHENTICATION_3_CERT_ID_SHA256 = 1

	EFI_VARIABLE_AUTHENTICATION_3_TIMESTAMP_TYPE = 1
	EFI_VARIABLE_AUTHENTICATION_3_NONCE_TYPE     = 2
)

type EFI_VARIABLE_AUTHENTICATION struct {
	MonotonicCount uint64
	AuthInfo       WIN_CERTIFICATE_UEFI_GUID
}

func Read_EFI_VARIABLE_AUTHENTICATION(r io.Reader) (out *EFI_VARIABLE_AUTHENTICATION, err error) {
	out = &EFI_VARIABLE_AUTHENTICATION{}
	if err := binary.Read(r, binary.LittleEndian, &out.MonotonicCount); err != nil {
		return nil, err
	}
	cert, err := Read_WIN_CERTIFICATE_UEFI_GUID(r)
	if err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	out.AuthInfo = *cert
	return out, nil
}

type EFI_VARIABLE_AUTHENTICATION_2 struct {
	TimeStamp EFI_TIME
	AuthInfo  WIN_CERTIFICATE_UEFI_GUID
}

func Read_EFI_VARIABLE_AUTHENTICATION_2(r io.Reader) (out *EFI_VARIABLE_AUTHENTICATION_2, err error) {
	out = &EFI_VARIABLE_AUTHENTICATION_2{}
	if err := binary.Read(r, binary.LittleEndian, &out.TimeStamp); err != nil {
		return nil, err
	}
	cert, err := Read_WIN_CERTIFICATE_UEFI_GUID(r)
	if err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	out.AuthInfo = *cert
	return out, nil
}

type EFI_VARIABLE_AUTHENTICATION_3 struct {
	Version      uint8
	Type         uint8
	MetadataSize uint32
	Flags        uint32
}

type EFI_VARIABLE_AUTHENTICATION_3_CERT_ID struct {
	Type   uint8
	IdSize uint32
	Id     []byte
}

func Read_EFI_VARIABLE_AUTHENTICATION_3_CERT_ID(r io.Reader) (out *EFI_VARIABLE_AUTHENTICATION_3_CERT_ID, err error) {
	out = &EFI_VARIABLE_AUTHENTICATION_3_CERT_ID{}
	if err := binary.Read(r, binary.LittleEndian, &out.Type); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &out.IdSize); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	out.Id = make([]byte, out.IdSize)
	if _, err := io.ReadFull(r, out.Id); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return out, nil
}

type EFI_VARIABLE_AUTHENTICATION_3_NONCE struct {
	NonceSize uint32
	Nonce     []byte
}

func Read_EFI_VARIABLE_AUTHENTICATION_3_NONCE(r io.Reader) (out *EFI_VARIABLE_AUTHENTICATION_3_NONCE, err error) {
	out = &EFI_VARIABLE_AUTHENTICATION_3_NONCE{}
	if err := binary.Read(r, binary.LittleEndian, &out.NonceSize); err != nil {
		return nil, err
	}

	out.Nonce = make([]byte, out.NonceSize)
	if _, err := io.ReadFull(r, out.Nonce); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return out, nil
}
