// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package drbg

import (
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/xerrors"
)

var (
	dfKey = []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}
)

type blockCipher interface {
	encrypt(key, data []byte) []byte
	blockSize() int
}

type aesBlockCipherImpl struct{}

var aesBlockCipher = aesBlockCipherImpl{}

func (b aesBlockCipherImpl) encrypt(key, data []byte) (out []byte) {
	c, err := aes.NewCipher(key)
	if err != nil {
		panic(fmt.Sprintf("cannot create cipher: %v", err))
	}

	out = make([]byte, len(data))
	c.Encrypt(out, data)
	return
}

func (b aesBlockCipherImpl) blockSize() int {
	return aes.BlockSize
}

// bcc implements BCC, described in section 10.3.3 of SP800-90A.
func bcc(b blockCipher, key, data []byte) []byte {
	if len(data)%b.blockSize() != 0 {
		panic("data length must be a multiple of the block length")
	}

	// 1) chaining_value = 0(x outlen)
	chainingValue := make([]byte, b.blockSize())

	// 2) n = len (data)/outlen.
	n := len(data) / b.blockSize()

	// 3) Starting with the leftmost bits of data, split data into n blocks of
	// outlen bits each, forming block[1] to block[n].
	blocks := make([][]byte, n)
	for i := 0; i < n; i++ {
		blocks[i] = data[i*b.blockSize() : (i+1)*b.blockSize()]
	}

	inputBlock := make([]byte, b.blockSize())

	// 4) For i = 1 to n do
	for i := 0; i < n; i++ {
		for j := 0; j < len(inputBlock); j++ {
			// 4.1) input_block = chaining_value ⊕ block[i].
			inputBlock[j] = chainingValue[j] ^ blocks[i][j]
		}

		// 4.2) chaining_value = Block_Encrypt (Key, input_block).
		chainingValue = b.encrypt(key, inputBlock)
	}

	return chainingValue
}

// block_cipher_df implements Block_Cipher_df, described in section 10.3.2 of SP800-90A.
func block_cipher_df(b blockCipher, keyLen int, input []byte, requestedBytes int) []byte {
	// 2) L = len (input_string)/8.
	l := uint32(len(input))

	// 3) N = number_of_bits_to_return/8.
	n := uint32(requestedBytes)

	// 4) S = L || N || input_string || 0x80.
	var s bytes.Buffer
	binary.Write(&s, binary.BigEndian, l)
	binary.Write(&s, binary.BigEndian, n)
	s.Write(input)
	s.Write([]byte{0x80})

	// 5) While (len (S) mod outlen) ≠ 0, do
	//      S = S || 0x00.
	for s.Len()%b.blockSize() != 0 {
		s.Write([]byte{0x00})
	}

	// 6) temp = the Null string.
	var temp bytes.Buffer

	// 7) i = 0.
	i := uint32(0)

	// 8) K = leftmost (0x00010203...1D1E1F, keylen).
	k := dfKey[:keyLen]

	iv := make([]byte, b.blockSize())

	// 9) While len(temp) < keylen + outlen, do
	for temp.Len() < (keyLen + b.blockSize()) {
		// 9.1) IV = i || 0(x (outlen - len (i))).
		binary.BigEndian.PutUint32(iv, i)

		// 9.2) temp = temp || BCC (K, (IV || S)).
		var data bytes.Buffer
		data.Write(iv)
		data.Write(s.Bytes())

		temp.Write(bcc(b, k, data.Bytes()))

		// 9.3) i = i + 1.
		i += 1
	}

	// 10) K = leftmost (temp, keylen).
	k = make([]byte, keyLen)
	copy(k, temp.Bytes())

	// 11) X = select (temp, keylen+1, keylen+outlen).
	x := make([]byte, b.blockSize())
	copy(x, temp.Bytes()[keyLen:])

	// 12) temp = the Null string.
	temp.Reset()

	// 13) While len (temp) < number_of_bits_to_return, do
	for temp.Len() < requestedBytes {
		// 13.1) = Block_Encrypt (K, X).
		x = b.encrypt(k, x)

		// 13.2) temp = temp || X.
		temp.Write(x)
	}

	// 14) requested_bits = leftmost (temp, number_of_bits_to_return).
	return temp.Bytes()[:requestedBytes]
}

