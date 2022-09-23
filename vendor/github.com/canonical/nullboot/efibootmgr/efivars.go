// This file is part of nullboot
// Copyright 2021 Canonical Ltd.
// SPDX-License-Identifier: GPL-3.0-only

package efibootmgr

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"

	//"errors"
	"github.com/canonical/go-efilib"
	efi_linux "github.com/canonical/go-efilib/linux"
)

// EFIVariables abstracts away the host-specific bits of the efivars module
type EFIVariables interface {
	ListVariables() ([]efi.VariableDescriptor, error)
	GetVariable(guid efi.GUID, name string) (data []byte, attrs efi.VariableAttributes, err error)
	SetVariable(guid efi.GUID, name string, data []byte, attrs efi.VariableAttributes) error
	NewFileDevicePath(filepath string, mode efi_linux.FileDevicePathMode) (efi.DevicePath, error)
}

// RealEFIVariables provides the real implementation of efivars
type RealEFIVariables struct{}

// ListVariables proxy
func (RealEFIVariables) ListVariables() ([]efi.VariableDescriptor, error) {
	return efi.ListVariables()
}

// GetVariable proxy
func (RealEFIVariables) GetVariable(guid efi.GUID, name string) (data []byte, attrs efi.VariableAttributes, err error) {
	return efi.ReadVariable(name, guid)
}

// SetVariable proxy
func (RealEFIVariables) SetVariable(guid efi.GUID, name string, data []byte, attrs efi.VariableAttributes) error {
	return efi.WriteVariable(name, guid, attrs, data)
}

// NewFileDevicePath proxy
func (RealEFIVariables) NewFileDevicePath(filepath string, mode efi_linux.FileDevicePathMode) (efi.DevicePath, error) {
	return efi_linux.NewFileDevicePath(filepath, mode)
}

type mockEFIVariable struct {
	data  []byte
	attrs efi.VariableAttributes
}

// MockEFIVariables implements an in-memory variable store.
type MockEFIVariables struct {
	store map[efi.VariableDescriptor]mockEFIVariable
}

// ListVariables implements EFIVariables
func (m MockEFIVariables) ListVariables() (out []efi.VariableDescriptor, err error) {
	for k := range m.store {
		out = append(out, k)
	}
	return out, nil
}

// GetVariable implements EFIVariables
func (m MockEFIVariables) GetVariable(guid efi.GUID, name string) (data []byte, attrs efi.VariableAttributes, err error) {
	out, ok := m.store[efi.VariableDescriptor{Name: name, GUID: guid}]
	if !ok {
		return nil, 0, efi.ErrVarNotExist
	}
	return out.data, out.attrs, nil
}

// SetVariable implements EFIVariables
func (m *MockEFIVariables) SetVariable(guid efi.GUID, name string, data []byte, attrs efi.VariableAttributes) error {
	if m.store == nil {
		m.store = make(map[efi.VariableDescriptor]mockEFIVariable)
	}
	if len(data) == 0 {
		delete(m.store, efi.VariableDescriptor{Name: name, GUID: guid})
	} else {
		m.store[efi.VariableDescriptor{Name: name, GUID: guid}] = mockEFIVariable{data, attrs}
	}
	return nil
}

// NewFileDevicePath implements EFIVariables
func (m MockEFIVariables) NewFileDevicePath(filepath string, mode efi_linux.FileDevicePathMode) (efi.DevicePath, error) {
	file, err := appFs.Open(filepath)
	if err != nil {
		return nil, err
	}
	file.Close()

	const espLocation = "/boot/efi/"
	if strings.HasPrefix(filepath, espLocation) {
		filepath = filepath[len(espLocation):]
	}

	return efi.DevicePath{
		efi.NewFilePathDevicePathNode(filepath),
	}, nil
}

// JSON renders the MockEFIVariables as an Azure JSON config
func (m MockEFIVariables) JSON() ([]byte, error) {
	payload := make(map[string]map[string]string)

	var numBytes [2]byte
	for key, entry := range m.store {
		entryID := key.Name
		entryBase64 := base64.StdEncoding.EncodeToString(entry.data)
		guidBase64 := base64.StdEncoding.EncodeToString(key.GUID[0:])
		binary.LittleEndian.PutUint16(numBytes[0:], uint16(entry.attrs))
		entryAttrBase64 := base64.StdEncoding.EncodeToString(numBytes[0:])

		payload[entryID] = map[string]string{
			"guid":       guidBase64,
			"attributes": entryAttrBase64,
			"value":      entryBase64,
		}
	}

	return json.MarshalIndent(payload, "", "  ")
}

// VariablesSupported indicates whether variables can be accessed.
func VariablesSupported(efiVars EFIVariables) bool {
	_, err := efiVars.ListVariables()
	return err == nil
}

// GetVariableNames returns the names of every variable with the specified GUID.
func GetVariableNames(efiVars EFIVariables, filterGUID efi.GUID) (names []string, err error) {
	vars, err := efiVars.ListVariables()
	if err != nil {
		return nil, err
	}
	for _, entry := range vars {
		if entry.GUID != filterGUID {
			continue
		}
		names = append(names, entry.Name)
	}
	return names, nil
}

// DelVariable deletes the non-authenticated variable with the specified name.
func DelVariable(efivars EFIVariables, guid efi.GUID, name string) error {
	_, attrs, err := efivars.GetVariable(guid, name)
	if err != nil {
		return err
	}
	// XXX: Update tests to not set these attributes in mock variables
	//if attrs&(efi.AttributeAuthenticatedWriteAccess|efi.AttributeTimeBasedAuthenticatedWriteAccess|efi.AttributeEnhancedAuthenticatedAccess) != 0 {
	//	return errors.New("variable must be deleted by setting an authenticated empty payload")
	//}
	return efivars.SetVariable(guid, name, nil, attrs)
}
