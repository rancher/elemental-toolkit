// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package drbg

import (
	"bytes"
	"crypto"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/xerrors"
)

func seedLength(h crypto.Hash) (int, error) {
	switch h {
	case crypto.SHA1, crypto.SHA224, crypto.SHA256, crypto.SHA512_224, crypto.SHA512_256:
		return 55, nil
	case crypto.SHA384, crypto.SHA512:
		return 111, nil
	default:
		return 0, fmt.Errorf("unsupported digest algorithm: %v", h)
	}
}

// hashgen implements Hashgen, described in section 10.1.1.4 of
// SP800-90A.
func hashgen(alg crypto.Hash, v []byte, requestedBytes int) []byte {
	// 1) m = requested_no_of_bits / outlen.
	m := (requestedBytes + (alg.Size() - 1)) / alg.Size()

	// 2) data = V.
	data := v

	// 3) W = the Null string.
	var W bytes.Buffer

	mod := twoExp(uint(len(v) * 8))
	h := alg.New()
	tmp := new(big.Int)

	// 4) For i = 1 to m
	for i := 1; i <= m; i++ {
		// 4.1) w = Hash (data).
		h.Reset()
		h.Write(data)
		w := h.Sum(nil)

		// 4.2) W = W || w.
		W.Write(w)

		// 4.3) data = (data + 1) mod 2^seedlen.
		tmp.SetBytes(data)
		tmp.Add(tmp, one)
		tmp.Mod(tmp, mod)

		data = zeroExtendBytes(tmp, len(v))
	}

	// 5) returned_bits = leftmost (W, requested_no_of_bits).
	return W.Bytes()[:requestedBytes]
}

// hash_df implements the Hash_df function described in section 10.3.1 of
// SP800-90A.
func hash_df(alg crypto.Hash, input []byte, requestedBytes int) []byte {
	// 1) temp = the Null string.
	var temp bytes.Buffer

	// 2) len = no_of_bits_to_return / outlen.
	n := (requestedBytes + (alg.Size() - 1)) / alg.Size()
	if n > 0xff {
		panic("invalid requested bytes")
	}

	// 3) counter = 0x01.
	counter := uint8(1)

	h := alg.New()
	requestedBits := uint32(requestedBytes * 8)

	// 4) For i = 1 to len do
	for i := 1; i <= n; i++ {
		// 4.1) temp = temp || Hash (counter || no_of_bits_to_return || input_string).
		h.Reset()
		h.Write([]byte{counter})
		binary.Write(h, binary.BigEndian, requestedBits)
		h.Write(input)

		temp.Write(h.Sum(nil))

		// 4.2) counter = counter + 1.
		counter += 1
	}

	// 5) requested_bits = leftmost (temp, no_of_bits_to_return).
	return temp.Bytes()[:requestedBytes]
}

type hashDRBG struct {
	h crypto.Hash

	v             []byte
	c             []byte
	reseedCounter uint64
}

func (d *hashDRBG) seedLen() int {
	return len(d.v)
}

// instantiate implements Hash_DRBG_Instantiate_algorithm, described in section 10.1.1.2 of
// SP800-90A.
func (d *hashDRBG) instantiate(entropyInput, nonce, personalization []byte, securityStrength int) {
	// 1) seed_material = entropy_input || nonce || personalization_string.
	var seedMaterial bytes.Buffer
	seedMaterial.Write(entropyInput)
	seedMaterial.Write(nonce)
	seedMaterial.Write(personalization)

	// 2) seed = Hash_df (seed_material, seedlen).
	seed := hash_df(d.h, seedMaterial.Bytes(), d.seedLen())

	// 3) V = seed.
	d.v = seed

	// 4) C = Hash_df ((0x00 || V), seedlen).
	d.c = hash_df(d.h, append([]byte{0x00}, d.v...), d.seedLen())

	// 5) reseed_counter = 1.
	d.reseedCounter = 1
}

// reseed implements Hash_DRBG_Reseed_algorithm, described in section 10.1.1.3 of
// SP800-90A.
func (d *hashDRBG) reseed(entropyInput, additionalInput []byte) {
	// 1) seed_material = 0x01 || V || entropy_input || additional_input.
	var seedMaterial bytes.Buffer
	seedMaterial.Write([]byte{0x01})
	seedMaterial.Write(d.v)
	seedMaterial.Write(entropyInput)
	seedMaterial.Write(additionalInput)

	// 2) seed = Hash_df (seed_material, seedlen).
	seed := hash_df(d.h, seedMaterial.Bytes(), d.seedLen())

	// 3) V = seed.
	d.v = seed

	// 4) C = Hash_df ((0x00 || V), seedlen).
	d.c = hash_df(d.h, append([]byte{0x00}, seed...), d.seedLen())

	// 5) reseed_counter = 1.
	d.reseedCounter = 1
}

