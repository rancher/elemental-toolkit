// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
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
	"fmt"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"

	"golang.org/x/xerrors"

	"github.com/snapcore/secboot/internal/tcg"
)

const (
	tryPersistentSRK = iota
	tryTransientSRK
	tryMax
)

// loadForUnseal loads the sealed key object into the TPM and returns a context
// for it. It first tries by using the persistent shared SRK at the well known
// handle as the parent object. If this object doesn't exist or loading fails with
// an error indicating that the supplied sealed key object data is invalid, this
// function will try to create a transient SRK and then retry loading of the sealed
// key object by specifying the newly created transient object as the parent.
//
// If both attempts to load the sealed key object fail, or if the first attempt fails
// and a transient SRK cannot be created, an error will be returned.
//
// If a transient SRK is created, it is flushed from the TPM before this function
// returns.
func (k *SealedKeyObject) loadForUnseal(tpm *tpm2.TPMContext, session tpm2.SessionContext) (tpm2.ResourceContext, error) {
	var lastError error
	for try := tryPersistentSRK; try <= tryMax; try++ {
		var srk tpm2.ResourceContext
		if try == tryPersistentSRK {
			var err error
			srk, err = tpm.CreateResourceContextFromTPM(tcg.SRKHandle)
			if tpm2.IsResourceUnavailableError(err, tcg.SRKHandle) {
				// No SRK - save the error and try creating a transient
				lastError = ErrTPMProvisioning
				continue
			} else if err != nil {
				// This is an unexpected error
				return nil, xerrors.Errorf("cannot create context for SRK: %w", err)
			}
		} else {
			var err error
			srk, _, _, _, _, err = tpm.CreatePrimary(tpm.OwnerHandleContext(), nil, selectSrkTemplate(tpm, session), nil, nil, session)
			if isAuthFailError(err, tpm2.CommandCreatePrimary, 1) {
				// We don't know the authorization value for the storage hierarchy - ignore
				// this so we end up returning the last error.
				continue
			} else if err != nil {
				// This is an unexpected error
				return nil, xerrors.Errorf("cannot create transient SRK: %w", err)
			}
			defer tpm.FlushContext(srk)
		}

		// Load the key data
		keyObject, err := k.load(tpm, srk, session)
		if isLoadInvalidParamError(err) || isImportInvalidParamError(err) {
			// The supplied key data is invalid or is not protected by the supplied SRK.
			lastError = InvalidKeyDataError{
				fmt.Sprintf("cannot load sealed key object into TPM: %v. Either the sealed key object is bad or the TPM owner has changed", err)}
			continue
		} else if isLoadInvalidParentError(err) || isImportInvalidParentError(err) {
			// The supplied SRK is not a valid storage parent.
			lastError = ErrTPMProvisioning
			continue
		} else if err != nil {
			// This is an unexpected error
			return nil, xerrors.Errorf("cannot load sealed key object into TPM: %w", err)
		}

		return keyObject, nil
	}

	// No more attempts left - return the last error
	return nil, lastError
}

