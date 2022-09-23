// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019-2021 Canonical Ltd
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
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"io"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"
	"github.com/canonical/go-tpm2/util"

	"golang.org/x/xerrors"
)

const keyPolicyUpdateDataHeader uint32 = 0x55534b50

// keyPolicyUpdateDataRaw_v0 is version 0 of the on-disk format of keyPolicyUpdateData.
type keyPolicyUpdateDataRaw_v0 struct {
	AuthKey        []byte
	CreationData   *tpm2.CreationData
	CreationTicket *tpm2.TkCreation
}

// keyPolicyUpdateData corresponds to the private part of a sealed key object that is required in order to create new dynamic
// authorization policies.
type keyPolicyUpdateData struct {
	version        uint32
	authKey        crypto.PrivateKey
	creationInfo   tpm2.Data
	creationData   *tpm2.CreationData
	creationTicket *tpm2.TkCreation
}

func (d keyPolicyUpdateData) Marshal(w io.Writer) error {
	panic("not implemented")
}

func (d *keyPolicyUpdateData) Unmarshal(r mu.Reader) error {
	var version uint32
	if _, err := mu.UnmarshalFromReader(r, &version); err != nil {
		return xerrors.Errorf("cannot unmarshal version number: %w", err)
	}

	switch version {
	case 0:
		var raw keyPolicyUpdateDataRaw_v0
		if _, err := mu.UnmarshalFromReader(r, &raw); err != nil {
			return xerrors.Errorf("cannot unmarshal data: %w", err)
		}

		authKey, err := x509.ParsePKCS1PrivateKey(raw.AuthKey)
		if err != nil {
			return xerrors.Errorf("cannot parse dynamic authorization policy signing key: %w", err)
		}

		h := crypto.SHA256.New()
		if _, err := mu.MarshalToWriter(h, raw.AuthKey); err != nil {
			panic(fmt.Sprintf("cannot marshal dynamic authorization policy signing key: %v", err))
		}

		*d = keyPolicyUpdateData{
			version:        version,
			authKey:        authKey,
			creationInfo:   h.Sum(nil),
			creationData:   raw.CreationData,
			creationTicket: raw.CreationTicket}
	default:
		return fmt.Errorf("unexpected version number (%d)", version)
	}
	return nil
}

// decodeKeyPolicyUpdateData deserializes keyPolicyUpdateData from the provided io.Reader.
func decodeKeyPolicyUpdateData(r io.Reader) (*keyPolicyUpdateData, error) {
	var header uint32
	if _, err := mu.UnmarshalFromReader(r, &header); err != nil {
		return nil, xerrors.Errorf("cannot unmarshal header: %w", err)
	}
	if header != keyPolicyUpdateDataHeader {
		return nil, fmt.Errorf("unexpected header (%d)", header)
	}

	var d keyPolicyUpdateData
	if _, err := mu.UnmarshalFromReader(r, &d); err != nil {
		return nil, xerrors.Errorf("cannot unmarshal data: %w", err)
	}

	return &d, nil
}

// keyData_v0 represents version 0 of keyData
type keyData_v0 struct {
	KeyPrivate        tpm2.Private
	KeyPublic         *tpm2.Public
	Unused            uint8 // previously AuthModeHint
	StaticPolicyData  *staticPolicyDataRaw_v0
	DynamicPolicyData *dynamicPolicyDataRaw_v0
}

func readKeyDataV0(r io.Reader) (keyData, error) {
	var d *keyData_v0
	if _, err := mu.UnmarshalFromReader(r, &d); err != nil {
		return nil, err
	}
	return d, nil
}

func (_ *keyData_v0) Version() uint32 { return 0 }

func (d *keyData_v0) Private() tpm2.Private {
	return d.KeyPrivate
}

func (d *keyData_v0) Public() *tpm2.Public {
	return d.KeyPublic
}

func (_ *keyData_v0) ImportSymSeed() tpm2.EncryptedSecret { return nil }

func (_ *keyData_v0) Imported(_ tpm2.Private) {
	panic("not supported")
}

