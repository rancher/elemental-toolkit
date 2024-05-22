/*
Copyright © 2022 - 2024 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mocks

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"strings"

	efi "github.com/canonical/go-efilib"
	efi_linux "github.com/canonical/go-efilib/linux"
	"github.com/twpayne/go-vfs/v4"
)

type mockEFIVariable struct {
	data  []byte
	attrs efi.VariableAttributes
}

// MockEFIVariables implements an in-memory variable store.
type MockEFIVariables struct {
	store         map[efi.VariableDescriptor]mockEFIVariable
	loadOptionErr error
}

func NewMockEFIVariables() *MockEFIVariables {
	return &MockEFIVariables{
		store: make(map[efi.VariableDescriptor]mockEFIVariable),
	}
}

func (m *MockEFIVariables) WithLoadOptionError(err error) *MockEFIVariables {
	m.loadOptionErr = err
	return m
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
func (m MockEFIVariables) SetVariable(guid efi.GUID, name string, data []byte, attrs efi.VariableAttributes) error {
	if len(data) == 0 {
		delete(m.store, efi.VariableDescriptor{Name: name, GUID: guid})
	} else {
		m.store[efi.VariableDescriptor{Name: name, GUID: guid}] = mockEFIVariable{data, attrs}
	}
	return nil
}

func (m MockEFIVariables) ReadLoadOption(_ io.Reader) (out *efi.LoadOption, err error) {
	return nil, m.loadOptionErr
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

func (m MockEFIVariables) NewFileDevicePath(fpath string, _ efi_linux.FilePathToDevicePathMode) (efi.DevicePath, error) {
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
