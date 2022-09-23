package afis // import "maze.io/x/crypto/afis"

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"hash"
	"io"
	"math"
)

// Errors.
var (
	ErrMinStripe = errors.New("afis: at least one stripe is required")
	ErrDataLen   = errors.New("afis: data length is not multiple of stripes")
)

// DefaultHash is our default hashing function.
var DefaultHash = sha1.New

// Split data using the default SHA-1 hash.
func Split(data []byte, stripes int) ([]byte, error) {
	return SplitHash(data, stripes, DefaultHash)
}

// SplitHash splits data using the selected hash function.
func SplitHash(data []byte, stripes int, hashFunc func() hash.Hash) ([]byte, error) {
	if stripes < 1 {
		return nil, ErrMinStripe
	}
	var (
		blockSize = len(data)
		block     = make([]byte, blockSize)
		random    = make([]byte, blockSize)
		splitted  []byte
	)
	for i := 0; i < stripes-1; i++ {
		if _, err := io.ReadFull(rand.Reader, random); err != nil {
			return nil, err
		}
		splitted = append(splitted, random...)

		xor(block, random, block)
		block = diffuse(block, blockSize, hashFunc)
	}

	size := len(splitted)
	splitted = append(splitted, make([]byte, blockSize)...)
	xor(splitted[size:], block, data)

	return splitted, nil
}

// Merge data splitted previously with Split using the default SHA-1 hash.
func Merge(data []byte, stripes int) ([]byte, error) {
	return MergeHash(data, stripes, DefaultHash)
}

// MergeHash merges data splitted previously with the selected hash function.
func MergeHash(data []byte, stripes int, hashFunc func() hash.Hash) ([]byte, error) {
	if len(data)%stripes != 0 {
		return nil, ErrDataLen
	}

	var (
		blockSize = len(data) / stripes
		block     = make([]byte, blockSize)
	)
	for i := 0; i < stripes-1; i++ {
		offset := i * blockSize
		xor(block, data[offset:offset+blockSize], block)
		block = diffuse(block, blockSize, hashFunc)
	}

	xor(block, data[(stripes-1)*blockSize:], block)
	return block, nil
}

func xor(dst, src1, src2 []byte) {
	for i := range dst {
		dst[i] = src1[i] ^ src2[i]
	}
}

func diffuse(block []byte, size int, hashFunc func() hash.Hash) []byte {
	var (
		hash       = hashFunc()
		digestSize = hash.Size()
		blocks     = int(math.Floor(float64(len(block)) / float64(digestSize)))
		padding    = len(block) % digestSize
		diffused   []byte
	)

	// Hash full blocks
	for i := 0; i < blocks; i++ {
		offset := i * digestSize
		hash.Reset()
		hash.Write(packInt(i))
		hash.Write(block[offset : offset+digestSize])
		diffused = append(diffused, hash.Sum(nil)...)
	}

	// Hash remainder
	if padding > 0 {
		hash.Reset()
		hash.Write(packInt(blocks))
		hash.Write(block[blocks*digestSize:])
		diffused = append(diffused, hash.Sum(nil)[:padding]...)
	}

	return diffused
}

func packInt(i int) []byte {
	var packed [4]byte
	binary.BigEndian.PutUint32(packed[:], uint32(i))
	return packed[:]
}