type ctrDRBG struct {
	b blockCipher

	v             []byte
	key           []byte
	reseedCounter uint64
}

func (d *ctrDRBG) keyLen() int {
	return len(d.key)
}

func (d *ctrDRBG) blockSize() int {
	return d.b.blockSize()
}

func (d *ctrDRBG) seedLength() int {
	return d.keyLen() + d.b.blockSize()
}

// update implements CTR_DRBG_Update, described in section 10.2.1.2 of
// SP800-90A.
func (d *ctrDRBG) update(providedData []byte) {
	if len(providedData) != d.seedLength() {
		panic("provided data has the wrong length")
	}

	// 1) temp = Null
	var temp bytes.Buffer

	mod := twoExp(uint(d.blockSize() * 8))

	v := new(big.Int)

	// 2) While (len(temp) < seedLen) do
	for temp.Len() < d.seedLength() {
		// 2.1) V = (V+1) mod 2^blocklen
		v.SetBytes(d.v)
		v.Add(v, one)
		v.Mod(v, mod)
		d.v = zeroExtendBytes(v, d.blockSize())

		// 2.2) output_block = Block_Encrypt (Key, V).
		// 2.3) temp = temp || output_block.
		temp.Write(d.b.encrypt(d.key, d.v))
	}

	// 3) temp = leftmost(temp, seedLen)
	temp.Truncate(d.seedLength())

	// 4) temp = temp ⊕ provided_data.
	for i := 0; i < temp.Len(); i++ {
		temp.Bytes()[i] ^= providedData[i]
	}

	// 5) Key = leftmost (temp, keylen).
	d.key = temp.Bytes()[:d.keyLen()]

	// 6) V = rightmost (temp, blocklen).
	d.v = temp.Bytes()[d.keyLen():]
}

// instantiate implements CTR_DRBG_Instantiate_algorithm, described in section 10.2.1.3.2 of
// SP800-90A.
func (d *ctrDRBG) instantiate(entropyInput, nonce, personalization []byte, securityStrength int) {
	var tmp bytes.Buffer

	// 1) seed_material = entropy_input || nonce || personalization_string.
	tmp.Write(entropyInput)
	tmp.Write(nonce)
	tmp.Write(personalization)
	seedMaterial := tmp.Bytes()

	// 2) seed_material = df (seed_material, seedlen).
	seedMaterial = block_cipher_df(d.b, d.keyLen(), seedMaterial, d.seedLength())

	// 3) Key = 0(x keylen) is done in NewCTR.

	// 4) V = 0(x blocklen).
	d.v = make([]byte, d.blockSize())

	// 5) (Key, V) = CTR_DRBG_Update (seed_material, Key, V).
	d.update(seedMaterial)

	// 6) reseed_counter = 1.
	d.reseedCounter = 1
}

// reseed implements CTR_DRBG_Reseed_algorithm, described in section 10.2.1.4.2 of
// SP800-90A.
func (d *ctrDRBG) reseed(entropyInput, additionalInput []byte) {
	var tmp bytes.Buffer

	// 1) seed_material = entropy_input || additional_input.
	tmp.Write(entropyInput)
	tmp.Write(additionalInput)
	seedMaterial := tmp.Bytes()

	// 2) seed_material = df (seed_material, seedlen).
	seedMaterial = block_cipher_df(d.b, d.keyLen(), seedMaterial, d.seedLength())

	// 3) (Key, V) = CTR_DRBG_Update (seed_material, Key, V).
	d.update(seedMaterial)

	// 4) reseed_counter = 1.
	d.reseedCounter = 1
}

