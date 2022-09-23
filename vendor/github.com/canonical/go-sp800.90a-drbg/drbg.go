// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

/*
Package drbg implements several DRBGs as recommended by NIST SP-800-90A (see
http://csrc.nist.gov/publications/nistpubs/800-90A/SP800-90A.pdf).

The hash, HMAC and block cipher mode DRBGs are implemented.

DRBG instances are automatically reseeded once the current seed period
expires.

All DRBGs are instantiated with the maximum security strength associated
with the requested configuration. The security strength cannot be specified
via the API.

DRBGs are instantiated by default using the platform's default entropy source
(via the crypto/rand package). This entropy source can be overridden, but it
must provide truly random data in order to achieve the selected security
strength.

Note that prediction resistance is not implemented. Prediction resistance
requires that the supplied entropy source is non-deterministic.
*/
package drbg

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"

	"golang.org/x/xerrors"
)

// ErrReseedRequired indicates that the DRBG must be reseeded before
// it can generate random bytes.
var ErrReseedRequired = errors.New("the DRGB must be reseeded")

var one = big.NewInt(1)

func twoExp(n uint) (out *big.Int) {
	d := make([]byte, n/8+1)
	d[0] = byte(1 << (n % 8))
	out = new(big.Int)
	out.SetBytes(d)
	return
}

func zeroExtendBytes(x *big.Int, l int) (out []byte) {
	out = make([]byte, l)
	tmp := x.Bytes()
	copy(out[len(out)-len(tmp):], tmp)
	return
}

func selectSecurityStrength(requested int) int {
	switch {
	case requested <= 14:
		return 14
	case requested <= 16:
		return 16
	case requested <= 24:
		return 24
	default:
		return 32
	}
}

type drbgImpl interface {
	instantiate(entropyInput, nonce, personalization []byte, securityStrength int)
	reseed(entropyInput, additionalInput []byte)
	generate(additionalInput, data []byte) error
}

// DRBG corresponds to an instantiated DRBG based on one of the mechanisms specified
// in SP-800-90A.
type DRBG struct {
	entropySource    io.Reader
	securityStrength int
	impl             drbgImpl
}

// instantiate implements the steps described in section 9.1 of SP800-90A.
func (d *DRBG) instantiate(personalization []byte, entropySource io.Reader, securityStrength int) error {
	// 3) If the length of the personalization_string > max_personalization_string_length,
	// return (ERROR_FLAG, Invalid).
	if int64(len(personalization)) > 1<<32 {
		return errors.New("personalization too large")
	}

	d.entropySource = rand.Reader
	if entropySource != nil {
		d.entropySource = entropySource
	}

	// 4) Set security_strength to the lowest security strength greater than or equal
	// to requested_instantiation_security_strength from the set {112, 128, 192, 256}
	d.securityStrength = selectSecurityStrength(securityStrength)

	// 6) (status, entropy_input) = Get_entropy_input (security_strength, min_length,
	// max_length, prediction_resistance_request).
	// 7) If (status ≠ SUCCESS), return (status, Invalid).
	entropyInput := make([]byte, securityStrength)
	if _, err := d.entropySource.Read(entropyInput); err != nil {
		return xerrors.Errorf("cannot get entropy: %w", err)
	}

	// 8) Obtain a nonce.
	nonce := make([]byte, securityStrength/2)
	if _, err := d.entropySource.Read(nonce); err != nil {
		return xerrors.Errorf("cannot get nonce: %w", err)
	}

	// 9) initial_working_state = Instantiate_algorithm (entropy_input, nonce,
	// personalization_string, security_strength).
	d.impl.instantiate(entropyInput, nonce, personalization, securityStrength)
	return nil
}

func (d *DRBG) instantiateWithExternalEntropy(entropyInput, nonce, personalization []byte, entropySource io.Reader, securityStrength int) error {
	if len(entropyInput) < securityStrength {
		return errors.New("entropyInput too small")
	}
	if int64(len(entropyInput)) > 1<<32 {
		return errors.New("entropyInput too large")
	}
	if int64(len(personalization)) > 1<<32 {
		return errors.New("personalization too large")
	}

	d.entropySource = entropySource
	d.securityStrength = selectSecurityStrength(securityStrength)
	d.impl.instantiate(entropyInput, nonce, personalization, securityStrength)
	return nil
}

