// This file is part of nullboot
// Copyright 2021 Canonical Ltd.
// SPDX-License-Identifier: GPL-3.0-only

package efibootmgr

import (
	"bytes"
	"crypto"
	_ "crypto/sha256" // ensure that sha256 is linked in
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	hashBlockSize     = 4096
	trustedAssetsPath = "/var/lib/nullboot/assets"
)

func computeRootHash(alg crypto.Hash, hashes [][]byte) []byte {
	if len(hashes) == 0 {
		panic("no hashes supplied!")
	}

	for len(hashes) != 1 {
		// Loop until we get to a single hash
		var next [][]byte

		for len(hashes) > 0 {
			// Loop whilst we still have hashes
			var block [hashBlockSize]byte
			for i := 0; hashBlockSize-i >= alg.Size() && len(hashes) > 0; i += alg.Size() {
				// Loop until we've filled a block or run out of hashes.
				copy(block[i:], hashes[0])
				hashes = hashes[1:]
			}

			// Hash the current block and save it for the next
			// outer loop iteration.
			h := alg.New()
			h.Write(block[:])
			next = append(next, h.Sum(nil))
		}

		// Process the hashes created on this loop iteration
		hashes = next
	}

	return hashes[0]
}

type hashAlg struct {
	crypto.Hash
}

func (a hashAlg) MarshalJSON() ([]byte, error) {
	var s string

	switch a.Hash {
	case crypto.SHA256:
		s = "sha256"
	//case crypto.SHA384:
	//	s = "sha384"
	//case crypto.SHA512:
	//	s = "sha512"
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %v", a.Hash)
	}

	return json.Marshal(s)
}

func (a *hashAlg) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch s {
	case "sha256":
		a.Hash = crypto.SHA256
	//case "sha384":
	//	a.Hash = crypto.SHA384
	//case "sha512":
	//	a.Hash = crypto.SHA512
	default:
		return fmt.Errorf("unsupported hash algorithm: %s", s)
	}

	return nil
}

type loadedTrustedAssets struct {
	Alg    hashAlg  `json:"alg"`
	Hashes [][]byte `json:"hashes"`
}

// TrustedAssets keeps a record of boot asset hashes that are trusted for the
// purpose of computing PCR profiles. New hashes are added by adding a directory
// that is trusted using TrustNewFromDir - the directory will be one inside the
// encrypted container, writable by root and managed by the package manager).
//
// Boot assets may be copied outside of a trusted directory, eg, to the ESP.
// Assets loaded from outside of a trusted directory in order to compute a PCR
// profile must be checked against one of the trusted hashes, to avoid tricking
// the resealing code into adding a malicious asset to a PCR profile.
//
// Snapd achieves the same thing by keeping a cache of trusted assets inside
// the encrypted data partition. This avoids having to do that.
//
// Note that the hashes are not constructed by hashing the file contents in
// a single pass, as this would require keeping entire PE images in memory
// after verifying their hashes in order to avoid TOCTOU type bugs. Files
// are hashed by producing a hash tree with a 4k block size. If a file's
// size is not a multiple of 4k, the last block is padded with zeros.
//
// The hash tree is not stored anywhere - only the root hash is stored.
// In order to verify that a file's contents are trusted, the leaf hashes
// are constructed when the file is read and then closed (see hashedFile)
// and the rest of the hash tree is reconstructed by calling checkLeafHashes.
// In order to verify any blocks, the entire file has to eventually be read
// in order to reconstruct the entire hash tree. This is a tradeoff between
// having to read an entire file in order to verify a few blocks, and not
// having to read an entire file in order to verify a few blocks, but having
// to store the entire hash tree somewhere.
//
// Use newCheckedHashedFile to have a file checked against the set of trusted
// boot assets.
type TrustedAssets struct {
	loaded    loadedTrustedAssets
	newAssets [][]byte
}

func (t *TrustedAssets) alg() crypto.Hash {
	return t.loaded.Alg.Hash
}

func (t *TrustedAssets) checkLeafHashes(hashes [][]byte) bool {
	d := computeRootHash(t.alg(), hashes)
	for _, a := range t.loaded.Hashes {
		if bytes.Equal(d, a) {
			return true
		}
	}
	return false
}

