// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/canonical/go-efilib/internal/uefi"
)

// LoadOptionAttributes corresponds to the attributes of a load option
type LoadOptionAttributes uint32

const (
	LoadOptionActive         LoadOptionAttributes = uefi.LOAD_OPTION_ACTIVE
	LoadOptionForceReconnect LoadOptionAttributes = uefi.LOAD_OPTION_FORCE_RECONNECT
	LoadOptionHidden         LoadOptionAttributes = uefi.LOAD_OPTION_HIDDEN
	LoadOptionCategory       LoadOptionAttributes = uefi.LOAD_OPTION_CATEGORY

	LoadOptionCategoryBoot LoadOptionAttributes = uefi.LOAD_OPTION_CATEGORY_BOOT
	LoadOptionCategoryApp  LoadOptionAttributes = uefi.LOAD_OPTION_CATEGORY_APP
)

// IsBootCategory indicates whether the attributes has the LOAD_OPTION_CATEGORY_BOOT
// flag set. These applications are typically part of the boot process.
func (a LoadOptionAttributes) IsBootCategory() bool {
	return a&LoadOptionCategory == LoadOptionCategoryBoot
}

// IsAppCategory indicates whether the attributes has the LOAD_OPTION_CATEGORY_APP
// flag set.
func (a LoadOptionAttributes) IsAppCategory() bool {
	return a&LoadOptionCategory == LoadOptionCategoryApp
}

// LoadOption corresponds to the EFI_LOAD_OPTION type.
type LoadOption struct {
	Attributes   LoadOptionAttributes
	Description  string
	FilePath     DevicePath
	OptionalData []byte
}

// String implements [fmt.Stringer].
func (o *LoadOption) String() string {
	return fmt.Sprintf(`EFI_LOAD_OPTION {
	Attributes: %d,
	Description: %q,
	FilePath: %s,
	OptionalData: %x,
}`, o.Attributes, o.Description, o.FilePath, o.OptionalData)
}

// Bytes returns the serialized form of this load option.
func (o *LoadOption) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := o.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write serializes this load option to the supplied io.Writer.
func (o *LoadOption) Write(w io.Writer) error {
	opt := uefi.EFI_LOAD_OPTION{
		Attributes:   uint32(o.Attributes),
		Description:  ConvertUTF8ToUCS2(o.Description + "\x00"),
		OptionalData: o.OptionalData}

	var dp bytes.Buffer
	if err := o.FilePath.Write(&dp); err != nil {
		return err
	}
	if dp.Len() > math.MaxUint16 {
		return errors.New("FilePath too long")
	}
	opt.FilePathList = dp.Bytes()
	opt.FilePathListLength = uint16(dp.Len())

	return opt.Write(w)
}

// IsActive indicates whether the attributes has the LOAD_OPTION_ACTIVE flag set.
// These will be tried automaitcally if they are in BootOrder.
func (o *LoadOption) IsActive() bool {
	return o.Attributes&LoadOptionActive > 0
}

// IsVisible indicates whether the attributes does not have the LOAD_OPTION_HIDDEN
// flag set.
func (o *LoadOption) IsVisible() bool {
	return o.Attributes&LoadOptionHidden == 0
}

// IsBootCategory indicates whether the attributes has the LOAD_OPTION_CATEGORY_BOOT
// flag set. These applications are typically part of the boot process.
func (o *LoadOption) IsBootCategory() bool {
	return o.Attributes.IsBootCategory()
}

// IsAppCategory indicates whether the attributes has the LOAD_OPTION_CATEGORY_APP
// flag set.
func (o *LoadOption) IsAppCategory() bool {
	return o.Attributes.IsAppCategory()
}

// ReadLoadOption reads a LoadOption from the supplied io.Reader. Due to the
// way that EFI_LOAD_OPTION is defined, where there is no size encoded for the
// OptionalData field, this function will consume all of the bytes available
// from the supplied reader.
func ReadLoadOption(r io.Reader) (out *LoadOption, err error) {
	opt, err := uefi.Read_EFI_LOAD_OPTION(r)
	if err != nil {
		return nil, err
	}

	out = &LoadOption{
		Attributes:   LoadOptionAttributes(opt.Attributes),
		Description:  ConvertUTF16ToUTF8(opt.Description),
		OptionalData: opt.OptionalData}

	dp, err := ReadDevicePath(bytes.NewReader(opt.FilePathList))
	if err != nil {
		return nil, fmt.Errorf("cannot read device path: %w", err)
	}
	out.FilePath = dp

	return out, nil
}