func (d *keyData_v0) ValidateData(tpm *tpm2.TPMContext, session tpm2.SessionContext) (tpm2.ResourceContext, error) {
	// Obtain the name of the legacy lock NV index.
	lockNV, err := tpm.CreateResourceContextFromTPM(lockNVHandle, session.IncludeAttrs(tpm2.AttrAudit))
	if err != nil {
		if tpm2.IsResourceUnavailableError(err, lockNVHandle) {
			return nil, keyDataError{errors.New("lock NV index is unavailable")}
		}
		return nil, xerrors.Errorf("cannot create context for lock NV index: %w", err)
	}
	lockNVPub, _, err := tpm.NVReadPublic(lockNV, session.IncludeAttrs(tpm2.AttrAudit))
	if err != nil {
		return nil, xerrors.Errorf("cannot read public area of lock NV index: %w", err)
	}
	lockNVPub.Attrs &^= tpm2.AttrNVReadLocked
	lockNVName, err := lockNVPub.Name()
	if err != nil {
		return nil, xerrors.Errorf("cannot compute name of lock NV index: %w", err)
	}

	// Validate the type and scheme of the dynamic authorization policy signing key.
	authPublicKey := d.StaticPolicyData.AuthPublicKey
	authKeyName, err := authPublicKey.Name()
	if err != nil {
		return nil, keyDataError{xerrors.Errorf("cannot compute name of dynamic authorization policy key: %w", err)}
	}
	if authPublicKey.Type != tpm2.ObjectTypeRSA {
		return nil, keyDataError{errors.New("public area of dynamic authorization policy signing key has the wrong type")}
	}
	authKeyScheme := authPublicKey.Params.AsymDetail(authPublicKey.Type).Scheme
	if authKeyScheme.Scheme != tpm2.AsymSchemeNull {
		if authKeyScheme.Scheme != tpm2.AsymSchemeRSAPSS {
			return nil, keyDataError{errors.New("dynamic authorization policy signing key has unexpected scheme")}
		}
		if authKeyScheme.Details.Any(authKeyScheme.Scheme).HashAlg != authPublicKey.NameAlg {
			return nil, keyDataError{errors.New("dynamic authorization policy signing key algorithm must match name algorithm")}
		}
	}

	// Create a context for the PIN index.
	pinIndexHandle := d.StaticPolicyData.PinIndexHandle
	if pinIndexHandle.Type() != tpm2.HandleTypeNVIndex {
		return nil, keyDataError{errors.New("PIN index handle is invalid")}
	}
	pinIndex, err := tpm.CreateResourceContextFromTPM(pinIndexHandle, session.IncludeAttrs(tpm2.AttrAudit))
	if err != nil {
		if tpm2.IsResourceUnavailableError(err, pinIndexHandle) {
			return nil, keyDataError{errors.New("PIN index is unavailable")}
		}
		return nil, xerrors.Errorf("cannot create context for PIN index: %w", err)
	}

	// Make sure that the static authorization policy data is consistent with the sealed key object's policy.
	if !d.KeyPublic.NameAlg.Available() {
		return nil, keyDataError{errors.New("cannot determine if static authorization policy matches sealed key object: algorithm unavailable")}
	}
	trial := util.ComputeAuthPolicy(d.KeyPublic.NameAlg)
	trial.PolicyAuthorize(nil, authKeyName)
	trial.PolicySecret(pinIndex.Name(), nil)
	trial.PolicyNV(lockNVName, nil, 0, tpm2.OpEq)

	if !bytes.Equal(trial.GetDigest(), d.KeyPublic.AuthPolicy) {
		return nil, keyDataError{errors.New("the sealed key object's authorization policy is inconsistent with the associated metadata or persistent TPM resources")}
	}

	// Validate that the OR policy digests for the PIN index match the public area of the index.
	pinIndexPub, _, err := tpm.NVReadPublic(pinIndex, session.IncludeAttrs(tpm2.AttrAudit))
	if err != nil {
		return nil, xerrors.Errorf("cannot read public area of PIN index: %w", err)
	}
	if !pinIndexPub.NameAlg.Available() {
		return nil, keyDataError{errors.New("cannot determine if PIN index has a valid authorization policy: algorithm unavailable")}
	}
	pinIndexAuthPolicies := d.StaticPolicyData.PinIndexAuthPolicies
	expectedPinIndexAuthPolicies := computeV0PinNVIndexPostInitAuthPolicies(pinIndexPub.NameAlg, authKeyName)
	if len(pinIndexAuthPolicies)-1 != len(expectedPinIndexAuthPolicies) {
		return nil, keyDataError{errors.New("unexpected number of OR policy digests for PIN index")}
	}
	for i, expected := range expectedPinIndexAuthPolicies {
		if !bytes.Equal(expected, pinIndexAuthPolicies[i+1]) {
			return nil, keyDataError{errors.New("unexpected OR policy digest for PIN index")}
		}
	}

	trial = util.ComputeAuthPolicy(pinIndexPub.NameAlg)
	trial.PolicyOR(pinIndexAuthPolicies)
	if !bytes.Equal(pinIndexPub.AuthPolicy, trial.GetDigest()) {
		return nil, keyDataError{errors.New("PIN index has unexpected authorization policy")}
	}

	return pinIndex, nil
}

func (d *keyData_v0) Write(w io.Writer) error {
	_, err := mu.MarshalToWriter(w, d)
	return err
}

func (d *keyData_v0) PcrPolicyCounterHandle() tpm2.Handle {
	return d.StaticPolicyData.PinIndexHandle
}

func (d *keyData_v0) ValidateAuthKey(key crypto.PrivateKey) error {
	pub, ok := d.StaticPolicyData.AuthPublicKey.Public().(*rsa.PublicKey)
	if !ok {
		return keyDataError{errors.New("unexpected dynamic authorization policy public key type")}
	}

	priv, ok := key.(*rsa.PrivateKey)
	if !ok {
		return errors.New("unexpected dynamic authorization policy signing private key type")
	}

	if priv.E != pub.E || priv.N.Cmp(pub.N) != 0 {
		return keyDataError{errors.New("dynamic authorization policy signing private key doesn't match public key")}
	}

	return nil
}

func (d *keyData_v0) StaticPolicy() *staticPolicyData {
	return d.StaticPolicyData.data()
}

func (d *keyData_v0) DynamicPolicy() *dynamicPolicyData {
	return d.DynamicPolicyData.data()
}

func (d *keyData_v0) SetDynamicPolicy(data *dynamicPolicyData) {
	d.DynamicPolicyData = makeDynamicPolicyDataRaw_v0(data)
}
