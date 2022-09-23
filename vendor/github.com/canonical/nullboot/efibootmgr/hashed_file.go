// This file is part of nullboot
// Copyright 2021 Canonical Ltd.
// SPDX-License-Identifier: GPL-3.0-only

package efibootmgr

import (
	"bytes"
	"crypto"
	"errors"
	"fmt"
	"io"
)

// hashedFile wraps a file handle and builds a list of per-block hashes for
// each block of data read from the file.
//
// During read operations, hashes are computed on a block-by-block basis. If a
// block's hash hasn't previously been computed, then it is recorded. If it
// has previously been computed, then the hash is compared against the previously
// recorded one and an error returns if it is different.
//
// During close, any previously unread blocks are read in order to compute their
// hashes. A notify function is then called with the list of hashes for external
// code to verify. This allows verification to be performed without the risk of
// TOCTOU type bugs and without having to read and keep the entire file in
// memory whilst it is being used.
type hashedFile struct {
	file File
	sz   int64

	alg              crypto.Hash
	closeNotify      func([][]byte)
	leafHashes       [][]byte
	cachedBlockIndex int64
	cachedBlock      []byte
}

// newHashedFile creates a new hashedFile from the supplied file handle. When the
// file is closed, the supplied closeNotify callback will be called with a list
// of per-block hashes from the file.
func newHashedFile(f File, alg crypto.Hash, closeNotify func([][]byte)) (*hashedFile, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return &hashedFile{
		file:             f,
		sz:               info.Size(),
		alg:              alg,
		closeNotify:      closeNotify,
		leafHashes:       make([][]byte, (info.Size()+(hashBlockSize-1))/hashBlockSize),
		cachedBlockIndex: -1}, nil
}

func (f *hashedFile) readAndCacheBlock(i int64) error {
	if i == f.cachedBlockIndex {
		// Reading from the cached block
		return nil
	}

	if i >= int64(len(f.leafHashes)) {
		// Huh, out of range
		return io.EOF
	}

	// Read the whole block
	r := io.NewSectionReader(f.file, i*hashBlockSize, hashBlockSize)

	var block [hashBlockSize]byte
	n, err := io.ReadFull(r, block[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		// Handle io.ErrUnexpectedEOF later.
		return err
	}

	// Cache this block to speed up small reads
	f.cachedBlockIndex = i
	f.cachedBlock = block[:n]

	// Hash the block
	h := f.alg.New()
	h.Write(block[:])

	if len(f.leafHashes[i]) == 0 {
		// This is the first time we read this block.
		f.leafHashes[i] = h.Sum(nil)
	} else if !bytes.Equal(h.Sum(nil), f.leafHashes[i]) {
		// We've read this block before, and it has changed.
		return fmt.Errorf("hash check fail for block %d", i)
	}

	return err
}

func (f *hashedFile) ReadAt(p []byte, off int64) (n int, err error) {
	// Calculate the starting block and number of blocks.
	start := ((off + hashBlockSize) / hashBlockSize) - 1
	end := ((off + int64(len(p)) + hashBlockSize) / hashBlockSize)
	num := end - start

	// Read and hash each block.
	for i := start; i < start+num; i++ {
		if err := f.readAndCacheBlock(i); err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}

		data := f.cachedBlock
		if n == 0 {
			off0 := off - (start * hashBlockSize)
			data = data[off0:]
		}
		sz := len(p) - n
		if sz < len(data) {
			data = data[:sz]
		}

		copy(p[n:], data)
		n += len(data)

		if err != nil {
			break
		}
	}

	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (f *hashedFile) Close() error {
	// Loop over missing leaf hashes.
	for i, d := range f.leafHashes {
		if len(d) > 0 {
			continue
		}

		// Hash missing block.
		if err := f.readAndCacheBlock(int64(i)); err != nil {
			break
		}
	}

	if f.closeNotify != nil {
		f.closeNotify(f.leafHashes)
	}

	return f.file.Close()
}

func (f *hashedFile) Size() int64 {
	return f.sz
}
