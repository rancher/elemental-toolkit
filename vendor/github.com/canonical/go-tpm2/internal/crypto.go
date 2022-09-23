// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package internal

import (
	"bytes"
	"crypto"
	"encoding/binary"

	"github.com/canonical/go-sp800.108-kdf"
)

func KDFa(hashAlg crypto.Hash, key, label, contextU, contextV []byte, sizeInBits int) []byte {
	context := make([]byte, len(contextU)+len(contextV))
	copy(context, contextU)
	copy(context[len(contextU):], contextV)
	return kdf.CounterModeKey(kdf.NewHMACPRF(hashAlg), key, label, context, uint32(sizeInBits))
}

func KDFe(hashAlg crypto.Hash, z, label, partyUInfo, partyVInfo []byte, sizeInBits int) []byte {
	digestSize := hashAlg.Size()

	counter := 0
	var res bytes.Buffer

	for bytes := (sizeInBits + 7) / 8; bytes > 0; bytes -= digestSize {
		if bytes < digestSize {
			digestSize = bytes
		}
		counter++

		h := hashAlg.New()

		binary.Write(h, binary.BigEndian, uint32(counter))
		h.Write(z)
		h.Write(label)
		h.Write([]byte{0})
		h.Write(partyUInfo)
		h.Write(partyVInfo)

		res.Write(h.Sum(nil)[0:digestSize])
	}

	outKey := res.Bytes()

	if sizeInBits%8 != 0 {
		outKey[0] &= ((1 << uint(sizeInBits%8)) - 1)
	}
	return outKey
}

func XORObfuscation(hashAlg crypto.Hash, key []byte, contextU, contextV, data []byte) {
	context := make([]byte, len(contextU)+len(contextV))
	copy(context, contextU)
	copy(context[len(contextU):], contextV)

	dataSize := len(data)
	mask := kdf.CounterModeKey(kdf.NewHMACPRF(hashAlg), key, []byte("XOR"), context, uint32(dataSize*8))
	for i := 0; i < dataSize; i++ {
		data[i] ^= mask[i]
	}
}