// generate implements Hash_DRBG_Generate_algorithm, described in section 10.1.1.4 of
// SP800-90A.
func (d *hashDRBG) generate(additionalInput, data []byte) error {
	// 1) If reseed_counter > reseed_interval, then return an indication that a reseed
	// is required.
	if d.reseedCounter > 1<<48 {
		return ErrReseedRequired
	}

	mod := twoExp(uint(d.seedLen() * 8))

	// 2) If (additional_input ≠ Null), then do
	if len(additionalInput) > 0 {
		// 2.1) w = Hash (0x02 || V || additional_input).
		h := d.h.New()
		h.Write([]byte{0x02})
		h.Write(d.v)
		h.Write(additionalInput)

		w := new(big.Int).SetBytes(h.Sum(nil))

		// 2.2) V = (V + w) mod 2^seedlen.
		v := new(big.Int).SetBytes(d.v)
		v.Add(v, w)
		v.Mod(v, mod)
		d.v = zeroExtendBytes(v, d.seedLen())
	}

	// 3) (returned_bits) = Hashgen (requested_number_of_bits, V).
	returnedBytes := hashgen(d.h, d.v, len(data))
	copy(data, returnedBytes)

	// 4) H = Hash (0x03 || V).
	hash := d.h.New()
	hash.Write([]byte{0x03})
	hash.Write(d.v)
	h := hash.Sum(nil)

	// 5) V = (V + H + C + reseed_counter) mod 2^seedlen.
	v := new(big.Int).SetBytes(d.v)
	v.Add(v, new(big.Int).SetBytes(h))
	v.Add(v, new(big.Int).SetBytes(d.c))
	v.Add(v, big.NewInt(int64(d.reseedCounter)))
	v.Mod(v, mod)

	d.v = zeroExtendBytes(v, d.seedLen())

	// 6) reseed_counter = reseed_counter + 1.
	d.reseedCounter += 1

	return nil
}

// NewHash creates a new hash based DRBG as specified in section 10.1.1 of SP-800-90A.
// The DRBG uses the supplied hash algorithm.
//
// The optional personalization argument is combined with entropy input to derive the
// initial seed. This argument can be used to differentiate this instantiation from others.
//
// The optional entropySource argument allows the default entropy source (rand.Reader from
// the crypto/rand package) to be overridden. The supplied entropy source must be truly
// random.
func NewHash(h crypto.Hash, personalization []byte, entropySource io.Reader) (*DRBG, error) {
	seedLen, err := seedLength(h)
	if err != nil {
		return nil, xerrors.Errorf("cannot compute seed length: %w", err)
	}
	d := &DRBG{impl: &hashDRBG{h: h, v: make([]byte, seedLen)}}
	if err := d.instantiate(personalization, entropySource, h.Size()/2); err != nil {
		return nil, xerrors.Errorf("cannot instantiate: %w", err)
	}

	return d, nil
}

// NewHashWithExternalEntropy creates a new hash based DRBG as specified in section
// 10.1.1 of SP-800-90A. The DRBG uses the supplied hash algorithm. The entropyInput and
// nonce arguments provide the initial entropy to seed the created DRBG.
//
// The optional personalization argument is combined with entropy input to derive the
// initial seed. This argument can be used to differentiate this instantiation from others.
//
// The optional entropySource argument provides the entropy source for future reseeding. If
// it is not supplied, then the DRBG can only be reseeded with externally supplied entropy.
// The supplied entropy source must be truly random.
func NewHashWithExternalEntropy(h crypto.Hash, entropyInput, nonce, personalization []byte, entropySource io.Reader) (*DRBG, error) {
	seedLen, err := seedLength(h)
	if err != nil {
		return nil, xerrors.Errorf("cannot compute seed length: %w", err)
	}
	d := &DRBG{impl: &hashDRBG{h: h, v: make([]byte, seedLen)}}
	if err := d.instantiateWithExternalEntropy(entropyInput, nonce, personalization, entropySource, h.Size()/2); err != nil {
		return nil, err
	}
	return d, nil
}
