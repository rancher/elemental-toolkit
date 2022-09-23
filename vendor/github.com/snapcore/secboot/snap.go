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

package secboot

import (
	"crypto"
	"crypto/hmac"
	_ "crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"hash"

	"github.com/snapcore/snapd/asserts"

	"golang.org/x/xerrors"
)

var sha3_384oid = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 9}

// SnapModel exposes the details of a snap device model that are bound
// to an encrypted container.
type SnapModel interface {
	Series() string
	BrandID() string
	Model() string
	Grade() asserts.ModelGrade
	SignKeyID() string
}

func computeSnapModelHMAC(alg crypto.Hash, key []byte, model SnapModel) (snapModelHMAC, error) {
	// XXX: Probably would be nice to know the hash algorithm used for the signing key,
	// rather than just assuming SHA3-384 here. Note that the actual algorithm ID here
	// isn't important - what is important is that this ID changes if the hash algorithm
	// changes to one with a different length.
	signKeyHashAlg, err := asn1.Marshal(sha3_384oid)
	if err != nil {
		return nil, xerrors.Errorf("cannot marshal sign key hash algorithm: %w", err)
	}

	signKeyId, err := base64.RawURLEncoding.DecodeString(model.SignKeyID())
	if err != nil {
		return nil, xerrors.Errorf("cannot decode signing key ID: %w", err)
	}

	h := hmac.New(func() hash.Hash { return alg.New() }, key)
	h.Write(signKeyHashAlg)
	h.Write(signKeyId)
	h.Write([]byte(model.BrandID()))
	d := h.Sum(nil)

	h = hmac.New(func() hash.Hash { return alg.New() }, key)
	h.Write(d)
	h.Write([]byte(model.Model()))
	d = h.Sum(nil)

	h = hmac.New(func() hash.Hash { return alg.New() }, key)
	h.Write(d)
	h.Write([]byte(model.Series()))
	binary.Write(h, binary.LittleEndian, model.Grade().Code())

	return h.Sum(nil), nil
}