// generate implements CTR_DRBG_Generate_algorithm, described in section 10.2.1.5.2 of
// SP800-90A.
func (d *ctrDRBG) generate(additionalInput, data []byte) error {
	// 1) If reseed_counter > reseed_interval, then return an indication that a
	// reseed is required.
	if d.reseedCounter > 1<<48 {
		return ErrReseedRequired
	}

	// 2) If (additional_input ≠ Null), then
	if len(additionalInput) > 0 {
		// 2.1) additional_input = Block_Cipher_df (additional_input, seedlen).
		additionalInput = block_cipher_df(d.b, d.keyLen(), additionalInput, d.seedLength())

		// 2.2) (Key, V) = CTR_DRBG_Update (additional_input, Key, V).
		d.update(additionalInput)
		// Else additional_input = 0(x seedlen).
	} else {
		additionalInput = make([]byte, d.seedLength())
	}

	// 3) temp = Null.
	var temp bytes.Buffer

	mod := twoExp(uint(d.blockSize() * 8))
	v := new(big.Int)

	// 4) While (len (temp) < requested_number_of_bits) do:
	for temp.Len() < len(data) {
		// 4.1.2) V = (V+1) mod 2^blocklen.
		v.SetBytes(d.v)
		v.Add(v, one)
		v.Mod(v, mod)
		d.v = zeroExtendBytes(v, d.blockSize())

		// 4.2) output_block = Block_Encrypt (Key, V).
		outputBlock := d.b.encrypt(d.key, d.v)

		// 4.3) temp = temp || output_block.
		temp.Write(outputBlock)
	}

	// 5) returned_bits = leftmost (temp, requested_number_of_bits).
	copy(data, temp.Bytes())

	// 6) (Key, V) = CTR_DRBG_Update (additional_input, Key, V).
	d.update(additionalInput)

	// 7) reseed_counter = reseed_counter + 1.
	d.reseedCounter += 1

	return nil
}

// NewCTR creates a new block cipher based DRBG as specified in section 10.2 of SP-800-90A.
// The DRBG uses the AES block cipher.
//
// The optional personalization argument is combined with entropy input to derive the
// initial seed. This argument can be used to differentiate this instantiation from others.
//
// The optional entropySource argument allows the default entropy source (rand.Reader from
// the crypto/rand package) to be overridden. The supplied entropy source must be truly
// random.
func NewCTR(keyLen int, personalization []byte, entropySource io.Reader) (*DRBG, error) {
	switch keyLen {
	case 16, 24, 32:
	default:
		return nil, errors.New("invalid key size")
	}

	d := &DRBG{impl: &ctrDRBG{b: aesBlockCipher, key: make([]byte, keyLen)}}
	if err := d.instantiate(personalization, entropySource, keyLen); err != nil {
		return nil, xerrors.Errorf("cannot instantiate: %w", err)
	}

	return d, nil
}

// NewCTRWithExternalEntropy creates a new block cipher based DRBG as specified in
// section 10.2 of SP-800-90A. The DRBG uses the AES block cipher. The entropyInput and
// nonce arguments provide the initial entropy to seed the created DRBG.
//
// The optional personalization argument is combined with entropy input to derive the
// initial seed. This argument can be used to differentiate this instantiation from others.
//
// The optional entropySource argument provides the entropy source for future reseeding. If
// it is not supplied, then the DRBG can only be reseeded with externally supplied entropy.
// The supplied entropy source must be truly random.
func NewCTRWithExternalEntropy(keyLen int, entropyInput, nonce, personalization []byte, entropySource io.Reader) (*DRBG, error) {
	switch keyLen {
	case 16, 24, 32:
	default:
		return nil, errors.New("invalid key size")
	}

	d := &DRBG{impl: &ctrDRBG{b: aesBlockCipher, key: make([]byte, keyLen)}}
	if err := d.instantiateWithExternalEntropy(entropyInput, nonce, personalization, entropySource, keyLen); err != nil {
		return nil, err
	}
	return d, nil
}
