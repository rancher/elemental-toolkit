// Copyright 2024 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
)

func readBinaryVariable(ctx context.Context, name string, guid GUID) (bool, error) {
	data, _, err := ReadVariable(ctx, name, guid)
	if err != nil {
		return false, err
	}
	if len(data) != 1 {
		return false, fmt.Errorf("variable contents has unexpected number of bytes (got %d bytes)", len(data))
	}
	switch data[0] {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("unexpected variable contents: %d", uint8(data[0]))
	}
}

// ReadSecureBootVariable reads the SecureBoot global variable which provides
// an indication of whether secure boot is enabled. If it returns false, then
// secure boot is disabled. If it returns true, then it is an indication that
// secure boot is enabled. In general, [DefaultVarContext] should be supplied
// to this.
//
// Note that this function cannot prove that secure boot is enabled. If a platform
// provides a way to disable secure boot and execute arbitrary code, then the
// platform or kernel may not tell the truth about this. Obtaining proof that
// secure boot is enabled would involve the use of attestations and a third
// party verifier.
func ReadSecureBootVariable(ctx context.Context) (bool, error) {
	return readBinaryVariable(ctx, "SecureBoot", GlobalVariable)
}

// ReadPlatformKeyCertificate reads the PK global variable and returns the corresponding
// certificate, if a platform key is enrolled. If no platform key is enrolled, this
// will return nil. In general, [DefaultVarContext] should be supplied to this.
func ReadPlatformKeyCertificate(ctx context.Context) (*x509.Certificate, error) {
	db, err := ReadSignatureDatabaseVariable(ctx, PKVariable)
	if err != nil {
		return nil, err
	}

	switch len(db) {
	case 0:
		return nil, nil
	case 1:
		esl := db[0]
		if esl.Type != CertX509Guid {
			return nil, fmt.Errorf("invalid PK contents: unexpected signature list type: %v", esl.Type)
		}
		if len(esl.Signatures) != 1 {
			return nil, fmt.Errorf("invalid PK contents: unexpected number of signature data entries (%d)", len(esl.Signatures))
		}
		return x509.ParseCertificate(esl.Signatures[0].Data)
	default:
		return nil, errors.New("invalid PK contents: more than one signature list")
	}
}

var (
	// PKVariable corresponds to the PK global variable
	PKVariable VariableDescriptor = VariableDescriptor{Name: "PK", GUID: GlobalVariable}

	// KEKVariable corresponds to the KEK global variable
	KEKVariable VariableDescriptor = VariableDescriptor{Name: "KEK", GUID: GlobalVariable}

	// DbVariable corresponds to the authorized signature database variable
	DbVariable VariableDescriptor = VariableDescriptor{Name: "db", GUID: ImageSecurityDatabaseGuid}

	// DbxVariable corresponds to the forbidden signature database variable
	DbxVariable VariableDescriptor = VariableDescriptor{Name: "dbx", GUID: ImageSecurityDatabaseGuid}
)

// ReadSignatureDatabaseVariable reads the signature database from the supplied
// variable. In general, [DefaultVarContext] should be supplied to this.
func ReadSignatureDatabaseVariable(ctx context.Context, desc VariableDescriptor) (SignatureDatabase, error) {
	data, _, err := ReadVariable(ctx, desc.Name, desc.GUID)
	if err != nil {
		return nil, err
	}
	return ReadSignatureDatabase(bytes.NewReader(data))
}

// InconsistentSecureBootModeError is returned from [ComputeSecureBootMode] if
// some of the variables are in an inconsistent state.
type InconsistentSecureBootModeError struct {
	err error
}

func (e *InconsistentSecureBootModeError) Error() string {
	return "inconsistent secure boot mode: " + e.err.Error()
}

func (e *InconsistentSecureBootModeError) Unwrap() error {
	return e.err
}

// SecureBootMode describes the secure boot mode of a platform.
type SecureBootMode int

