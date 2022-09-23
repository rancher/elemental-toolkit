// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2021 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package tpm2

import (
	"bytes"
	"crypto"
	"encoding/json"
	"errors"
	"io/ioutil"

	"golang.org/x/xerrors"

	"github.com/snapcore/secboot"
)

const legacyPlatformName = "tpm2-legacy"

type legacyPlatformKeyDataHandler struct{}

func (h *legacyPlatformKeyDataHandler) RecoverKeys(data *secboot.PlatformKeyData) (secboot.KeyPayload, error) {
	tpm, err := ConnectToTPM()
	switch {
	case err == ErrNoTPM2Device:
		return nil, &secboot.PlatformKeyRecoveryError{
			Type: secboot.PlatformKeyRecoveryErrorUnavailable,
			Err:  err}
	case err != nil:
		return nil, xerrors.Errorf("cannot connect to TPM: %w", err)
	}
	defer tpm.Close()

	var handle []byte
	if err := json.Unmarshal(data.Handle, &handle); err != nil {
		return nil, &secboot.PlatformKeyRecoveryError{
			Type: secboot.PlatformKeyRecoveryErrorInvalidData,
			Err:  err}
	}

	k, err := ReadSealedKeyObject(bytes.NewReader(handle))
	if err != nil {
		var e InvalidKeyDataError
		if xerrors.As(err, &e) {
			return nil, &secboot.PlatformKeyRecoveryError{
				Type: secboot.PlatformKeyRecoveryErrorInvalidData,
				Err:  err}
		}
		return nil, xerrors.Errorf("cannot read key object: %w", err)
	}

	key, authKey, err := k.UnsealFromTPM(tpm)
	if err != nil {
		var e InvalidKeyDataError
		switch {
		case xerrors.As(err, &e):
			return nil, &secboot.PlatformKeyRecoveryError{
				Type: secboot.PlatformKeyRecoveryErrorInvalidData,
				Err:  errors.New(e.msg)}
		case err == ErrTPMProvisioning:
			return nil, &secboot.PlatformKeyRecoveryError{
				Type: secboot.PlatformKeyRecoveryErrorUninitialized,
				Err:  err}
		case err == ErrTPMLockout:
			return nil, &secboot.PlatformKeyRecoveryError{
				Type: secboot.PlatformKeyRecoveryErrorUnavailable,
				Err:  err}
		}
		return nil, xerrors.Errorf("cannot unseal key: %w", err)
	}

	return secboot.MarshalKeys(key, secboot.AuxiliaryKey(authKey)), nil
}

// NewKeyDataFromSealedKeyObjectFile creates a secboot.KeyData for the TPM
// sealed key object at the supplied path, in order to enable keys to be
// recovered from the TPM sealed key object using the secboot.KeyData API.
//
// Note that the returned KeyData does not support the snap model authorization
// API, and consumers of this function should not attempt to use this API.
//
// If the file cannot be opened, an *os.PathError error will be returned.
//
// This function decodes enough metadata to construct the KeyData object If
// this fails, an InvalidKeyDataError error will be returned.
func NewKeyDataFromSealedKeyObjectFile(path string) (*secboot.KeyData, error) {
	r, err := NewFileSealedKeyObjectReader(path)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	handle, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	creationData := secboot.KeyCreationData{
		PlatformKeyData: secboot.PlatformKeyData{
			Handle: handle,
		},
		PlatformName:      legacyPlatformName,
		AuxiliaryKey:      make([]byte, 32), // Not used, but must be the expected size
		SnapModelAuthHash: crypto.SHA256,    // Not used, but just set it a valid alg
	}

	return secboot.NewKeyData(&creationData)
}

func init() {
	secboot.RegisterPlatformKeyDataHandler(legacyPlatformName, &legacyPlatformKeyDataHandler{})
}
