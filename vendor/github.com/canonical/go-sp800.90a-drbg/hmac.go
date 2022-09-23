// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package drbg

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	"hash"
	"io"

	"golang.org/x/xerrors"
)

type hmacDRBG struct {
	h crypto.Hash

	v             []byte
	key           []byte
	reseedCounter uint64
}

// update implements HMAC_DRBG_Update, described in section 10.1.2.2 of
// SP800-90A.
func (d *hmacDRBG) update(providedData []byte) {
	// 1) K = HMAC (K, V || 0x00 || provided_data).
	h := hmac.New(func() hash.Hash { return d.h.New() }, d.key)
	h.Write(d.v)
	h.Write([]byte{0x00})
	h.Write(providedData)
	d.key = h.Sum(nil)

	// 2) V = HMAC (K, V).
	h = hmac.New(func() hash.Hash { return d.h.New() }, d.key)
	h.Write(d.v)
	d.v = h.Sum(nil)

	// 3) If (provided_data = Null), then return K and V.
	if len(providedData) == 0 {
		return
	}

	// 4) K = HMAC (K, V || 0x01 || provided_data).
	h = hmac.New(func() hash.Hash { return d.h.New() }, d.key)
	h.Write(d.v)
	h.Write([]byte{0x01})
	h.Write(providedData)
	d.key = h.Sum(nil)

	// 5) V = HMAC (K, V).
	h = hmac.New(func() hash.Hash { return d.h.New() }, d.key)
	h.Write(d.v)
	d.v = h.Sum(nil)
}

// instantiate implements HMAC_DRBG_Instantiate_algorithm, described in section 10.1.2.3 of
// SP800-90A.
func (d *hmacDRBG) instantiate(entropyInput, nonce, personalization []byte, securityStrength int) {
	// 1) seed_material = entropy_input || nonce || personalization_string.
	var seedMaterial bytes.Buffer
	seedMaterial.Write(entropyInput)
	seedMaterial.Write(nonce)
	seedMaterial.Write(personalization)

	// 2) Key = 0x00 00...00. Comment: outlen bits.
	d.key = make([]byte, d.h.Size())

	// 3) V = 0x01 01...01. Comment: outlen bits.
	d.v = make([]byte, d.h.Size())
	for i := range d.v {
		d.v[i] = 0x01
	}

	// 4) (Key, V) = HMAC_DRBG_Update (seed_material, Key, V).
	d.update(seedMaterial.Bytes())

	// 5) reseed_counter = 1.
	d.reseedCounter = 1
}

// reseed implements HMAC_DRBG_Reseed_algorithm, described in section 10.1.2.4 of
// SP800-90A.
func (d *hmacDRBG) reseed(entropyInput, additionalInput []byte) {
	// 1) seed_material = entropy_input || additional_input.
	var seedMaterial bytes.Buffer
	seedMaterial.Write(entropyInput)
	seedMaterial.Write(additionalInput)

	// 2) (Key, V) = HMAC_DRBG_Update (seed_material, Key, V).
	d.update(seedMaterial.Bytes())

	// 3) reseed_counter = 1.
	d.reseedCounter = 1
}

// generate implements HMAC_DRBG_Generate_algorithm, described in section 10.1.2.5 of
// SP800-90A.
func (d *hmacDRBG) generate(additionalInput, data []byte) error {
	// 1) If reseed_counter > reseed_interval, then return an indication that a
	// reseed is required.
	if d.reseedCounter > 1<<48 {
		return ErrReseedRequired
	}

	// 2) If additional_input ≠ Null, then
	// (Key, V) = HMAC_DRBG_Update (additional_input, Key, V).
	if len(additionalInput) > 0 {
		d.update(additionalInput)
	}

	// 3) temp = Null.
	var temp bytes.Buffer

	h := hmac.New(func() hash.Hash { return d.h.New() }, d.key)

	// 4) While (len (temp) < requested_number_of_bits) do:
	for temp.Len() < len(data) {
		// 4.1) V = HMAC (Key , V).
		h.Reset()
		h.Write(d.v)
		d.v = h.Sum(nil)

		// 4.2) temp = temp || V.
		temp.Write(d.v)
	}

	// 5) returned_bits = leftmost (temp, requested_number_of_bits).
	copy(data, temp.Bytes())

	// 6) (Key, V) = HMAC_DRBG_Update (additional_input, Key, V).
	d.update(additionalInput)

	// 7) reseed_counter = reseed_counter + 1.
	d.reseedCounter += 1

	return nil
}

// NewHMAC creates a new HMAC based DRBG as specified in section 10.1.2 of SP-800-90A.
// The DRBG uses the supplied hash algorithm.
//
// The optional personalization argument is combined with entropy input to derive the
// initial seed. This argument can be used to differentiate this instantiation from others.
//
// The optional entropySource argument allows the default entropy source (rand.Reader from
// the crypto/rand package) to be overridden. The supplied entropy source must be truly
// random.
func NewHMAC(h crypto.Hash, personalization []byte, entropySource io.Reader) (*DRBG, error) {
	d := &DRBG{impl: &hmacDRBG{h: h}}
	if err := d.instantiate(personalization, entropySource, h.Size()/2); err != nil {
		return nil, xerrors.Errorf("cannot instantiate: %w", err)
	}

	return d, nil
}

// NewHMACWithExternalEntropy creates a new hash based DRBG as specified in section
// 10.1.2 of SP-800-90A. The DRBG uses the supplied hash algorithm. The entropyInput and
// nonce arguments provide the initial entropy to seed the created DRBG.
//
// The optional personalization argument is combined with entropy input to derive the
// initial seed. This argument can be used to differentiate this instantiation from others.
//
// The optional entropySource argument provides the entropy source for future reseeding. If
// it is not supplied, then the DRBG can only be reseeded with externally supplied entropy.
// The supplied entropy source must be truly random.
func NewHMACWithExternalEntropy(h crypto.Hash, entropyInput, nonce, personalization []byte, entropySource io.Reader) (*DRBG, error) {
	d := &DRBG{impl: &hmacDRBG{h: h}}
	if err := d.instantiateWithExternalEntropy(entropyInput, nonce, personalization, entropySource, h.Size()/2); err != nil {
		return nil, err
	}
	return d, nil
}
