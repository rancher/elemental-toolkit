// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

// Section 16 - Random Number Generator

// GetRandom executes the TPM2_GetRandom command to return the next bytesRequested number of bytes from the TPM's
// random number generator. If the requested bytes cannot be read in a single command, this function will reexecute
// the TPM2_GetRandom command until all requested bytes have been read.
func (t *TPMContext) GetRandom(bytesRequested uint16, sessions ...SessionContext) (randomBytes []byte, err error) {
	if err := t.initPropertiesIfNeeded(); err != nil {
		return nil, err
	}

	randomBytes = make([]byte, bytesRequested)

	total := 0
	remaining := bytesRequested

	for {
		sz := remaining
		if sz > uint16(t.maxDigestSize) {
			sz = uint16(t.maxDigestSize)
		}

		var tmpBytes Digest
		if err := t.RunCommand(CommandGetRandom, sessions,
			Delimiter,
			sz, Delimiter,
			Delimiter,
			&tmpBytes); err != nil {
			return nil, err
		}

		copy(randomBytes[total:], tmpBytes)
		total += int(sz)
		remaining -= sz

		if remaining == 0 {
			break
		}
	}

	return randomBytes, nil
}

func (t *TPMContext) StirRandom(inData SensitiveData, sessions ...SessionContext) error {
	return t.RunCommand(CommandStirRandom, sessions, Delimiter, inData)
}
