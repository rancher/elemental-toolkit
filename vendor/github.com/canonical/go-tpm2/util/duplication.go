// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package util

import (
	"crypto"
	"errors"

	"golang.org/x/xerrors"

	"github.com/canonical/go-tpm2"
)

// UnwrapDuplicationObjectToSensitive unwraps the supplied duplication object and returns the
// corresponding sensitive area. If inSymSeed is supplied, then it is assumed that the object
// has an outer wrapper. In this case, privKey, parentNameAlg and parentSymmetricAlg must be
// supplied - privKey is the key with which inSymSeed is protected, parentNameAlg is the name
// algorithm for the parent key (and must not be HashAlgorithmNull), and parentSymmetricAlg
// defines the symmetric algorithm for the parent key (and the Algorithm field must not be
// SymObjectAlgorithmNull).
//
// If symmetricAlg is supplied and the Algorithm field is not SymObjectAlgorithmNull, then it is
// assumed that the object has an inner wrapper. In this case, the symmetric key for the inner
// wrapper must be supplied using the encryptionKey argument.
func UnwrapDuplicationObjectToSensitive(duplicate tpm2.Private, public *tpm2.Public, privKey crypto.PrivateKey, parentNameAlg tpm2.HashAlgorithmId, parentSymmetricAlg *tpm2.SymDefObject, encryptionKey tpm2.Data, inSymSeed tpm2.EncryptedSecret, symmetricAlg *tpm2.SymDefObject) (*tpm2.Sensitive, error) {
	var seed []byte
	if len(inSymSeed) > 0 {
		if privKey == nil {
			return nil, errors.New("parent private key is required for outer wrapper")
		}

		var err error
		seed, err = CryptSecretDecrypt(privKey, parentNameAlg, []byte(tpm2.DuplicateString), inSymSeed)
		if err != nil {
			return nil, xerrors.Errorf("cannot decrypt symmetric seed: %w", err)
		}
	}

	name, err := public.Name()
	if err != nil {
		return nil, xerrors.Errorf("cannot compute name: %w", err)
	}

	return DuplicateToSensitive(duplicate, name, parentNameAlg, parentSymmetricAlg, seed, symmetricAlg, encryptionKey)
}

// CreateDuplicationObjectFromSensitive creates a duplication object that can be imported in to a
// TPM from the supplied sensitive area.
//
// If symmetricAlg is supplied and the Algorithm field is not SymObjectAlgorithmNull, this function
// will apply an inner wrapper to the duplication object. If encryptionKeyIn is supplied, it will be
// used as the symmetric key for the inner wrapper. It must have a size appropriate for the selected
// symmetric algorithm. If encryptionKeyIn is not supplied, a symmetric key will be created and
// returned
//
// If parentPublic is supplied, an outer wrapper will be applied to the duplication object. The
// parentPublic argument should correspond to the public area of the storage key to which the
// duplication object will be imported. When applying the outer wrapper, the seed used to derice the
// symmetric key and HMAC key will be encrypted using parentPublic and returned.
func CreateDuplicationObjectFromSensitive(sensitive *tpm2.Sensitive, public, parentPublic *tpm2.Public, encryptionKeyIn tpm2.Data, symmetricAlg *tpm2.SymDefObject) (encryptionKeyOut tpm2.Data, duplicate tpm2.Private, outSymSeed tpm2.EncryptedSecret, err error) {
	if public.Attrs&(tpm2.AttrFixedTPM|tpm2.AttrFixedParent) != 0 {
		return nil, nil, nil, errors.New("object must be a duplication root")
	}

	if public.Attrs&tpm2.AttrEncryptedDuplication != 0 {
		if symmetricAlg == nil || symmetricAlg.Algorithm == tpm2.SymObjectAlgorithmNull {
			return nil, nil, nil, errors.New("symmetric algorithm must be supplied for an object with AttrEncryptedDuplication")
		}
		if parentPublic == nil {
			return nil, nil, nil, errors.New("parent object must be supplied for an object with AttrEncryptedDuplication")
		}
	}

	name, err := public.Name()
	if err != nil {
		return nil, nil, nil, xerrors.Errorf("cannot compute name: %w", err)
	}

	var seed []byte
	if parentPublic != nil {
		if !parentPublic.IsStorageParent() || !parentPublic.IsAsymmetric() {
			return nil, nil, nil, errors.New("parent object must be an asymmetric storage key")
		}
		outSymSeed, seed, err = tpm2.CryptSecretEncrypt(parentPublic, []byte(tpm2.DuplicateString))
		if err != nil {
			return nil, nil, nil, xerrors.Errorf("cannot create encrypted symmetric seed: %w", err)
		}
	}

	encryptionKeyOut, duplicate, err = SensitiveToDuplicate(sensitive, name, parentPublic, seed, symmetricAlg, encryptionKeyIn)
	if err != nil {
		return nil, nil, nil, err
	}

	return encryptionKeyOut, duplicate, outSymSeed, nil
}
