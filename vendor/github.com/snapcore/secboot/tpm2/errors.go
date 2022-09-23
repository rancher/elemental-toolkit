// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
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

package tpm2

import (
	"errors"
	"fmt"

	"github.com/canonical/go-tpm2"

	"golang.org/x/xerrors"
)

var (
	// ErrTPMClearRequiresPPI is returned from Connection.EnsureProvisioned and indicates that clearing the TPM must be performed via
	// the Physical Presence Interface.
	ErrTPMClearRequiresPPI = errors.New("clearing the TPM requires the use of the Physical Presence Interface")

	// ErrTPMProvisioningRequiresLockout is returned from Connection.EnsureProvisioned when fully provisioning the TPM requires
	// the use of the lockout hierarchy. In this case, the provisioning steps that can be performed without the use of the lockout
	// hierarchy are completed.
	ErrTPMProvisioningRequiresLockout = errors.New("provisioning the TPM requires the use of the lockout hierarchy")

	// ErrTPMProvisioning indicates that the TPM is not provisioned correctly for the requested operation. Please note that other errors
	// that can be returned may also be caused by incomplete provisioning, as it is not always possible to detect incomplete or
	// incorrect provisioning in all contexts.
	ErrTPMProvisioning = errors.New("the TPM is not correctly provisioned")

	// ErrTPMLockout is returned from any function when the TPM is in dictionary-attack lockout mode. Until
	// the TPM exits lockout mode, the key will need to be recovered via a mechanism that is independent of
	// the TPM (eg, a recovery key)
	ErrTPMLockout = errors.New("the TPM is in DA lockout mode")

	// ErrNoTPM2Device is returned from ConnectToDefaultTPM or SecureConnectToDefaultTPM if no TPM2 device is avaiable.
	ErrNoTPM2Device = errors.New("no TPM2 device is available")
)

// TPMResourceExistsError is returned from any function that creates a persistent TPM resource if a resource already exists
// at the specified handle.
type TPMResourceExistsError struct {
	Handle tpm2.Handle
}

func (e TPMResourceExistsError) Error() string {
	return fmt.Sprintf("a resource already exists on the TPM at handle %v", e.Handle)
}

// AuthFailError is returned when an authorization check fails. The provided handle indicates the resource for which authorization
// failed. Whilst the error normally indicates that the provided authorization value is incorrect, it may also be returned
// for other reasons that would cause a HMAC check failure, such as a communication failure between the host CPU and the TPM
// or the name of a resource on the TPM not matching the name of the ResourceContext passed to the function that failed - this
// latter issue can occur when using a resource manager if another process accesses the TPM and makes changes to persistent
// resources or sessions.
type AuthFailError struct {
	Handle tpm2.Handle
}

func (e AuthFailError) Error() string {
	return fmt.Sprintf("cannot access resource at handle %v because an authorization check failed", e.Handle)
}

// EKCertVerificationError is returned from SecureConnectToDefaultTPM if verification of the EK certificate against the built-in
// root CA certificates fails, or the EK certificate does not have the correct properties, or the supplied certificate data cannot
// be unmarshalled correctly because it is invalid.
type EKCertVerificationError struct {
	msg string
}

func (e EKCertVerificationError) Error() string {
	return fmt.Sprintf("cannot verify the endorsement key certificate: %s", e.msg)
}

func isEKCertVerificationError(err error) bool {
	var e EKCertVerificationError
	return xerrors.As(err, &e)
}

// TPMVerificationError is returned from SecureConnectToDefaultTPM if the TPM cannot prove it is the device for which the verified
// EK certificate was issued.
type TPMVerificationError struct {
	msg string
}

func (e TPMVerificationError) Error() string {
	return fmt.Sprintf("cannot verify that the TPM is the device for which the supplied EK certificate was issued: %s", e.msg)
}

func isTPMVerificationError(err error) bool {
	var e TPMVerificationError
	return xerrors.As(err, &e)
}

// InvalidKeyDataError indicates that the provided key data file is invalid. This error may also be returned in some
// scenarious where the TPM is incorrectly provisioned, but it isn't possible to determine whether the error is with
// the provisioning status or because the key data file is invalid.
type InvalidKeyDataError struct {
	msg string
}

func (e InvalidKeyDataError) Error() string {
	return fmt.Sprintf("invalid key data: %s", e.msg)
}

func isInvalidKeyDataError(err error) bool {
	var e InvalidKeyDataError
	return xerrors.As(err, &e)
}
