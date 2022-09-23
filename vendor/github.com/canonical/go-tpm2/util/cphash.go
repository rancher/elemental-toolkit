// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package util

import (
	"errors"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/mu"
)

// ComputeCpHash computes a command parameter digest from the specified command code, the supplied
// handles (identified by their names) and parameters using the specified digest algorithm.
//
// The required parameters is defined in part 3 of the TPM 2.0 Library Specification for the
// specific command.
//
// The result of this is useful for extended authorization commands that bind an authorization to
// a command and set of command parameters, such as TPMContext.PolicySigned, TPMContext.PolicySecret,
// TPMContext.PolicyTicket and TPMContext.PolicyCpHash.
func ComputeCpHash(alg tpm2.HashAlgorithmId, command tpm2.CommandCode, handles []tpm2.Name, params ...interface{}) (tpm2.Digest, error) {
	if !alg.Available() {
		return nil, errors.New("algorithm is not available")
	}

	cpBytes, err := mu.MarshalToBytes(params...)
	if err != nil {
		return nil, err
	}

	return tpm2.ComputeCpHash(alg, command, handles, cpBytes), nil
}
