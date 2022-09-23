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
	"crypto"
	"crypto/ecdsa"
	"errors"
	"io"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"
)

// keyData_v2 represents version 2 of keyData.
type keyData_v2 struct {
	KeyPrivate        tpm2.Private
	KeyPublic         *tpm2.Public
	Unused            uint8 // previously AuthModeHint
	KeyImportSymSeed  tpm2.EncryptedSecret
	StaticPolicyData  *staticPolicyDataRaw_v1
	DynamicPolicyData *dynamicPolicyDataRaw_v0
}

func readKeyDataV2(r io.Reader) (keyData, error) {
	var d *keyData_v2
	if _, err := mu.UnmarshalFromReader(r, &d); err != nil {
		return nil, err
	}
	return d, nil
}

func newKeyData(keyPrivate tpm2.Private, keyPublic *tpm2.Public, importSymSeed tpm2.EncryptedSecret,
	staticPolicyData *staticPolicyDataRaw_v1, dynamicPolicyData *dynamicPolicyDataRaw_v0) keyData {
	return &keyData_v2{
		KeyPrivate:        keyPrivate,
		KeyPublic:         keyPublic,
		KeyImportSymSeed:  importSymSeed,
		StaticPolicyData:  staticPolicyData,
		DynamicPolicyData: dynamicPolicyData}
}

func (d *keyData_v2) AsV1() keyData {
	if d.KeyImportSymSeed != nil {
		panic("importable object cannot be converted to v1")
	}
	return &keyData_v1{
		KeyPrivate:        d.KeyPrivate,
		KeyPublic:         d.KeyPublic,
		StaticPolicyData:  d.StaticPolicyData,
		DynamicPolicyData: d.DynamicPolicyData}
}

func (d *keyData_v2) Version() uint32 {
	if d.KeyImportSymSeed == nil {
		// The only difference between v1 and v2 is support for
		// importable objects. Pretend to be v1 if the object
		// doesn't need importing.
		return 1
	}
	return 2
}

func (d *keyData_v2) Private() tpm2.Private {
	return d.KeyPrivate
}

func (d *keyData_v2) Public() *tpm2.Public {
	return d.KeyPublic
}

func (d *keyData_v2) ImportSymSeed() tpm2.EncryptedSecret {
	return d.KeyImportSymSeed
}

func (d *keyData_v2) Imported(priv tpm2.Private) {
	if d.KeyImportSymSeed == nil {
		panic("does not need to be imported")
	}
	d.KeyPrivate = priv
	d.KeyImportSymSeed = nil
}

func (d *keyData_v2) ValidateData(tpm *tpm2.TPMContext, session tpm2.SessionContext) (tpm2.ResourceContext, error) {
	if d.KeyImportSymSeed != nil {
		return nil, errors.New("cannot validate importable key data")
	}
	return d.AsV1().ValidateData(tpm, session)
}

func (d *keyData_v2) Write(w io.Writer) error {
	if d.KeyImportSymSeed == nil {
		// The only difference between v1 and v2 is support for
		// importable objects. Implicitly downgrade to v1 on write
		// if the object doesn't need importing.
		return d.AsV1().Write(w)
	}

	_, err := mu.MarshalToWriter(w, d)
	return err
}

func (d *keyData_v2) PcrPolicyCounterHandle() tpm2.Handle {
	return d.StaticPolicyData.PCRPolicyCounterHandle
}

func (d *keyData_v2) ValidateAuthKey(key crypto.PrivateKey) error {
	pub, ok := d.StaticPolicyData.AuthPublicKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return keyDataError{errors.New("unexpected dynamic authorization policy public key type")}
	}

	priv, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return errors.New("unexpected dynamic authorization policy signing private key type")
	}

	expectedX, expectedY := priv.Curve.ScalarBaseMult(priv.D.Bytes())
	if expectedX.Cmp(pub.X) != 0 || expectedY.Cmp(pub.Y) != 0 {
		return keyDataError{errors.New("dynamic authorization policy signing private key doesn't match public key")}
	}

	return nil
}

func (d *keyData_v2) StaticPolicy() *staticPolicyData {
	return d.StaticPolicyData.data()
}

func (d *keyData_v2) DynamicPolicy() *dynamicPolicyData {
	return d.DynamicPolicyData.data()
}

func (d *keyData_v2) SetDynamicPolicy(data *dynamicPolicyData) {
	d.DynamicPolicyData = makeDynamicPolicyDataRaw_v0(data)
}
