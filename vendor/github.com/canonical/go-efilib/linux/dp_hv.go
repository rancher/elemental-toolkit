// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"os"
	"path/filepath"

	efi "github.com/canonical/go-efilib"
)

var (
	hvVendorGuid = efi.MakeGUID(0x9b17e5a2, 0x0891, 0x42dd, 0xb653, [...]uint8{0x80, 0xb5, 0xc2, 0x28, 0x09, 0xba})

	hvSCSIGuid = efi.MakeGUID(0xba6163d9, 0x04a1, 0x4d29, 0xb605, [...]uint8{0x72, 0xe2, 0xff, 0xb1, 0xdc, 0x7f})
)

func handleHVDevicePathNode(state *devicePathBuilderState) error {
	component := state.PeekUnhandledSysfsComponents(1)
	deviceId, err := efi.DecodeGUIDString(component)
	if err != nil {
		return err
	}

	state.AdvanceSysfsPath(1)
	path := state.SysfsPath()

	classIdStr, err := os.ReadFile(filepath.Join(path, "class_id"))
	if err != nil {
		return err
	}

	classId, err := efi.DecodeGUIDString(string(classIdStr))
	if err != nil {
		return err
	}

	switch classId {
	case hvSCSIGuid:
		state.Interface = interfaceTypeSCSI
	default:
		return errUnsupportedDevice("unhandled device class: " + classId.String())
	}

	data := make([]byte, len(deviceId)+len(classId))
	copy(data, classId[:])
	copy(data[len(classId):], deviceId[:])

	state.Path = append(state.Path, &efi.VendorDevicePathNode{
		Type: efi.HardwareDevicePath,
		GUID: hvVendorGuid,
		Data: data})
	return nil
}