func (t *TrustedAssets) maybeAddHash(d []byte) {
	for _, a := range t.loaded.Hashes {
		if bytes.Equal(d, a) {
			return
		}
	}

	t.loaded.Hashes = append(t.loaded.Hashes, d)
}

func (t *TrustedAssets) trustLeafHashes(hashes [][]byte) {
	d := computeRootHash(t.alg(), hashes)
	t.maybeAddHash(d)
	t.newAssets = append(t.newAssets, d)
}

func (t *TrustedAssets) trustFile(path string) error {
	f, err := appFs.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var hashes [][]byte

	h := t.alg().New()
	for {
		var block [hashBlockSize]byte
		_, err := io.ReadFull(f, block[:])
		if err == io.EOF {
			break
		}
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return err
		}

		h.Reset()
		h.Write(block[:])
		hashes = append(hashes, h.Sum(nil))

		if err != nil {
			break
		}
	}

	t.trustLeafHashes(hashes)
	return nil
}

func (t *TrustedAssets) trustDir(path string) error {
	dirents, err := appFs.ReadDir(path)
	if err != nil {
		return err
	}

	for _, e := range dirents {
		p := filepath.Join(path, e.Name())
		if err := t.trustPath(p); err != nil {
			return fmt.Errorf("cannot process path %s: %w", p, err)
		}
	}

	return nil
}

func (t *TrustedAssets) trustPath(path string) error {
	fi, err := appFs.Stat(path)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		return t.trustDir(path)
	}
	return t.trustFile(path)
}

// TrustNewFromDir adds hashes of the files under the specified path to the list
// of trusted hashes for the purpose of computing PCR profiles. The path should
// be within the encrypted container, writable only by root and managed by the
// package manager.
func (t *TrustedAssets) TrustNewFromDir(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("path is not absolute")
	}
	return t.trustDir(filepath.Clean(path))
}

// RemoveObsolete drops all asset hashes that haven't been added in this context
// via a call to TrustNewFromDir. This should be called after newly trusted assets
// have been properly committed and obsolete assets have been removed.
func (t *TrustedAssets) RemoveObsolete() {
	t.loaded.Hashes = nil
	for _, d := range t.newAssets {
		t.maybeAddHash(d)
	}
}

// Save persists the list of trusted hashes to disk.
func (t *TrustedAssets) Save() (err error) {
	if err := appFs.MkdirAll(filepath.Dir(trustedAssetsPath), 0600); err != nil {
		return fmt.Errorf("cannot make directory: %v", err)
	}

	f, err := appFs.TempFile(filepath.Dir(trustedAssetsPath), "."+filepath.Base(trustedAssetsPath)+".")
	if err != nil {
		return err
	}
	defer func() {
		name := f.Name()
		f.Close()
		if err == nil {
			return
		}
		os.Remove(name)
	}()

	if err := json.NewEncoder(f).Encode(t.loaded); err != nil {
		return err
	}

	return appFs.Rename(f.Name(), trustedAssetsPath)
}

func newTrustedAssets() *TrustedAssets {
	return &TrustedAssets{loaded: loadedTrustedAssets{Alg: hashAlg{Hash: crypto.SHA256}}}
}

// ReadTrustedAssets loads the list of previously trusted hashes from
// disk.
func ReadTrustedAssets() (*TrustedAssets, error) {
	f, err := appFs.Open(trustedAssetsPath)
	switch {
	case os.IsNotExist(err):
		// Ignore this.
		return newTrustedAssets(), nil
	case err != nil:
		return nil, err
	}
	defer f.Close()

	assets := new(TrustedAssets)
	if err := json.NewDecoder(f).Decode(&assets.loaded); err != nil {
		return nil, err
	}
	if !assets.loaded.Alg.Available() {
		return nil, fmt.Errorf("digest algorithm %v is not available", assets.loaded.Alg)
	}

	return assets, nil
}

// newCheckedHashedFile wraps a file handle and calls the supplied
// closeNotify callback when the file is closed with an indication
// as to whether the file's contents are included in the supplied set
// of trusted boot assets
func newCheckedHashedFile(f File, assets *TrustedAssets, closeNotify func(bool)) (*hashedFile, error) {
	return newHashedFile(f, assets.alg(), func(leafHashes [][]byte) {
		closeNotify(assets.checkLeafHashes(leafHashes))
	})
}
