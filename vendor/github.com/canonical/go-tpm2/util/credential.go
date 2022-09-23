// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package util

import (
	"errors"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"
)

// MakeCredential performs the duties of a certificate authority in order to
// create an activation credential. It produces a seed and encrypts this with
// the supplied public key. The seed and supplied object name is then used to
// apply an outer wrapper to the credential.
//
// The encrypted credential blob and encrypted seed are returned, and these can
// be passed to the TPM2_ActivateCredential on the TPM on which both the private
// part of key and the object associated with objectName are loaded.
func MakeCredential(key *tpm2.Public, credential tpm2.Digest, objectName tpm2.Name) (credentialBlob tpm2.IDObjectRaw, secret tpm2.EncryptedSecret, err error) {
	if !key.IsStorageParent() || !key.IsAsymmetric() {
		return nil, nil, errors.New("key must be an asymmetric storage parent")
	}
	if !key.NameAlg.Available() {
		return nil, nil, errors.New("name algorithm for key is not available")
	}

	secret, seed, err := tpm2.CryptSecretEncrypt(key, []byte(tpm2.IdentityKey))
	if err != nil {
		return nil, nil, err
	}

	credentialBlob = mu.MustMarshalToBytes(credential)

	credentialBlob, err = ProduceOuterWrap(key.NameAlg, &key.Params.AsymDetail(key.Type).Symmetric, objectName, seed, false, credentialBlob)
	if err != nil {
		return nil, nil, err
	}

	return credentialBlob, secret, nil
}
