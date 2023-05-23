// nolint:goheader

// This file is part of nullboot
// Copyright 2021 Canonical Ltd.
// SPDX-License-Identifier: GPL-3.0-only

package efi

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"

	efi "github.com/canonical/go-efilib"
	efi_linux "github.com/canonical/go-efilib/linux"
	"github.com/twpayne/go-vfs"
)

// Variables abstracts away the host-specific bits of the efivars module
type Variables interface {
	ListVariables() ([]efi.VariableDescriptor, error)
	GetVariable(guid efi.GUID, name string) (data []byte, attrs efi.VariableAttributes, err error)
	SetVariable(guid efi.GUID, name string, data []byte, attrs efi.VariableAttributes) error
	NewFileDevicePath(filepath string, mode efi_linux.FileDevicePathMode) (efi.DevicePath, error)
	DelVariable(guid efi.GUID, name string) error
}

// RealEFIVariables provides the real implementation of efivars
type RealEFIVariables struct{}

func (v RealEFIVariables) DelVariable(guid efi.GUID, name string) error {
	_, attrs, err := v.GetVariable(guid, name)
	if err != nil {
		return err
	}
	return v.SetVariable(guid, name, nil, attrs)
}

func (v RealEFIVariables) NewFileDevicePath(filepath string, mode efi_linux.FileDevicePathMode) (efi.DevicePath, error) {
	return efi_linux.NewFileDevicePath(filepath, mode)
}

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

type mockEFIVariable struct {
	data  []byte
	attrs efi.VariableAttributes
}

// MockEFIVariables implements an in-memory variable store.
type MockEFIVariables struct {
	store map[efi.VariableDescriptor]mockEFIVariable
}

func (m MockEFIVariables) DelVariable(_ efi.GUID, _ string) error {
	return nil
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

func (m MockEFIVariables) NewFileDevicePath(fpath string, _ efi_linux.FileDevicePathMode) (efi.DevicePath, error) {
	file, err := vfs.OSFS.Open(fpath)
	if err != nil {
		return nil, err
	}
	file.Close()

	const espLocation = "/boot/efi/"
	fpath = strings.TrimPrefix(fpath, espLocation)

	return efi.DevicePath{
		efi.NewFilePathDevicePathNode(fpath),
	}, nil
}

// BootManager manages the boot device selection menu entries (Boot0000...BootFFFF).
type BootManager struct {
	efivars        Variables                 // EFIVariables implementation
	entries        map[int]BootEntryVariable // The Boot<number> variables
	bootOrder      []int                     // The BootOrder variable, parsed
	bootOrderAttrs efi.VariableAttributes    // The attributes of BootOrder variable
}

// BootEntryVariable defines a boot entry variable
type BootEntryVariable struct {
	BootNumber int                    // number of the Boot variable, for example, for Boot0004 this is 4
	Data       []byte                 // the data of the variable
	Attributes efi.VariableAttributes // any attributes set on the variable
	LoadOption *efi.LoadOption        // the data of the variable parsed as a load option, if it is a valid load option
}

// BootEntry is a boot entry.
type BootEntry struct {
	Filename    string
	Label       string
	Options     string
	Description string
}