const (
	// SetupMode indicates that a platform is in setup mode. In this mode, no platform
	// key is enrolled and secure boot cannot be enabled. Writes to secure boot
	// variables other than PK can be performed without authentication.
	//
	// SetupMode can transition to UserMode by enrolling a platform key, which can be
	// done from the OS by performing a self-signed authenticated write to the PK
	// global variable.
	//
	// Since UEFI 2.5, SetupMode can transition to AuditMode by writing 1 to the
	// AuditMode global variable before ExitBootServices.
	SetupMode SecureBootMode = iota + 1

	// AuditMode indicates that a platform is in audit mode. This mode implies setup
	// mode - no platform key is enrolled and secure boot cannot be enabled. Writes to
	// secure boot variables other than PK can be performed without authentication.
	//
	// AuditMode provides a way of ensuring that the current signature database
	// configuration is able to authenticate an OS without preventing it from booting
	// if authentication fails.
	//
	// AuditMode can transition to DeployedMode by enrolling a platform key, which can be
	// done from the OS by performing a self-signed authenticated write to the PK
	// global variable.
	//
	// AuditMode only exists since UEFI 2.5.
	AuditMode

	// UserMode indicates that a platform is in user mode. In this mode, a platform
	// key is enrolled and secure boot can be enabled (but may be disabled using some
	// platform specific mechanism). Writes to secure boot variables require authentication.
	//
	// UserMode can transition to SetupMode by erasing the platform key, either via
	// some platform specific mechanism or by an authenticated write of an empty payload
	// to the PK global variable.
	//
	// Since UEFI 2.5, UserMode can transition to AuditMode by writing 1 to the AuditMode
	// global variable before ExitBootServices.
	//
	// Since UEFI 2.5, UserMode can transition to DeployedMode by writing 1 to the
	// DeployedMode global variable before ExitBootServices.
	UserMode

	// DeployedMode indicates that a platform is in deployed mode. In this mode, a
	// platform key is enrolled and secure boot can be enabled (but may be disabled using
	// some platform specific mechanism. Writes to secure boot variables require
	// authentication. This is the most secure mode.
	//
	// DeployedMode may transition back to UserMode by some optional platform specific
	// mechanism which clears the DeployedMode variable.
	//
	// DeployedMode exists since UEFI 2.5.
	DeployedMode
)

type secureBootModeFeatures int

const (
	secureBootModeFeaturesUndetermined secureBootModeFeatures = iota
	secureBootModeFeaturesAtLeastUefi2_5
	secureBootModeFeaturesBeforeUefi2_5
)

// ComputeSecureBootMode determines the secure boot mode of a platform. In general,
// [DefaultVarContext] should be supplied to this.
func ComputeSecureBootMode(ctx context.Context) (SecureBootMode, error) {
	setupMode, err := readBinaryVariable(ctx, "SetupMode", GlobalVariable)
	if err != nil {
		return 0, fmt.Errorf("cannot read SetupMode variable: %w", err)
	}

	var features secureBootModeFeatures

	auditMode, err := readBinaryVariable(ctx, "AuditMode", GlobalVariable)
	switch {
	case errors.Is(err, ErrVarNotExist):
		features = secureBootModeFeaturesBeforeUefi2_5
	case err != nil:
		return 0, fmt.Errorf("cannot read AuditMode variable: %w", err)
	default:
		features = secureBootModeFeaturesAtLeastUefi2_5
	}

	var deployedMode bool
	if features == secureBootModeFeaturesAtLeastUefi2_5 {
		var err error
		deployedMode, err = readBinaryVariable(ctx, "DeployedMode", GlobalVariable)
		if err != nil {
			return 0, fmt.Errorf("cannot read DeployedMode variable: %w", err)
		}
	}

	secureBoot, err := ReadSecureBootVariable(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot read SecureBoot variable: %w", err)
	}
	pk, err := ReadPlatformKeyCertificate(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot read PK variable: %w", err)
	}

	switch setupMode {
	case true:
		if secureBoot {
			// Secure boot cannot be enabled in setup mode
			return 0, &InconsistentSecureBootModeError{errors.New("firmware indicates secure boot is enabled in setup mode")}
		}
		if pk != nil {
			// There should be no platform key in setup mode. If one is enrolled from the OS,
			// the firmware should update the value of SetupMode.
			return 0, &InconsistentSecureBootModeError{errors.New("firmware indicates setup mode is enabled with a platform key enrolled")}
		}
		if features == secureBootModeFeaturesBeforeUefi2_5 {
			return SetupMode, nil
		}
		if deployedMode {
			// Deployed mode cannot be enabled in setup mode.
			return 0, &InconsistentSecureBootModeError{errors.New("firmware indicates deployed mode is enabled in setup mode")}
		}
		if auditMode {
			return AuditMode, nil
		}
		return SetupMode, nil
	case false:
		if pk == nil {
			// There should be a platform key when not in setup mode. If it is deleted with
			// an authenticated write from the OS, then the firmware should update the value
			// of SetupMode.
			return 0, &InconsistentSecureBootModeError{errors.New("firmware indicates it isn't in setup mode when no platform key is enrolled")}
		}
		if features == secureBootModeFeaturesBeforeUefi2_5 {
			return UserMode, nil
		}
		if auditMode {
			// Audit mode implies setup mode.
			return 0, &InconsistentSecureBootModeError{errors.New("firmware indicates audit mode is enabled when not in setup mode")}
		}
		if deployedMode {
			return DeployedMode, nil
		}
		return UserMode, nil
	}

	panic("not reached")
}

// IsDeployedModeSupported indicates whether the firmware is new enough (ie based on
// at least UEFI 2.5) to support deployed mode. In general, [DefaultVarContext] should
// be supplied to this.
func IsDeployedModeSupported(ctx context.Context) bool {
	_, _, err := ReadVariable(ctx, "DeployedMode", GlobalVariable)
	return err == nil
}
