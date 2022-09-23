// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package util

import (
	"errors"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"
)

// ComputeQualifiedName can compute the qualified name of an object with
// the specified name that is protected by a parent with the specified
// qualified name.
func ComputeQualifiedName(name, parentQn tpm2.Name) (tpm2.Name, error) {
	if name.Algorithm() == tpm2.HashAlgorithmNull {
		return nil, errors.New("invalid name")
	}
	if !name.Algorithm().Available() {
		return nil, errors.New("name algorithm is not available")
	}

	h := name.Algorithm().NewHash()
	h.Write(parentQn)
	h.Write(name)

	return mu.MarshalToBytes(name.Algorithm(), mu.RawBytes(h.Sum(nil)))
}

// ComputeQualifiedNameFull can compute the qualified name of an object with
// the specified name that is protected in the specified hierarchy by the chain
// of parent objects with the specified names. The ancestor names are ordered
// from the primary key towards the immediate parent.
func ComputeQualifiedNameFull(name tpm2.Name, hierarchy tpm2.Handle, ancestors ...tpm2.Name) (tpm2.Name, error) {
	lastQn := tpm2.Name(mu.MustMarshalToBytes(hierarchy))

	for len(ancestors) > 0 {
		current := ancestors[0]
		ancestors = ancestors[1:]

		var err error
		lastQn, err = ComputeQualifiedName(current, lastQn)
		if err != nil {
			return nil, err
		}
	}

	return ComputeQualifiedName(name, lastQn)
}
