// Copyright 2020-2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"errors"

	"github.com/canonical/go-efilib/internal/uefi"
)

type VariableAttributes uint32

const (
	AttributeNonVolatile                       VariableAttributes = uefi.EFI_VARIABLE_NON_VOLATILE
	AttributeBootserviceAccess                 VariableAttributes = uefi.EFI_VARIABLE_BOOTSERVICE_ACCESS
	AttributeRuntimeAccess                     VariableAttributes = uefi.EFI_VARIABLE_RUNTIME_ACCESS
	AttributeHardwareErrorRecord               VariableAttributes = uefi.EFI_VARIABLE_HARDWARE_ERROR_RECORD
	AttributeAuthenticatedWriteAccess          VariableAttributes = uefi.EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS
	AttributeTimeBasedAuthenticatedWriteAccess VariableAttributes = uefi.EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS
	AttributeAppendWrite                       VariableAttributes = uefi.EFI_VARIABLE_APPEND_WRITE
	AttributeEnhancedAuthenticatedAccess       VariableAttributes = uefi.EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS
)

var (
	ErrVarsUnavailable = errors.New("no variable backend is available")
	ErrVarNotExist     = errors.New("variable does not exist")
	ErrVarPermission   = errors.New("permission denied")
)

// VariableDescriptor represents the identity of a variable.
type VariableDescriptor struct {
	Name string
	GUID GUID
}

type varsBackend interface {
	Get(name string, guid GUID) (VariableAttributes, []byte, error)
	Set(name string, guid GUID, attrs VariableAttributes, data []byte) error
	List() ([]VariableDescriptor, error)
}

type nullVarsBackend struct{}

func (v nullVarsBackend) Get(name string, guid GUID) (VariableAttributes, []byte, error) {
	return 0, nil, ErrVarsUnavailable
}

func (v nullVarsBackend) Set(name string, guid GUID, attrs VariableAttributes, data []byte) error {
	return ErrVarsUnavailable
}

func (v nullVarsBackend) List() ([]VariableDescriptor, error) {
	return nil, ErrVarsUnavailable
}

var vars varsBackend = nullVarsBackend{}

// ReadVariable returns the value and attributes of the EFI variable with the specified
// name and GUID.
func ReadVariable(name string, guid GUID) ([]byte, VariableAttributes, error) {
	attrs, data, err := vars.Get(name, guid)
	return data, attrs, err
}

// WriteVariable writes the supplied data value with the specified attributes to the
// EFI variable with the specified name and GUID.
//
// If the variable already exists, the specified attributes must match the existing
// attributes with the exception of AttributeAppendWrite.
//
// If the variable does not exist, it will be created.
func WriteVariable(name string, guid GUID, attrs VariableAttributes, data []byte) error {
	return vars.Set(name, guid, attrs, data)
}

// ListVariables returns a list of variables that can be accessed.
func ListVariables() ([]VariableDescriptor, error) {
	return vars.List()
}
