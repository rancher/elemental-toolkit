// Copyright 2024 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/canonical/go-efilib/internal/uefi"
)

// LoadOptionClass describes a class of load option
type LoadOptionClass string

const (
	// LoadOptionClassDriver corresponds to drivers that are processed before
	// normal boot options and before the initial ready to boot signal.
	LoadOptionClassDriver LoadOptionClass = "Driver"

	// LadOptionClassSysPrep corresponds to system preparation applications that
	// are processed before normal boot options and before the initial
	// ready to boot signal.
	LoadOptionClassSysPrep LoadOptionClass = "SysPrep"

	// LoadOptionClassBoot corresponds to normal boot applicationds.
	LoadOptionClassBoot LoadOptionClass = "Boot"

	// LoadOptionClassPlatformRecovery corresponds to platform supplied recovery
	// applications.
	LoadOptionClassPlatformRecovery LoadOptionClass = "PlatformRecovery"
)

// OSIndications provides a way for the firmware to advertise features to the OS
// and a way to request the firmware perform a specific action on the next boot.
type OSIndications uint64

const (
	OSIndicationBootToFWUI                   = uefi.EFI_OS_INDICATIONS_BOOT_TO_FW_UI
	OSIndicationTimestampRevocation          = uefi.EFI_OS_INDICATIONS_TIMESTAMP_REVOCATION
	OSIndicationFileCapsuleDeliverySupported = uefi.EFI_OS_INDICATIONS_FILE_CAPSULE_DELIVERY_SUPPORTED
	OSIndicationFMPCapsuleSupported          = uefi.EFI_OS_INDICATIONS_FMP_CAPSULE_SUPPORTED
	OSIndicationCapsuleResultVarSupported    = uefi.EFI_OS_INDICATIONS_CAPSULE_RESULT_VAR_SUPPORTED
	OSIndicationStartOSRecovery              = uefi.EFI_OS_INDICATIONS_START_OS_RECOVERY
	OSIndicationStartPlatformRecovery        = uefi.EFI_OS_INDICATIONS_START_PLATFORM_RECOVERY
	OSIndicationJSONConfigDataRefresh        = uefi.EFI_OS_INDICATIONS_JSON_CONFIG_DATA_REFRESH
)

// BootOptionSupport provides a way for the firmware to indicate certain boot
// options that are supported.
type BootOptionSupport uint32

const (
	BootOptionSupportKey     = uefi.EFI_BOOT_OPTION_SUPPORT_KEY
	BootOptionSupportApp     = uefi.EFI_BOOT_OPTION_SUPPORT_APP
	BootOptionSupportSysPrep = uefi.EFI_BOOT_OPTION_SUPPORT_SYSPREP
	BootOptionSupportCount   = uefi.EFI_BOOT_OPTION_SUPPORT_COUNT
)

// KeyCount returns the supported number of key presses (up to 3).
func (s BootOptionSupport) KeyCount() uint8 {
	return uint8((s & BootOptionSupportCount) >> 8)
}

// ReadOSIndicationsSupportedVariable returns the value of the OSIndicationsSupported
// variable in the global namespace. In general [DefaultVarContext] should be supplied
// to this.
func ReadOSIndicationsSupportedVariable(ctx context.Context) (OSIndications, error) {
	data, _, err := ReadVariable(ctx, "OsIndicationsSupported", GlobalVariable)
	if err != nil {
		return 0, err
	}
	if len(data) != 8 {
		return 0, fmt.Errorf("variable contents has an unexpected size (%d bytes)", len(data))
	}
	return OSIndications(binary.LittleEndian.Uint64(data)), nil
}