// UnsealFromTPM will load the TPM sealed object in to the TPM and attempt to unseal it, returning the cleartext key on success.
//
// If the TPM's dictionary attack logic has been triggered, a ErrTPMLockout error will be returned.
//
// If the TPM is not provisioned correctly, then a ErrTPMProvisioning error will be returned. In this case, ProvisionTPM should be
// called to attempt to resolve this.
//
// If the TPM sealed object cannot be loaded in to the TPM for reasons other than the lack of a storage root key, then a
// InvalidKeyDataError error will be returned. This could be caused because the sealed object data is invalid in some way, or because
// the sealed object is associated with another TPM owner (the TPM has been cleared since the sealed key data file was created with
// SealKeyToTPM), or because the TPM object at the persistent handle reserved for the storage root key has a public area that looks
// like a valid storage root key but it was created with the wrong template. This latter case is really caused by an incorrectly
// provisioned TPM, but it isn't possible to detect this. A subsequent call to SealKeyToTPM or ProvisionTPM will rectify this.
//
// If the TPM's current PCR values are not consistent with the PCR protection policy for this key file, a InvalidKeyDataError error
// will be returned.
//
// If any of the metadata in this key file is invalid, a InvalidKeyDataError error will be returned.
//
// If the TPM is missing any persistent resources associated with this key file, then a InvalidKeyDataError error will be returned.
//
// If the key file has been superceded (eg, by a call to SealedKeyObject.UpdatePCRProtectionPolicy), then a InvalidKeyDataError error
// will be returned.
//
// If the signature of the updatable part of the key file's authorization policy is invalid, then a InvalidKeyDataError error will
// be returned.
//
// If the metadata for the updatable part of the key file's authorization policy is not consistent with the approved policy, then a
// InvalidKeyDataError error will be returned.
//
// If the authorization policy check fails during unsealing, then a InvalidKeyDataError error will be returned. Note that this
// condition can also occur as the result of an incorrectly provisioned TPM, which will be detected during a subsequent call to
// SealKeyToTPM.
//
// On success, the unsealed cleartext key is returned as the first return value, and the private part of the key used for
// authorizing PCR policy updates with UpdateKeyPCRProtectionPolicy is returned as the second return value.
func (k *SealedKeyObject) UnsealFromTPM(tpm *Connection) (key []byte, authKey PolicyAuthKey, err error) {
	// Check if the TPM is in lockout mode
	props, err := tpm.GetCapabilityTPMProperties(tpm2.PropertyPermanent, 1)
	if err != nil {
		return nil, nil, xerrors.Errorf("cannot fetch properties from TPM: %w", err)
	}

	if tpm2.PermanentAttributes(props[0].Value)&tpm2.AttrInLockout > 0 {
		return nil, nil, ErrTPMLockout
	}

	// Use the HMAC session created when the connection was opened for parameter encryption rather than creating a new one.
	hmacSession := tpm.HmacSession()

	keyObject, err := k.loadForUnseal(tpm.TPMContext, hmacSession)
	if err != nil {
		return nil, nil, err
	}
	defer tpm.FlushContext(keyObject)

	// Begin and execute policy session
	policySession, err := tpm.StartAuthSession(nil, nil, tpm2.SessionTypePolicy, nil, k.data.Public().NameAlg)
	if err != nil {
		return nil, nil, xerrors.Errorf("cannot start policy session: %w", err)
	}
	defer tpm.FlushContext(policySession)

	if err := executePolicySession(tpm.TPMContext, policySession, k.data.Version(), k.data.StaticPolicy(), k.data.DynamicPolicy(), hmacSession); err != nil {
		err = xerrors.Errorf("cannot complete authorization policy assertions: %w", err)
		switch {
		case isDynamicPolicyDataError(err):
			// TODO: Add a separate error for this
			return nil, nil, InvalidKeyDataError{err.Error()}
		case isStaticPolicyDataError(err):
			return nil, nil, InvalidKeyDataError{err.Error()}
		case tpm2.IsResourceUnavailableError(err, lockNVHandle):
			return nil, nil, InvalidKeyDataError{"required legacy lock NV index is not present"}
		}
		return nil, nil, err
	}

	// Unseal
	keyData, err := tpm.Unseal(keyObject, policySession, hmacSession.IncludeAttrs(tpm2.AttrResponseEncrypt))
	switch {
	case tpm2.IsTPMSessionError(err, tpm2.ErrorPolicyFail, tpm2.CommandUnseal, 1):
		return nil, nil, InvalidKeyDataError{"the authorization policy check failed during unsealing"}
	case err != nil:
		return nil, nil, xerrors.Errorf("cannot unseal key: %w", err)
	}

	if k.data.Version() == 0 {
		return keyData, nil, nil
	}

	var sealedData sealedData
	if _, err := mu.UnmarshalFromBytes(keyData, &sealedData); err != nil {
		return nil, nil, InvalidKeyDataError{err.Error()}
	}

	return sealedData.Key, sealedData.AuthPrivateKey, nil
}
