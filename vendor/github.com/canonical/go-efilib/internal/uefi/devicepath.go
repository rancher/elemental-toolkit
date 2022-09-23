// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

import (
	"encoding/binary"
	"io"

	"github.com/canonical/go-efilib/internal/ioerr"
)

const (
	NO_DISK_SIGNATURE   = 0x00
	SIGNATURE_TYPE_MBR  = 0x01
	SIGNATURE_TYPE_GUID = 0x02

	HARDWARE_DEVICE_PATH  = 0x01
	ACPI_DEVICE_PATH      = 0x02
	MESSAGING_DEVICE_PATH = 0x03
	MEDIA_DEVICE_PATH     = 0x04
	BBS_DEVICE_PATH       = 0x05
	END_DEVICE_PATH_TYPE  = 0x7f

	HW_PCI_DP    = 0x01
	HW_VENDOR_DP = 0x04

	ACPI_DP          = 0x01
	ACPI_EXTENDED_DP = 0x02

	MSG_ATAPI_DP               = 0x01
	MSG_SCSI_DP                = 0x02
	MSG_USB_DP                 = 0x05
	MSG_USB_CLASS_DP           = 0x0f
	MSG_VENDOR_DP              = 0x0a
	MSG_USB_WWID_DP            = 0x10
	MSG_DEVICE_LOGICAL_UNIT_DP = 0x11
	MSG_SATA_DP                = 0x12
	MSG_NVME_NAMESPACE_DP      = 0x17

	MEDIA_HARDDRIVE_DP             = 0x01
	MEDIA_CDROM_DP                 = 0x02
	MEDIA_VENDOR_DP                = 0x03
	MEDIA_FILEPATH_DP              = 0x04
	MEDIA_PIWG_FW_FILE_DP          = 0x06
	MEDIA_PIWG_FW_VOL_DP           = 0x07
	MEDIA_RELATIVE_OFFSET_RANGE_DP = 0x08

	END_ENTIRE_DEVICE_PATH_SUBTYPE = 0xff
)

type EFI_DEVICE_PATH_PROTOCOL struct {
	Type    uint8
	SubType uint8
	Length  uint16
}

type PCI_DEVICE_PATH struct {
	Header   EFI_DEVICE_PATH_PROTOCOL
	Function uint8
	Device   uint8
}

type VENDOR_DEVICE_PATH struct {
	Header EFI_DEVICE_PATH_PROTOCOL
	Guid   EFI_GUID
}

type ACPI_HID_DEVICE_PATH struct {
	Header EFI_DEVICE_PATH_PROTOCOL
	HID    uint32
	UID    uint32
}

type ACPI_EXTENDED_HID_DEVICE_PATH struct {
	Header EFI_DEVICE_PATH_PROTOCOL
	HID    uint32
	UID    uint32
	CID    uint32
}

type ATAPI_DEVICE_PATH struct {
	Header           EFI_DEVICE_PATH_PROTOCOL
	PrimarySecondary uint8
	SlaveMaster      uint8
	Lun              uint16
}

type SCSI_DEVICE_PATH struct {
	Header EFI_DEVICE_PATH_PROTOCOL
	Pun    uint16
	Lun    uint16
}

type USB_DEVICE_PATH struct {
	Header           EFI_DEVICE_PATH_PROTOCOL
	ParentPortNumber uint8
	InterfaceNumber  uint8
}

type USB_CLASS_DEVICE_PATH struct {
	Header         EFI_DEVICE_PATH_PROTOCOL
	VendorId       uint16
	ProductId      uint16
	DeviceClass    uint8
	DeviceSubClass uint8
	DeviceProtocol uint8
}

type USB_WWID_DEVICE_PATH struct {
	Header          EFI_DEVICE_PATH_PROTOCOL
	InterfaceNumber uint16
	VendorId        uint16
	ProductId       uint16
	SerialNumber    []uint16
}

func (p *USB_WWID_DEVICE_PATH) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, &p.Header); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.InterfaceNumber); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.VendorId); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.ProductId); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.SerialNumber); err != nil {
		return err
	}
	return nil
}

func Read_USB_WWID_DEVICE_PATH(r io.Reader) (out *USB_WWID_DEVICE_PATH, err error) {
	out = &USB_WWID_DEVICE_PATH{}
	if err := binary.Read(r, binary.LittleEndian, &out.Header); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &out.InterfaceNumber); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	if err := binary.Read(r, binary.LittleEndian, &out.VendorId); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	if err := binary.Read(r, binary.LittleEndian, &out.ProductId); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	out.SerialNumber = make([]uint16, (int(out.Header.Length)-binary.Size(out.Header)-binary.Size(out.InterfaceNumber)-binary.Size(out.VendorId)-binary.Size(out.ProductId))/2)
	if err := binary.Read(r, binary.LittleEndian, out.SerialNumber); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return out, nil
}

type DEVICE_LOGICAL_UNIT_DEVICE_PATH struct {
	Header EFI_DEVICE_PATH_PROTOCOL
	Lun    uint8
}

type SATA_DEVICE_PATH struct {
	Header                   EFI_DEVICE_PATH_PROTOCOL
	HBAPortNumber            uint16
	PortMultiplierPortNumber uint16
	Lun                      uint16
}

type NVME_NAMESPACE_DEVICE_PATH struct {
	Header        EFI_DEVICE_PATH_PROTOCOL
	NamespaceId   uint32
	NamespaceUuid uint64
}

type HARDDRIVE_DEVICE_PATH struct {
	Header          EFI_DEVICE_PATH_PROTOCOL
	PartitionNumber uint32
	PartitionStart  uint64
	PartitionSize   uint64
	Signature       [16]uint8
	MBRType         uint8
	SignatureType   uint8
}

type CDROM_DEVICE_PATH struct {
	Header         EFI_DEVICE_PATH_PROTOCOL
	BootEntry      uint32
	PartitionStart uint64
	PartitionSize  uint64
}

type FILEPATH_DEVICE_PATH struct {
	Header   EFI_DEVICE_PATH_PROTOCOL
	PathName []uint16
}

func (p *FILEPATH_DEVICE_PATH) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, &p.Header); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.PathName); err != nil {
		return err
	}
	return nil
}

func Read_FILEPATH_DEVICE_PATH(r io.Reader) (out *FILEPATH_DEVICE_PATH, err error) {
	out = &FILEPATH_DEVICE_PATH{}
	if err := binary.Read(r, binary.LittleEndian, &out.Header); err != nil {
		return nil, err
	}

	out.PathName = make([]uint16, (int(out.Header.Length)-binary.Size(out.Header))/2)
	if err := binary.Read(r, binary.LittleEndian, &out.PathName); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return out, nil
}

type MEDIA_FW_VOL_FILEPATH_DEVICE_PATH struct {
	Header     EFI_DEVICE_PATH_PROTOCOL
	FvFileName EFI_GUID
}

type MEDIA_FW_VOL_DEVICE_PATH struct {
	Header EFI_DEVICE_PATH_PROTOCOL
	FvName EFI_GUID
}

type MEDIA_RELATIVE_OFFSET_RANGE_DEVICE_PATH struct {
	Header         EFI_DEVICE_PATH_PROTOCOL
	Reserved       uint32
	StartingOffset uint64
	EndingOffset   uint64
}