// WriteOSIndicationsVariable writes the supplied value to the OsIndications
// global variable in order to send commands to the firmware for the next
// boot. In general [DefaultVarContext] should be supplied to this.
func WriteOSIndicationsVariable(ctx context.Context, value OSIndications) error {
	if value&^(OSIndicationBootToFWUI|OSIndicationFileCapsuleDeliverySupported|OSIndicationStartOSRecovery|OSIndicationStartPlatformRecovery|OSIndicationJSONConfigDataRefresh) > 0 {
		return errors.New("supplied value contains bits set that have no function")
	}
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], uint64(value))

	return WriteVariable(ctx, "OsIndications", GlobalVariable, AttributeNonVolatile|AttributeBootserviceAccess|AttributeRuntimeAccess, data[:])
}

// ReadBootOptionSupportVariable returns the value of the BootOptionSupport
// variable in the global namespace. In general [DefaultVarContext] should be supplied
// to this.
func ReadBootOptionSupportVariable(ctx context.Context) (BootOptionSupport, error) {
	data, _, err := ReadVariable(ctx, "BootOptionSupport", GlobalVariable)
	if err != nil {
		return 0, err
	}
	if len(data) != 4 {
		return 0, fmt.Errorf("variable contents has an unexpected size (%d bytes)", len(data))
	}
	return BootOptionSupport(binary.LittleEndian.Uint32(data)), nil
}

