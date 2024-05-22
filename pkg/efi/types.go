// nolint:goheader

// This file is part of nullboot
// Copyright 2021 Canonical Ltd.
// SPDX-License-Identifier: GPL-3.0-only

package efi

import (
	"io"

	efi "github.com/canonical/go-efilib"
	efi_linux "github.com/canonical/go-efilib/linux"
)

// Variables abstracts away the host-specific bits of the efivars module
type Variables interface {
	ListVariables() ([]efi.VariableDescriptor, error)
	GetVariable(guid efi.GUID, name string) (data []byte, attrs efi.VariableAttributes, err error)
	SetVariable(guid efi.GUID, name string, data []byte, attrs efi.VariableAttributes) error
	NewFileDevicePath(filepath string, mode efi_linux.FilePathToDevicePathMode) (efi.DevicePath, error)
	DelVariable(guid efi.GUID, name string) error
	ReadLoadOption(r io.Reader) (out *efi.LoadOption, err error)
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

func (v RealEFIVariables) NewFileDevicePath(filepath string, mode efi_linux.FilePathToDevicePathMode) (efi.DevicePath, error) {
	return efi_linux.FilePathToDevicePath(filepath, mode)
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

// ReadLoadOptions proxy
func (RealEFIVariables) ReadLoadOption(r io.Reader) (out *efi.LoadOption, err error) {
	return efi.ReadLoadOption(r)
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