// ReseedWithExternalEntropy will reseed the DRBG with the supplied entropy.
func (d *DRBG) ReseedWithExternalEntropy(entropyInput, additionalInput []byte) error {
	if int64(len(additionalInput)) > 1<<32 {
		return errors.New("additionalInput too large")
	}

	if len(entropyInput) < d.securityStrength {
		return errors.New("entropyInput too small")
	}
	if int64(len(entropyInput)) > 1<<32 {
		return errors.New("entropyInput too large")
	}

	d.impl.reseed(entropyInput, additionalInput)
	return nil
}

// Reseed will reseed the DRBG with additional entropy using the entropy source
// it was initialized with.
func (d *DRBG) Reseed(additionalInput []byte) error {
	// 3) If the length of the additional_input > max_additional_input_length,
	// return (ERROR_FLAG).
	if int64(len(additionalInput)) > 1<<32 {
		return errors.New("additionalInput too large")
	}

	if d.entropySource == nil {
		return errors.New("cannot reseed without external entropy")
	}

	// 4) (status, entropy_input) = Get_entropy_input (security_strength, min_length,
	// max_length, prediction_resistance_request).
	entropyInput := make([]byte, d.securityStrength)
	if _, err := d.entropySource.Read(entropyInput); err != nil {
		// 5) If (status ≠ SUCCESS), return (status).
		return xerrors.Errorf("cannot get entropy: %w", err)
	}

	// 6) new_working_state = Reseed_algorithm (working_state, entropy_input,
	// additional_input).
	d.impl.reseed(entropyInput, additionalInput)
	return nil
}

// Generate will fill the supplied data buffer with random bytes.
//
// If the DRBG needs to be reseeded before it can generate random bytes and it
// has been initialized with a source of entropy, the reseed operation will be
// performed automatically. If the DRBG hasn't been initialized with a source of
// entropy and it needs to be reseeded, ErrNeedsReseed will be returned.
//
// If the length of data is greater than 65536 bytes, an error will be returned.
func (d *DRBG) Generate(additionalInput, data []byte) error {
	// 2) If requested_number_of_bits > max_number_of_bits_per_request, then
	// return (ERROR_FLAG, Null).
	if len(data) > 65536 {
		return errors.New("too many bytes requested")
	}

	// 4) If the length of the additional_input > max_additional_input_length,
	// then return (ERROR_FLAG, Null).
	if int64(len(additionalInput)) > 1<<32 {
		return errors.New("additionalInput too large")
	}

	// 6) Clear the reseed_required_flag.
	reseedRequired := false

	for {
		// 7) If reseed_required_flag is set, or if prediction_resistance_request
		// is set, then
		if reseedRequired {
			// 7.1) status = Reseed_function (state_handle, prediction_resistance_request,
			// additional_input).
			if err := d.Reseed(additionalInput); err != nil {
				// 7.2) If (status ≠ SUCCESS), then return (status, Null).
				return xerrors.Errorf("cannot reseed: %w", err)
			}

			// 7.4) additional_input = the Null string.
			additionalInput = nil

			// 7.5) Clear the reseed_required_flag.
			reseedRequired = false
		}

		// 8) (status, pseudorandom_bits, new_working_state) = Generate_algorithm (
		// working_state, requested_number_of_bits, additional_input).
		err := d.impl.generate(additionalInput, data)
		switch {
		case err == ErrReseedRequired && d.entropySource != nil:
			// 9) If status indicates that a reseed is required before the requested bits
			// can be generated, then
			// 9.1) Set the reseed_required_flag.
			// 9.3) Go to step 7.
			reseedRequired = true
		case err == ErrReseedRequired:
			return err
		case err != nil:
			return xerrors.Errorf("cannot generate random data: %w", err)
		default:
			return nil
		}
	}
}

// Read will read len(data) random bytes in to data.
//
// If the DRBG needs to be reseeded in order to generate all of the random bytes
// and it has been initialized with a source of entropy, the reseed operation will
// be performed automatically. If the DRBG hasn't been initialized with a source of
// entropy and it needs to be reseeded, ErrNeedsReseed will be returned.
func (d *DRBG) Read(data []byte) (int, error) {
	total := 0

	for len(data) > 0 {
		b := data
		if len(data) > 65536 {
			b = data[:65536]
		}

		if err := d.Generate(nil, b); err != nil {
			return total, err
		}

		total += len(b)
		data = data[len(b):]
	}

	return total, nil
}