// ReadLoadOrderVariable returns the load option order for the specified class,
// which must be one of LoadOptionClassDriver, LoadOptionClassSysPrep, or
// LoadOptionClassBoot. In general [DefaultVarContext] should be supplied
// to this.
func ReadLoadOrderVariable(ctx context.Context, class LoadOptionClass) ([]uint16, error) {
	switch class {
	case LoadOptionClassDriver, LoadOptionClassSysPrep, LoadOptionClassBoot:
		// ok
	default:
		return nil, fmt.Errorf("invalid class %q: only suitable for Driver, SysPrep or Boot", class)
	}

	data, _, err := ReadVariable(ctx, string(class)+"Order", GlobalVariable)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)
	if r.Size()&0x1 > 0 {
		return nil, fmt.Errorf("%sOrder variable contents has odd size (%d bytes)", class, r.Size())
	}

	out := make([]uint16, r.Size()>>1)
	if err := binary.Read(r, binary.LittleEndian, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// WriteLoadOrderVariable writes the load option order for the specified class,
// which must be one of LoadOptionClassDriver, LoadOptionClassSysprep, or
// LoadOptionClassBoot. In general [DefaultVarContext] should be supplied
// to this.
//
// This will check that each entry corresponds to a valid load option before
// writing the new order.
func WriteLoadOrderVariable(ctx context.Context, class LoadOptionClass, order []uint16) error {
	switch class {
	case LoadOptionClassDriver, LoadOptionClassSysPrep, LoadOptionClassBoot:
		// ok
	default:
		return fmt.Errorf("invalid class %q: only suitable for Driver, SysPrep or Boot", class)
	}

	// Check each load option
	for _, n := range order {
		if _, err := ReadLoadOptionVariable(ctx, class, n); err != nil {
			return fmt.Errorf("invalid load option %d: %w", n, err)
		}
	}

	w := new(bytes.Buffer)
	if err := binary.Write(w, binary.LittleEndian, order); err != nil {
		return err
	}

	return WriteVariable(ctx, string(class)+"Order", GlobalVariable, AttributeNonVolatile|AttributeBootserviceAccess|AttributeRuntimeAccess, w.Bytes())
}

// ReadLoadOptionVariable returns the LoadOption for the specified class and option number.
// The variable is read from the global namespace. In general [DefaultVarContext] should be
// supplied to this.
func ReadLoadOptionVariable(ctx context.Context, class LoadOptionClass, n uint16) (*LoadOption, error) {
	switch class {
	case LoadOptionClassDriver, LoadOptionClassSysPrep, LoadOptionClassBoot, LoadOptionClassPlatformRecovery:
		// ok
	default:
		return nil, fmt.Errorf("invalid class %q: only suitable for Driver, SysPrep, Boot, or PlatformRecovery", class)
	}

	data, _, err := ReadVariable(ctx, fmt.Sprintf("%s%04x", class, n), GlobalVariable)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	option, err := ReadLoadOption(r)
	if err != nil {
		return nil, fmt.Errorf("cannot decode LoadOption: %w", err)
	}

	return option, nil
}

// WriteLoadOptionVariable writes the supplied LoadOption to a variable for the specified
// class and option number. The variable is written to the global namespace. This will
// overwrite any variable that already exists. The class must be one of LoadOptionClassDriver,
// LoadOptionClassSysprep, or LoadOptionClassBoot. In general [DefaultVarContext] should be
// supplied to this.
func WriteLoadOptionVariable(ctx context.Context, class LoadOptionClass, n uint16, option *LoadOption) error {
	switch class {
	case LoadOptionClassDriver, LoadOptionClassSysPrep, LoadOptionClassBoot:
		// ok
	default:
		return fmt.Errorf("invalid class %q: only suitable for Driver, SysPrep or Boot", class)
	}

	w := new(bytes.Buffer)
	if err := option.Write(w); err != nil {
		return fmt.Errorf("cannot serialize load option: %w", err)
	}

	return WriteVariable(ctx, fmt.Sprintf("%s%04x", class, n), GlobalVariable, AttributeNonVolatile|AttributeBootserviceAccess|AttributeRuntimeAccess, w.Bytes())
}

// DeleteLoadOptionVariable deletes the load option variable for the specified
// class and option number. The variable is written to the global namespace. This will
// succeed even if the variable doesn't alreeady exist. The class must be one of
// LoadOptionClassDriver, LoadOptionClassSysprep, or LoadOptionClassBoot. In general
// [DefaultVarContext] should be supplied to this.
func DeleteLoadOptionVariable(ctx context.Context, class LoadOptionClass, n uint16) error {
	switch class {
	case LoadOptionClassDriver, LoadOptionClassSysPrep, LoadOptionClassBoot:
		// ok
	default:
		return fmt.Errorf("invalid class %q: only suitable for Driver, SysPrep or Boot", class)
	}

	return WriteVariable(ctx, fmt.Sprintf("%s%04x", class, n), GlobalVariable, AttributeNonVolatile|AttributeBootserviceAccess|AttributeRuntimeAccess, nil)
}

// ListLoadOptionNumbers lists the numbers of all of the load option variables
// for the specified class from the global namespace. The returned numbers will be
// sorted in ascending order. In general [DefaultVarContext] should be supplied to
// this.
func ListLoadOptionNumbers(ctx context.Context, class LoadOptionClass) ([]uint16, error) {
	names, err := ListVariables(ctx)
	if err != nil {
		return nil, err
	}

	var out []uint16
	for _, name := range names {
		if name.GUID != GlobalVariable {
			continue
		}
		if !strings.HasPrefix(name.Name, string(class)) {
			continue
		}
		if len(name.Name) != len(class)+4 {
			continue
		}

		var x uint16
		if n, err := fmt.Sscanf(name.Name, string(class)+"%x", &x); err != nil || n != 1 {
			continue
		}

		out = append(out, x)
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// NextAvailableLoadOptionNumber returns the next available load option number for
// the specified class, which must be one of LoadOptionClassDriver,
// LoadOptionClassSysprep, or LoadOptionClassBoot. In general [DefaultVarContext]
// should be supplied to this.
func NextAvailableLoadOptionNumber(ctx context.Context, class LoadOptionClass) (uint16, error) {
	switch class {
	case LoadOptionClassDriver, LoadOptionClassSysPrep, LoadOptionClassBoot:
		// ok
	default:
		return 0, fmt.Errorf("invalid class %q: only suitable for Driver, SysPrep or Boot", class)
	}

	used, err := ListLoadOptionNumbers(ctx, class)
	if err != nil {
		return 0, err
	}
	if len(used) > math.MaxUint16 {
		return 0, errors.New("no load option number available")
	}
	for i, n := range used {
		if uint16(i) != n {
			return uint16(i), nil
		}
	}
	return uint16(len(used)), nil
}

// ReadBootNextVariable returns the option number of the boot entry to try next.
// In general [DefaultVarContext] should be supplied to this.
func ReadBootNextVariable(ctx context.Context) (uint16, error) {
	data, _, err := ReadVariable(ctx, "BootNext", GlobalVariable)
	if err != nil {
		return 0, err
	}

	if len(data) != 2 {
		return 0, fmt.Errorf("BootNext variable contents has the wrong size (%d bytes)", len(data))
	}

	return binary.LittleEndian.Uint16(data), nil
}

// WriteBootNextVariable writes the option number of the boot entry to try next.
// In general [DefaultVarContext] should be supplied to this.
func WriteBootNextVariable(ctx context.Context, n uint16) error {
	if _, err := ReadLoadOptionVariable(ctx, LoadOptionClassBoot, n); err != nil {
		return fmt.Errorf("invalid load option %d: %w", n, err)
	}

	var data [2]byte
	binary.LittleEndian.PutUint16(data[:], n)

	return WriteVariable(ctx, "BootNext", GlobalVariable, AttributeNonVolatile|AttributeBootserviceAccess|AttributeRuntimeAccess, data[:])
}

// DeleteBootNextVariable deletes the option number of the boot entry to try next.
// In general [DefaultVarContext] should be supplied to this.
func DeleteBootNextVariable(ctx context.Context) error {
	return WriteVariable(ctx, "BootNext", GlobalVariable, AttributeNonVolatile|AttributeBootserviceAccess|AttributeRuntimeAccess, nil)
}

// ReadBootNextLoadOptionVariable returns the LoadOption for the boot entry to try next.
// In general [DefaultVarContext] should be supplied to this.
func ReadBootNextLoadOptionVariable(ctx context.Context) (*LoadOption, error) {
	n, err := ReadBootNextVariable(ctx)
	if err != nil {
		return nil, err
	}

	return ReadLoadOptionVariable(ctx, LoadOptionClassBoot, n)
}

// ReadBootCurrentVariable returns the option number used for the current boot.
// In general [DefaultVarContext] should be supplied to this.
func ReadBootCurrentVariable(ctx context.Context) (uint16, error) {
	data, _, err := ReadVariable(ctx, "BootCurrent", GlobalVariable)
	if err != nil {
		return 0, err
	}

	if len(data) != 2 {
		return 0, fmt.Errorf("BootCurrent variable contents has the wrong size (%d bytes)", len(data))
	}

	return binary.LittleEndian.Uint16(data), nil
}

// ReadOrderedLoadOptionVariables returns a list of LoadOptions in the order in which
// they will be tried by the boot manager for the specified class. The variables are all
// read from the global namespace. Where class is LoadOptionClassDriver, LoadOptionClassSysPrep,
// or LoadOptionClassBoot, this will use the corresponding *Order variable. It will skip entries
// for which there isn't a corresponding variable. Where class is LoadOptionClassPlatformRecovery,
// the order is determined by the variable names.
// In general [DefaultVarContext] should be supplied to this.
func ReadOrderedLoadOptionVariables(ctx context.Context, class LoadOptionClass) ([]*LoadOption, error) {
	var optNumbers []uint16
	switch class {
	case LoadOptionClassDriver, LoadOptionClassSysPrep, LoadOptionClassBoot:
		var err error
		optNumbers, err = ReadLoadOrderVariable(ctx, class)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain order: %w", err)
		}
	case LoadOptionClassPlatformRecovery:
		var err error
		optNumbers, err = ListLoadOptionNumbers(ctx, LoadOptionClassPlatformRecovery)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain load option numbers: %w", err)
		}
	}

	var opts []*LoadOption
	for _, n := range optNumbers {
		opt, err := ReadLoadOptionVariable(ctx, class, n)
		switch {
		case errors.Is(err, ErrVarNotExist):
			// skip and ignore missing number
		case err != nil:
			// handle all other errors
			return nil, fmt.Errorf("cannot read load option %d: %w", n, err)
		default:
			opts = append(opts, opt)
		}
	}

	return opts, nil
}
