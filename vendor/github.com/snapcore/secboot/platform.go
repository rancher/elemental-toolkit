// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2021 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package secboot

// PlatformKeyRecoveryErrorType describes the type of error returned from one of
// the PlatformKeyDataHandler.RecoverKeys* functions.
type PlatformKeyRecoveryErrorType int

const (
	// PlatformKeyRecoveryErrorInvalidData indicates that keys could not be
	// recovered successfully because the supplied data is invalid.
	PlatformKeyRecoveryErrorInvalidData PlatformKeyRecoveryErrorType = iota + 1

	// PlatformKeyRecoveryErrorUninitialized indicates that keys could not be
	// recovered successfully because the platform's secure device is not properly
	// initialized.
	PlatformKeyRecoveryErrorUninitialized

	// PlatformKeyRecoveryErrorUnavailable indicates that keys could not be
	// recovered successfully because the platform's secure device is unavailable.
	PlatformKeyRecoveryErrorUnavailable
)

// PlatformKeyRecoveryError is returned from any of the PlatformKeyDataHandler.RecoverKeys*
// functions if the keys cannot be successfully recovered by the platform's secure device.
type PlatformKeyRecoveryError struct {
	Type PlatformKeyRecoveryErrorType // type of the error
	Err  error                        // underlying error
}

func (e *PlatformKeyRecoveryError) Error() string {
	return e.Err.Error()
}

func (e *PlatformKeyRecoveryError) Unwrap() error {
	return e.Err
}

// PlatformKeyData represents the data exchanged between this package and
// platform implementations.
type PlatformKeyData struct {
	// Handle contains metadata required by the platform in order to recover
	// this key. It is opaque to this go package. It should be an encoded JSON
	// value, which could be something as simple as a single string containing
	// a base64 encoded binary payload or a more complex JSON object, depending
	// on the requirements of the implementation.
	Handle []byte

	EncryptedPayload []byte // The encrypted payload
}

// PlatormKeyDataHandler is the interface that this go package uses to
// interact with a platform's secure device for the purpose of recovering keys.
type PlatformKeyDataHandler interface {
	// RecoverKeys attempts to recover the cleartext keys from the supplied key
	// data using this platform's secure device.
	RecoverKeys(data *PlatformKeyData) (KeyPayload, error)

	// RecoverKeysWithAuthValue attempts to recover the cleartext keys from the
	// supplied data using this platform's secure device. The authValue parameter
	// is a passphrase derived key to enable passphrase support to be integrated
	// with the secure device. The platform implementation doesn't provide the primary
	// mechanism of protecting keys with a passphrase - this is done in the platform
	// agnostic API. Some devices (such as TPMs) support this integration natively. For
	// other devices, the integration should provide a way of validating the authValue in
	// a way that requires the use of the secure device (eg, such as computing a HMAC of
	// it using a hardware backed key).
	// RecoverKeysWithAuthValue(data *PlatformKeyData, authValue []byte) (KeyPayload, error)

	// ChangeAuthValue is called to notify the platform implementation that the
	// passphrase is being changed. The oldAuthValue and newAuthValue parameters
	// are passphrase derived keys. On success, it should return an updated
	// PlatformKeyData.
	// ChangeAuthValue(data *PlatformKeyData, oldAuthValue, newAuthValue []byte) (*PlatformKeyData, error)
}

var handlers = make(map[string]PlatformKeyDataHandler)

// RegisterPlatformKeyDataHandler registers a handler for the specified platform name.
func RegisterPlatformKeyDataHandler(name string, handler PlatformKeyDataHandler) {
	handlers[name] = handler
}
