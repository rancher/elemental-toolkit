// Copyright 2020 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/canonical/go-efilib/internal/ioerr"
	"github.com/canonical/go-efilib/internal/uefi"
	"github.com/canonical/go-efilib/mbr"
)

// DevicePathShortFormType describes whether a path is a recognized short-form
// path, and what type it is.
type DevicePathShortFormType int

const (
	// DevicePathNotShortForm indicates that a path is not a recognized short-form path
	DevicePathNotShortForm DevicePathShortFormType = iota

	// DevicePathShortFormHD indicates that a path is a HD() short-form path
	DevicePathShortFormHD

	// DevicePathShortFormUSBWWID indicates that a path is a UsbWwid() short-form path
	DevicePathShortFormUSBWWID

	// DevicePathShortFormUSBClass indicates that a path is a UsbClass() short-form path
	DevicePathShortFormUSBClass

	// DevicePathShortFormURI indicates that a path is a Uri() short-form path. Note that
	// this package does not currently directly support device paths containing URIs.
	DevicePathShortFormURI

	// DevicePathShortFormFilePath indicates that a path is a file path short-form path
	DevicePathShortFormFilePath
)

// IsShortForm whether the current form is a short-form value (ie, not DevicePathNotShortForm).
func (t DevicePathShortFormType) IsShortForm() bool {
	return t > DevicePathNotShortForm
}

// DevicePathMatch indicates how a device path matched
type DevicePathMatch int

const (
	// DevicePathNoMatch indicates that a pair of device paths did not match.
	DevicePathNoMatch DevicePathMatch = iota

	// DevicePathFullMatch indicates that a pair of device paths fully matched.
	DevicePathFullMatch

	// DevicePathShortFormHDMatch indicates that one device path begins with a
	// *[HardDriveDevicePathNode] and matches the end of the longer device path.
	DevicePathShortFormHDMatch

	// DevicePathShortFormUSBWWIDMatch indicates that one device path begins with
	// a *[USBWWIDDevicePathNode] and matches the end of the longer device path.
	DevicePathShortFormUSBWWIDMatch

	// DevicePathShortFormUSBClassMatch indicates that one device path begins with
	// a *[USBClassDevicePathNode] and matches the end of the longer device path.
	DevicePathShortFormUSBClassMatch

	// DevicePathShortFormFileMatch indicates that one device path begins with a
	// [FilePathDevicePathNode] and matches the end of the longer device path.
	DevicePathShortFormFileMatch
)

// DevicePathType is the type of a device path node.
// Deprecated: use [DevicePathNodeType].
type DevicePathType = DevicePathNodeType

// DevicePathNodeType is the type of a device path node.
type DevicePathNodeType uint8

// String implements [fmt.Stringer].
func (t DevicePathNodeType) String() string {
	switch t {
	case HardwareDevicePath:
		return "HardwarePath"
	case ACPIDevicePath:
		return "AcpiPath"
	case MessagingDevicePath:
		return "Msg"
	case MediaDevicePath:
		return "MediaPath"
	case BBSDevicePath:
		return "BbsPath"
	default:
		return fmt.Sprintf("Path[%02x]", uint8(t))
	}
}

const (
	HardwareDevicePath  DevicePathNodeType = uefi.HARDWARE_DEVICE_PATH
	ACPIDevicePath      DevicePathNodeType = uefi.ACPI_DEVICE_PATH
	MessagingDevicePath DevicePathNodeType = uefi.MESSAGING_DEVICE_PATH
	MediaDevicePath     DevicePathNodeType = uefi.MEDIA_DEVICE_PATH
	BBSDevicePath       DevicePathNodeType = uefi.BBS_DEVICE_PATH
)

// DevicePathSubType is the sub-type of a device path node. The meaning of
// this depends on the [DevicePathNodeType].
// Deprecated: use [DevicePathNodeSubType].
type DevicePathSubType = DevicePathNodeSubType

// DevicePathNodeSubType is the sub-type of a device path node. The meaning of
// this depends on the [DevicePathNodeType].
type DevicePathNodeSubType uint8

const (
	DevicePathNodeHWPCISubType    DevicePathNodeSubType = uefi.HW_PCI_DP
	DevicePathNodeHWVendorSubType DevicePathNodeSubType = uefi.HW_VENDOR_DP

	DevicePathNodeACPISubType         DevicePathNodeSubType = uefi.ACPI_DP
	DevicePathNodeACPIExtendedSubType DevicePathNodeSubType = uefi.ACPI_EXTENDED_DP

	DevicePathNodeMsgATAPISubType             DevicePathNodeSubType = uefi.MSG_ATAPI_DP
	DevicePathNodeMsgSCSISubType              DevicePathNodeSubType = uefi.MSG_SCSI_DP
	DevicePathNodeMsgUSBSubType               DevicePathNodeSubType = uefi.MSG_USB_DP
	DevicePathNodeMsgUSBClassSubType          DevicePathNodeSubType = uefi.MSG_USB_CLASS_DP
	DevicePathNodeMsgVendorSubType            DevicePathNodeSubType = uefi.MSG_VENDOR_DP
	DevicePathNodeMsgMACAddrSubType           DevicePathNodeSubType = uefi.MSG_MAC_ADDR_DP
	DevicePathNodeMsgIPv4SubType              DevicePathNodeSubType = uefi.MSG_IPv4_DP
	DevicePathNodeMsgIPv6SubType              DevicePathNodeSubType = uefi.MSG_IPv6_DP
	DevicePathNodeMsgUSBWWIDSubType           DevicePathNodeSubType = uefi.MSG_USB_WWID_DP
	DevicePathNodeMsgDeviceLogicalUnitSubType DevicePathNodeSubType = uefi.MSG_DEVICE_LOGICAL_UNIT_DP
	DevicePathNodeMsgSATASubType              DevicePathNodeSubType = uefi.MSG_SATA_DP
	DevicePathNodeMsgNVMENamespaceSubType     DevicePathNodeSubType = uefi.MSG_NVME_NAMESPACE_DP

	DevicePathNodeMediaHardDriveSubType           DevicePathNodeSubType = uefi.MEDIA_HARDDRIVE_DP
	DevicePathNodeMediaCDROMSubType               DevicePathNodeSubType = uefi.MEDIA_CDROM_DP
	DevicePathNodeMediaVendorSubType              DevicePathNodeSubType = uefi.MEDIA_VENDOR_DP
	DevicePathNodeMediaFilePathSubType            DevicePathNodeSubType = uefi.MEDIA_FILEPATH_DP
	DevicePathNodeMediaFwFileSubType              DevicePathNodeSubType = uefi.MEDIA_PIWG_FW_FILE_DP
	DevicePathNodeMediaFwVolSubType               DevicePathNodeSubType = uefi.MEDIA_PIWG_FW_VOL_DP
	DevicePathNodeMediaRelativeOffsetRangeSubType DevicePathNodeSubType = uefi.MEDIA_RELATIVE_OFFSET_RANGE_DP
)

// DevicePathNodeCompoundType describes the compound type of a device path node,
// comprised of the type and sub-type.
type DevicePathNodeCompoundType uint16

// Type returns the devicepath node type.
func (t DevicePathNodeCompoundType) Type() DevicePathNodeType {
	return DevicePathNodeType(t >> 8)
}

// Subtype returns the devicepath node sub-type.
func (t DevicePathNodeCompoundType) SubType() DevicePathNodeSubType {
	return DevicePathNodeSubType(t)
}

// IsValid indicates whether this compound type is valid. If either the
// type or sub-type are 0, then it is invalid.
func (t DevicePathNodeCompoundType) IsValid() bool {
	return t.Type() != 0 && t.SubType() != 0
}

const (
	devicePathNodePCIType      DevicePathNodeCompoundType = DevicePathNodeCompoundType(HardwareDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeHWPCISubType)
	devicePathNodeVendorHWType DevicePathNodeCompoundType = DevicePathNodeCompoundType(HardwareDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeHWVendorSubType)

	devicePathNodeACPIType         DevicePathNodeCompoundType = DevicePathNodeCompoundType(ACPIDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeACPISubType)
	devicePathNodeACPIExtendedType DevicePathNodeCompoundType = DevicePathNodeCompoundType(ACPIDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeACPIExtendedSubType)

	devicePathNodeATAPIType             DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgATAPISubType)
	devicePathNodeSCSIType              DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgSCSISubType)
	devicePathNodeUSBType               DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgUSBSubType)
	devicePathNodeUSBClassType          DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgUSBClassSubType)
	devicePathNodeVendorMsgType         DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgVendorSubType)
	devicePathNodeMACAddrType           DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgMACAddrSubType)
	devicePathNodeIPv4Type              DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgIPv4SubType)
	devicePathNodeIPv6Type              DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgIPv6SubType)
	devicePathNodeUSBWWIDType           DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgUSBWWIDSubType)
	devicePathNodeDeviceLogicalUnitType DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgDeviceLogicalUnitSubType)
	devicePathNodeSATAType              DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgSATASubType)
	devicePathNodeNVMENamespaceType     DevicePathNodeCompoundType = DevicePathNodeCompoundType(MessagingDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMsgNVMENamespaceSubType)

	devicePathNodeHardDriveType           DevicePathNodeCompoundType = DevicePathNodeCompoundType(MediaDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMediaHardDriveSubType)
	devicePathNodeCDROMType               DevicePathNodeCompoundType = DevicePathNodeCompoundType(MediaDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMediaCDROMSubType)
	devicePathNodeVendorMediaType         DevicePathNodeCompoundType = DevicePathNodeCompoundType(MediaDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMediaVendorSubType)
	devicePathNodeFilePathType            DevicePathNodeCompoundType = DevicePathNodeCompoundType(MediaDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMediaFilePathSubType)
	devicePathNodeFwFileType              DevicePathNodeCompoundType = DevicePathNodeCompoundType(MediaDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMediaFwFileSubType)
	devicePathNodeFwVolType               DevicePathNodeCompoundType = DevicePathNodeCompoundType(MediaDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMediaFwVolSubType)
	devicePathNodeRelativeOffsetRangeType DevicePathNodeCompoundType = DevicePathNodeCompoundType(MediaDevicePath)<<8 | DevicePathNodeCompoundType(DevicePathNodeMediaRelativeOffsetRangeSubType)
)

const (
	DevicePathNodePCIType      DevicePathNodeCompoundType = devicePathNodePCIType      // nodes of this type are implemented by *PCIDevicePathNode
	DevicePathNodeVendorHWType DevicePathNodeCompoundType = devicePathNodeVendorHWType // nodes of this type are implemented by *VendorDevicePathNode

	DevicePathNodeACPIType         DevicePathNodeCompoundType = devicePathNodeACPIType         // nodes of this type are implemented by *ACPIDevicePathNode
	DevicePathNodeACPIExtendedType DevicePathNodeCompoundType = devicePathNodeACPIExtendedType // nodes of this type are implemented by *ACPIExtendedDevicePathNode

	DevicePathNodeATAPIType             DevicePathNodeCompoundType = devicePathNodeATAPIType             // nodes of this type are implemented by *ATAPIDevicePathNode
	DevicePathNodeSCSIType              DevicePathNodeCompoundType = devicePathNodeSCSIType              // nodes of this type are implemented by *SCSIDevicePathNode
	DevicePathNodeUSBType               DevicePathNodeCompoundType = devicePathNodeUSBType               // nodes of this type are implemented by *USBDevicePathNode
	DevicePathNodeUSBClassType          DevicePathNodeCompoundType = devicePathNodeUSBClassType          // nodes of this type are implemented by *USBClassDevicePathNode
	DevicePathNodeVendorMsgType         DevicePathNodeCompoundType = devicePathNodeVendorMsgType         // nodes of this type are implemented by *VendorDevicePathNode
	DevicePathNodeMACAddrType           DevicePathNodeCompoundType = devicePathNodeMACAddrType           // nodes of this type are implemented by *MACAddrDevicePathNode
	DevicePathNodeIPv4Type              DevicePathNodeCompoundType = devicePathNodeIPv4Type              // nodes of this type are implemented by *IPv4DevicePathNode
	DevicePathNodeIPv6Type              DevicePathNodeCompoundType = devicePathNodeIPv6Type              // nodes of this type are implemented by *IPv6DevicePathNode
	DevicePathNodeUSBWWIDType           DevicePathNodeCompoundType = devicePathNodeUSBWWIDType           // nodes of this type are implemented by *USBWWIDDevicePathNode
	DevicePathNodeDeviceLogicalUnitType DevicePathNodeCompoundType = devicePathNodeDeviceLogicalUnitType // nodes of this type are implemented by *DeviceLogicalUnitDevicePathNode
	DevicePathNodeSATAType              DevicePathNodeCompoundType = devicePathNodeSATAType              // nodes of this type are implemented by *SATADevicePathNode
	DevicePathNodeNVMENamespaceType     DevicePathNodeCompoundType = devicePathNodeNVMENamespaceType     // nodes of this type are implemented by *NVMENamespaceDevicePathNode

	DevicePathNodeHardDriveType           DevicePathNodeCompoundType = devicePathNodeHardDriveType           // nodes of this type are implemented by *HardDriveDevicePathNode
	DevicePathNodeCDROMType               DevicePathNodeCompoundType = devicePathNodeCDROMType               // nodes of this type are implemented by *CDROMDevicePathNode
	DevicePathNodeVendorMediaType         DevicePathNodeCompoundType = devicePathNodeVendorMediaType         // nodes of this type are implemented by *VendorDevicePathNode
	DevicePathNodeFilePathType            DevicePathNodeCompoundType = devicePathNodeFilePathType            // nodes of this type are implemented by FilePathDevicePathNode
	DevicePathNodeFwFileType              DevicePathNodeCompoundType = devicePathNodeFwFileType              // nodes of this type are implemented by FWFileDevicePathNode
	DevicePathNodeFwVolType               DevicePathNodeCompoundType = devicePathNodeFwVolType               // nodes of this type are implemented by FWVolDevicePathNode
	DevicePathNodeRelativeOffsetRangeType DevicePathNodeCompoundType = devicePathNodeRelativeOffsetRangeType // nodes of this type are implemented by *RelativeOffsetRangeDevicePathNode
)

// DevicePathToStringFlags defines flags for [DevicePath.ToString] and
// DevicePathNode.ToString.
type DevicePathToStringFlags int

func (f DevicePathToStringFlags) DisplayOnly() bool {
	return f&DevicePathDisplayOnly > 0
}

func (f DevicePathToStringFlags) AllowVendorShortcuts() bool {
	return f&DevicePathAllowVendorShortcuts > 0
}

func (f DevicePathToStringFlags) DisplayFWGUIDNames() bool {
	mask := DevicePathDisplayOnly | DevicePathDisplayFWGUIDNames
	return f&mask == mask
}

const (
	// DevicePathDisplayOnly indicates that each node is converted
	// to the shorter text representation.
	DevicePathDisplayOnly DevicePathToStringFlags = 1 << 0

	// DevicePathAllowVendorShortcuts indicates that that vendor
	// specific device path nodes should print using a type-specific
	// format based on the vendor GUID, if it is recognized.
	DevicePathAllowVendorShortcuts DevicePathToStringFlags = 1 << 1

	// DevicePathDisplayFWGUIDNames indicates that firmware volume
	// and firmware file device path nodes should print the industry
	// standard name for a GUID, if it is recognized and if lookup
	// functions have been registered with RegisterFWVolNameLookup
	// and RegisterFWFileNameLookup. This only works with
	// DevicePathDisplayOnly.
	DevicePathDisplayFWGUIDNames DevicePathToStringFlags = 1 << 2
)

// DevicePathNode represents a single node in a device path.
type DevicePathNode interface {
	CompoundType() DevicePathNodeCompoundType                 // Return the compound type of this node
	AsGenericDevicePathNode() (*GenericDevicePathNode, error) // Convert this node to a GenericDevicePathNode

	// String should return a string for this node, equivalent to
	// ToString(DevicePathNodeDisplayOnly|DevicePathNodeAllowShortcuts).
	// For more control, use ToString
	String() string

	ToString(flags DevicePathToStringFlags) string // Return a string for this node
	Write(w io.Writer) error                       // Serialize this node to the supplied io.Writer
}

type devicePathToStringer interface {
	ToString(flags DevicePathToStringFlags) string
}

func devicePathString[P devicePathToStringer](path P) string {
	return path.ToString(DevicePathDisplayOnly | DevicePathAllowVendorShortcuts)
}

type devicePathFormatter[P devicePathToStringer] struct {
	path P
}

func (formatter devicePathFormatter[P]) Format(f fmt.State, verb rune) {
	switch verb {
	case 's', 'v':
		flags := DevicePathAllowVendorShortcuts
		if !f.Flag('+') {
			flags |= DevicePathDisplayOnly
			if f.Flag('#') {
				flags |= DevicePathDisplayFWGUIDNames
			}
		}

		var b bytes.Buffer
		b.WriteString(formatter.path.ToString(flags))
		b.WriteTo(f)
	default:
		panic(fmt.Sprintf("invalid verb: %c", verb))
	}
}

// DevicePath represents a complete device path with the first node
// representing the root.
type DevicePath []DevicePathNode

func (p DevicePath) Formatter() fmt.Formatter {
	return devicePathFormatter[DevicePath]{path: p}
}

// ToString returns a string representation of this device path with the
// supplied flags.
func (p DevicePath) ToString(flags DevicePathToStringFlags) string {
	var b strings.Builder
	for _, node := range p {
		fmt.Fprintf(&b, "\\%s", node.ToString(flags))
	}
	return b.String()
}

// String implements [fmt.Stringer], and returns the display only string.
// For more control, use [DevicePath.ToString].
func (p DevicePath) String() string {
	return devicePathString(p)
}

// Bytes returns the serialized form of this device path.
func (p DevicePath) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write serializes the complete device path to w.
func (p DevicePath) Write(w io.Writer) error {
	for i, node := range p {
		if err := node.Write(w); err != nil {
			return fmt.Errorf("cannot write node %d: %w", i, err)
		}
	}

	end := uefi.EFI_DEVICE_PATH_PROTOCOL{
		Type:    uefi.END_DEVICE_PATH_TYPE,
		SubType: uefi.END_ENTIRE_DEVICE_PATH_SUBTYPE,
		Length:  4}
	return binary.Write(w, binary.LittleEndian, &end)
}

// DevicePathFindFirstOccurrence finds the first occurrence of the device path
// node with the specified type and returns it and the remaining components of
// the device path.
func DevicePathFindFirstOccurrence[T DevicePathNode](p DevicePath) DevicePath {
	for i, n := range p {
		if _, ok := n.(T); ok {
			return p[i:]
		}
	}
	return nil
}

func (p DevicePath) matchesInternal(other DevicePath, onlyFull bool) DevicePathMatch {
	var pBytes bytes.Buffer
	if err := p.Write(&pBytes); err != nil {
		return DevicePathNoMatch
	}
	var otherBytes bytes.Buffer
	if err := other.Write(&otherBytes); err != nil {
		return DevicePathNoMatch
	}
	if bytes.Equal(pBytes.Bytes(), otherBytes.Bytes()) {
		// We have a full, exact match
		return DevicePathFullMatch
	}

	if onlyFull {
		// If we're only permitted to find a full match, return no match now.
		return DevicePathNoMatch
	}

	// Check if the shortest path is a short-form path. If so, convert the longer
	// path to the same type of short-form path and test if there is a short-form
	// match.
	var (
		shortestPath DevicePath
		longestPath  DevicePath
	)
	switch {
	case len(p) > len(other):
		shortestPath = other
		longestPath = p
	case len(p) < len(other):
		shortestPath = p
		longestPath = other
	default:
		// Both paths are the same length and were not a full match.
		// There is no match here.
		return DevicePathNoMatch
	}

	switch shortestPath.ShortFormType() {
	case DevicePathShortFormHD:
		longestPath = DevicePathFindFirstOccurrence[*HardDriveDevicePathNode](longestPath)
		if res := longestPath.matchesInternal(shortestPath, true); res == DevicePathFullMatch {
			return DevicePathShortFormHDMatch
		}
	case DevicePathShortFormUSBWWID:
		longestPath = DevicePathFindFirstOccurrence[*USBWWIDDevicePathNode](longestPath)
		if res := longestPath.matchesInternal(shortestPath, true); res == DevicePathFullMatch {
			return DevicePathShortFormUSBWWIDMatch
		}
	case DevicePathShortFormUSBClass:
		longestPath = DevicePathFindFirstOccurrence[*USBClassDevicePathNode](longestPath)
		if res := longestPath.matchesInternal(shortestPath, true); res == DevicePathFullMatch {
			return DevicePathShortFormUSBClassMatch
		}
	case DevicePathShortFormFilePath:
		longestPath = DevicePathFindFirstOccurrence[FilePathDevicePathNode](longestPath)
		if res := longestPath.matchesInternal(shortestPath, true); res == DevicePathFullMatch {
			return DevicePathShortFormFileMatch
		}
	}

	return DevicePathNoMatch
}

// Matches indicates whether other matches this path in some way, and returns
// the type of match. If either path is a HD() short-form path, this may return
// DevicePathShortFormHDMatch. If either path is a UsbWwid() short-form path, this may
// return DevicePathShortFormUSBWWIDMatch. If either path is a UsbClass() short-form path,
// this may return DevicePathShortFormUSBClassMatch. If either path is a file path short-form
// path, this may return DevicePathShortFormFileMatch. This returns DevicePathFullMatch
// if the supplied path fully matches, and DevicePathNoMatch if there is no match.
func (p DevicePath) Matches(other DevicePath) DevicePathMatch {
	return p.matchesInternal(other, false)
}

// ShortFormType returns whether this is a short-form type of path, and if so,
// what type of short-form path. The UEFI boot manager is required to handle a
// certain set of well defined short-form paths that begin with a specific
// component.
func (p DevicePath) ShortFormType() DevicePathShortFormType {
	if len(p) == 0 {
		return DevicePathNotShortForm
	}

	switch n := p[0].(type) {
	case *HardDriveDevicePathNode:
		_ = n
		return DevicePathShortFormHD
	case *USBWWIDDevicePathNode:
		_ = n
		return DevicePathShortFormUSBWWID
	case *USBClassDevicePathNode:
		_ = n
		return DevicePathShortFormUSBClass
	case *GenericDevicePathNode:
		if n.Type == MessagingDevicePath && n.SubType == uefi.MSG_URI_DP {
			return DevicePathShortFormURI
		}
	case FilePathDevicePathNode:
		return DevicePathShortFormFilePath
	}

	return DevicePathNotShortForm
}

// GenericDevicePathNode corresponds to a device path nodes with a type that is
// not handled by this package
type GenericDevicePathNode struct {
	Type    DevicePathNodeType
	SubType DevicePathSubType // the meaning of the sub-type depends on the Type field.
	Data    []byte            // An opaque blob of data associated with this node
}

func readGenericDevicePathNode(r io.Reader) (*GenericDevicePathNode, error) {
	var n uefi.EFI_DEVICE_PATH_PROTOCOL
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	data, _ := io.ReadAll(r)

	return &GenericDevicePathNode{
		Type:    DevicePathNodeType(n.Type),
		SubType: DevicePathSubType(n.SubType),
		Data:    data}, nil
}

func convertToGenericDevicePathNode(n DevicePathNode) (*GenericDevicePathNode, error) {
	var buf bytes.Buffer
	if err := n.Write(&buf); err != nil {
		return nil, err
	}
	return readGenericDevicePathNode(&buf)
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *GenericDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeCompoundType(n.Type)<<8 | DevicePathNodeCompoundType(n.SubType)
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *GenericDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *GenericDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	var b strings.Builder

	switch n.Type {
	case HardwareDevicePath, ACPIDevicePath, MessagingDevicePath,
		MediaDevicePath, BBSDevicePath:
		fmt.Fprintf(&b, "%s(", n.Type)
	default:
		fmt.Fprintf(&b, "Path(%d,", n.Type)
	}
	fmt.Fprintf(&b, "%d", n.SubType)
	if len(n.Data) > 0 {
		fmt.Fprintf(&b, ",%x", n.Data)
	}
	b.WriteString(")")

	return b.String()
}

// String implements [fmt.Stringer].
func (n *GenericDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *GenericDevicePathNode) Write(w io.Writer) error {
	hdr := uefi.EFI_DEVICE_PATH_PROTOCOL{
		Type:    uint8(n.Type),
		SubType: uint8(n.SubType)}
	hdrSz := binary.Size(hdr)
	dataSz := len(n.Data)

	if dataSz > math.MaxUint16-hdrSz {
		return errors.New("Data too large")
	}
	hdr.Length = uint16(hdrSz + dataSz)

	if err := binary.Write(w, binary.LittleEndian, &hdr); err != nil {
		return err
	}
	_, err := w.Write(n.Data)
	return err
}

// PCIDevicePathNode corresponds to a PCI device path node.
type PCIDevicePathNode struct {
	Function uint8 // Function of device
	Device   uint8 // Device number of PCI bus
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *PCIDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodePCIType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *PCIDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

func (n *PCIDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return fmt.Sprintf("Pci(%#x,%#x)", n.Device, n.Function)
}

// String implements [fmt.Stringer].
func (n *PCIDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *PCIDevicePathNode) Write(w io.Writer) error {
	node := uefi.PCI_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		Function: n.Function,
		Device:   n.Device}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// VendorDevicePathNode corresponds to a vendor specific node.
type VendorDevicePathNode struct {
	Type DevicePathNodeType // The type of this node
	GUID GUID               // The vendor specific GUID
	Data []byte             // Vendor specific data
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *VendorDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	switch n.Type {
	case HardwareDevicePath:
		return DevicePathNodeVendorHWType
	case MediaDevicePath:
		return DevicePathNodeVendorMediaType
	case MessagingDevicePath:
		return DevicePathNodeVendorMsgType
	default:
		// Return an invalid compound type.
		return DevicePathNodeCompoundType(n.Type) << 8
	}
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *VendorDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *VendorDevicePathNode) ToString(flags DevicePathToStringFlags) string {
	var nodeType string
	switch n.Type {
	case HardwareDevicePath:
		nodeType = "Hw"
	case MessagingDevicePath:
		nodeType = "Msg"
		if flags.AllowVendorShortcuts() {
			switch n.GUID {
			case PCAnsiGuid:
				return "VenPcAnsi()"
			case VT100Guid:
				return "VenVt100()"
			case VT100PlusGuid:
				return "VenVt100Plus()"
			case VTUTF8Guid:
				return "VenUtf8()"
			case DebugPortProtocolGuid:
				return "DebugPort()"
			case DevicePathMessagingUARTFlowControl:
				if str, ok := n.uartFlowControlString(); ok {
					return str
				}
			case SASDevicePathGuid:
				if str, ok := n.sasString(); ok {
					return str
				}
			}
		}
	case MediaDevicePath:
		nodeType = "Media"
	default:
		nodeType = "?"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Ven%s(%s", nodeType, n.GUID)
	if len(n.Data) > 0 {
		fmt.Fprintf(&b, ",%x", n.Data)
	}
	b.WriteString(")")

	return b.String()
}

// String implement [fmt.Stringer].
func (n *VendorDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *VendorDevicePathNode) Write(w io.Writer) error {
	if !n.CompoundType().IsValid() {
		return errors.New("invalid device path type")
	}

	node := uefi.VENDOR_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		Guid: uefi.EFI_GUID(n.GUID)}
	nodeSz := binary.Size(node)

	dataSz := len(n.Data)
	if dataSz > math.MaxUint16-nodeSz {
		return errors.New("Data too large")
	}
	node.Header.Length = uint16(nodeSz + dataSz)

	if err := binary.Write(w, binary.LittleEndian, &node); err != nil {
		return err
	}
	_, err := w.Write(n.Data)
	return err
}

func (n *VendorDevicePathNode) uartFlowControlString() (string, bool) {
	var buf bytes.Buffer
	if err := n.Write(&buf); err != nil {
		return "", false
	}

	var ufc uefi.UART_FLOW_CONTROL_DEVICE_PATH
	if err := binary.Read(&buf, binary.LittleEndian, &ufc); err != nil {
		return "", false
	}

	var value string
	switch ufc.FlowControlMap & 3 {
	case 0:
		value = "None"
	case 1:
		value = "Hardware"
	case 2:
		value = "XonXoff"
	default:
		value = "0x3"
	}

	return fmt.Sprintf("UartFlowControl(%s)", value), true
}

func (n *VendorDevicePathNode) sasString() (string, bool) {
	var buf bytes.Buffer
	if err := n.Write(&buf); err != nil {
		return "", false
	}

	var sas uefi.SAS_DEVICE_PATH
	if err := binary.Read(&buf, binary.LittleEndian, &sas); err != nil {
		return "", false
	}

	var b strings.Builder
	fmt.Fprintf(&b, "SAS(%#x,%#x,%#x,", sas.SasAddress, sas.Lun, sas.RelativeTargetPort)

	info := sas.DeviceTopology
	switch {
	case info&0xf == 0 && info&0x80 == 0:
		b.WriteString("NoTopology,0,0,0,")
	case info&0xf <= 2 && info&0x80 == 0:
		var sasOrSata string
		switch {
		case info&0x10 == 0:
			sasOrSata = "SAS"
		default:
			sasOrSata = "SATA"
		}

		var location string
		switch {
		case info&0x20 == 0:
			location = "Internal"
		default:
			location = "External"
		}

		var connect string
		switch {
		case info&0x40 == 0:
			connect = "Direct"
		default:
			connect = "Expanded"
		}

		fmt.Fprintf(&b, "%s,%s,%s,", sasOrSata, location, connect)

		switch {
		case info&0xf == 1:
			b.WriteString("0,") // DriveBay
		default:
			fmt.Fprintf(&b, "%#x,", ((info>>8)&0xff)+1) // DriveBay
		}
	default:
		fmt.Fprintf(&b, "%#x,0,0,0,", info)
	}

	fmt.Fprintf(&b, "%#x)", sas.Reserved)

	return b.String(), true
}

func readVendorDevicePathNode(r io.Reader) (out *VendorDevicePathNode, err error) {
	var n uefi.VENDOR_DEVICE_PATH
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}

	out = &VendorDevicePathNode{
		Type: DevicePathNodeType(n.Header.Type),
		GUID: GUID(n.Guid),
	}
	if !out.CompoundType().IsValid() {
		panic("invalid device path type")
	}

	// The rest of the data from this io.Reader is for us.
	out.Data, _ = io.ReadAll(r)

	return out, nil
}

// EISAID represents a compressed EISA PNP ID
type EISAID uint32

// Vendor returns the 3-letter vendor ID.
func (id EISAID) Vendor() string {
	return fmt.Sprintf("%c%c%c",
		((id>>10)&0x1f)+'A'-1,
		((id>>5)&0x1f)+'A'-1,
		(id&0x1f)+'A'-1)
}

// Product returns the product ID.
func (id EISAID) Product() uint16 {
	return uint16(id >> 16)
}

// String implements [fmt.Stringer].
func (id EISAID) String() string {
	if id == 0 {
		return "0"
	}
	return fmt.Sprintf("%s%04x", id.Vendor(), id.Product())
}

func NewEISAID(vendor string, product uint16) (EISAID, error) {
	if len(vendor) != 3 {
		return 0, errors.New("invalid vendor length")
	}

	var out EISAID
	out |= EISAID((vendor[0]-'A'+1)&0x1f) << 10
	out |= EISAID((vendor[1]-'A'+1)&0x1f) << 5
	out |= EISAID((vendor[2] - 'A' + 1) & 0x1f)
	out |= EISAID(product) << 16

	return out, nil
}

// ACPIDevicePathNode corresponds to an ACPI device path node.
type ACPIDevicePathNode struct {
	HID EISAID // Compressed hardware ID
	UID uint32 // Unique ID
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *ACPIDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeACPIType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *ACPIDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *ACPIDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	if n.HID.Vendor() == "PNP" {
		switch n.HID.Product() {
		case 0x0a03:
			return fmt.Sprintf("PciRoot(%#x)", n.UID)
		case 0x0a08:
			return fmt.Sprintf("PcieRoot(%#x)", n.UID)
		case 0x0604:
			return fmt.Sprintf("Floppy(%#x)", n.UID)
		case 0x0301:
			return fmt.Sprintf("Keyboard(%#x)", n.UID)
		case 0x0501:
			return fmt.Sprintf("Serial(%#x)", n.UID)
		case 0x0401:
			return fmt.Sprintf("ParallelPort(%#x)", n.UID)
		}
	}
	return fmt.Sprintf("Acpi(%s,%#x)", n.HID, n.UID)
}

// String implements [fmt.Stringer].
func (n *ACPIDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *ACPIDevicePathNode) Write(w io.Writer) error {
	node := uefi.ACPI_HID_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		HID: uint32(n.HID),
		UID: uint32(n.UID)}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// ACPIExtendedDevicePathNode corresponds to an ACPI device path node
// and is used where a CID field is required or a string field is
// required for HID or UID.
type ACPIExtendedDevicePathNode struct {
	HID    EISAID
	UID    uint32
	CID    EISAID
	HIDStr string
	UIDStr string
	CIDStr string
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *ACPIExtendedDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeACPIExtendedType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *ACPIExtendedDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *ACPIExtendedDevicePathNode) ToString(flags DevicePathToStringFlags) string {
	switch {
	case flags.DisplayOnly() && n.HID.Vendor() == "PNP" && (n.HID.Product() == 0x0a03 || (n.CID.Product() == 0x0a03 && n.HID.Product() != 0x0a08)):
		if n.UIDStr != "" {
			return fmt.Sprintf("PciRoot(%s)", n.UIDStr)
		}
		return fmt.Sprintf("PciRoot(%#x)", n.UID)
	case flags.DisplayOnly() && n.HID.Vendor() == "PNP" && (n.HID.Product() == 0x0a08 || n.CID.Product() == 0x0a08):
		if n.UIDStr != "" {
			return fmt.Sprintf("PcieRoot(%s)", n.UIDStr)
		}
		return fmt.Sprintf("PcieRoot(%#x)", n.UID)
	case n.HIDStr == "" && n.CIDStr == "" && n.UIDStr != "":
		return fmt.Sprintf("AcpiExp(%s,%s,%s)", n.HID, n.CID, n.UIDStr)
	}

	if !flags.DisplayOnly() {
		hidStr := n.HIDStr
		if hidStr == "" {
			hidStr = "<nil>"
		}
		cidStr := n.CIDStr
		if cidStr == "" {
			cidStr = "<nil>"
		}
		uidStr := n.UIDStr
		if uidStr == "" {
			uidStr = "<nil>"
		}

		return fmt.Sprintf("AcpiEx(%s,%s,%#x,%s,%s,%s)", n.HID, n.CID, n.UID, hidStr, cidStr, uidStr)
	}

	hidText := n.HID.String()
	if n.HIDStr != "" {
		hidText = n.HIDStr
	}
	cidText := n.CID.String()
	if n.CIDStr != "" {
		cidText = n.CIDStr
	}

	if n.UIDStr != "" {
		return fmt.Sprintf("AcpiEx(%s,%s,%s)", hidText, cidText, n.UIDStr)
	}
	return fmt.Sprintf("AcpiEx(%s,%s,%#x)", hidText, cidText, n.UID)
}

// String implements [fmt.Stringer].
func (n *ACPIExtendedDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *ACPIExtendedDevicePathNode) Write(w io.Writer) error {
	node := uefi.ACPI_EXTENDED_HID_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		HID: uint32(n.HID),
		UID: n.UID,
		CID: uint32(n.CID)}

	// Set a reasonable limit on each string field
	length := binary.Size(node) + 3 // extra 3 bytes for NULL terminators

	for _, s := range []string{n.HIDStr, n.UIDStr, n.CIDStr} {
		sz := len(s)
		if sz > math.MaxUint16-length {
			return errors.New("string fields too large")
		}
		length += sz
	}
	node.Header.Length = uint16(length)

	if err := binary.Write(w, binary.LittleEndian, &node); err != nil {
		return err
	}
	for _, s := range []string{n.HIDStr, n.UIDStr, n.CIDStr} {
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
		w.Write([]byte{0x00})
	}

	return nil
}

// ATAPIControllerRole describes the port that an IDE device is connected to.
type ATAPIControllerRole uint8

// String implements [fmt.Stringer].
func (r ATAPIControllerRole) String() string {
	switch r {
	case ATAPIControllerPrimary:
		return "Primary"
	case ATAPIControllerSecondary:
		return "Secondary"
	default:
		return strconv.FormatUint(uint64(r), 10)
	}
}

const (
	ATAPIControllerPrimary   ATAPIControllerRole = 0
	ATAPIControllerSecondary ATAPIControllerRole = 1
)

// ATAPIDriveRole describes the role of a device on a specific IDE port.
type ATAPIDriveRole uint8

// String implements [fmt.Stringer].
func (r ATAPIDriveRole) String() string {
	switch r {
	case ATAPIDriveMaster:
		return "Master"
	case ATAPIDriveSlave:
		return "Slave"
	default:
		return strconv.FormatUint(uint64(r), 10)
	}
}

const (
	ATAPIDriveMaster ATAPIDriveRole = 0
	ATAPIDriveSlave  ATAPIDriveRole = 1
)

// ATAPIDevicePathNode corresponds to an ATA device path node.
type ATAPIDevicePathNode struct {
	Controller ATAPIControllerRole
	Drive      ATAPIDriveRole
	LUN        uint16 // Logical unit number
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *ATAPIDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeATAPIType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *ATAPIDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *ATAPIDevicePathNode) ToString(flags DevicePathToStringFlags) string {
	if flags.DisplayOnly() {
		return fmt.Sprintf("Ata(%#x)", n.LUN)
	}
	return fmt.Sprintf("Ata(%s,%s,%#x)", n.Controller, n.Drive, n.LUN)
}

// String implements [fmt.Stringer].
func (n *ATAPIDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *ATAPIDevicePathNode) Write(w io.Writer) error {
	node := uefi.ATAPI_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		PrimarySecondary: uint8(n.Controller),
		SlaveMaster:      uint8(n.Drive),
		Lun:              n.LUN}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// SCSIDevicePathNode corresponds to a SCSI device path node.
type SCSIDevicePathNode struct {
	PUN uint16 // Target ID on the SCSI bus
	LUN uint16 // Logical unit number
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *SCSIDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeSCSIType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *SCSIDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *SCSIDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return fmt.Sprintf("Scsi(%#x,%#x)", n.PUN, n.LUN)
}

// String implements [fmt.Stringer].
func (n *SCSIDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *SCSIDevicePathNode) Write(w io.Writer) error {
	node := uefi.SCSI_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		Pun: n.PUN,
		Lun: n.LUN}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// USBDevicePathNode corresponds to a USB device path node.
type USBDevicePathNode struct {
	ParentPortNumber uint8
	InterfaceNumber  uint8
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *USBDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeUSBType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *USBDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *USBDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return fmt.Sprintf("USB(%#x,%#x)", n.ParentPortNumber, n.InterfaceNumber)
}

// String implements [fmt.Stringer].
func (n *USBDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *USBDevicePathNode) Write(w io.Writer) error {
	node := uefi.USB_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		ParentPortNumber: n.ParentPortNumber,
		InterfaceNumber:  n.InterfaceNumber}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// USBClass defines the base class of a USB interface.
type USBClass uint8

const (
	USBClassAudio       USBClass = 0x01
	USBClassCDCControl  USBClass = 0x02
	USBClassHID         USBClass = 0x03
	USBClassImage       USBClass = 0x06
	USBClassPrinter     USBClass = 0x07
	USBClassMassStorage USBClass = 0x08
	USBClassHub         USBClass = 0x09
	USBClassCDCData     USBClass = 0x0a
	USBClassSmartCard   USBClass = 0x0b
	USBClassVideo       USBClass = 0x0e
	USBClassDiagnostic  USBClass = 0xdc
	USBClassWireless    USBClass = 0xe0
	USBClassAppSpecific USBClass = 0xfe
)

// String implements [fmt.Stringer].
func (c USBClass) String() string {
	switch c {
	case USBClassAudio:
		return "UsbAudio"
	case USBClassCDCControl:
		return "UsbCDCControl"
	case USBClassHID:
		return "UsbHID"
	case USBClassImage:
		return "UsbImage"
	case USBClassPrinter:
		return "UsbPrinter"
	case USBClassMassStorage:
		return "UsbMassStorage"
	case USBClassHub:
		return "UsbHub"
	case USBClassCDCData:
		return "UsbCDCData"
	case USBClassSmartCard:
		return "UsbSmartCard"
	case USBClassVideo:
		return "UsbVideo"
	case USBClassDiagnostic:
		return "UsbDiagnostic"
	case USBClassWireless:
		return "UsbWireless"
	case USBClassAppSpecific:
		return "UsbAppSpecific"
	default:
		return fmt.Sprintf("%#x", uint8(c))
	}
}

// USBSubClass defines the sub class of a USB interface. The meaning of
// this is dependent on the value of [USBClass].
type USBSubClass uint8

const (
	USBSubClassAppSpecificFWUpdate           USBSubClass = 0x01
	USBSubClassAppSpecificIRDABridge         USBSubClass = 0x02
	USBSubClassAppSpecificTestAndMeasurement USBSubClass = 0x03
)

// USBProtocol defines the protocol of a USB interface. The meaning of
// this is dependent on the values of [USBClass] and [USBSubClass].
type USBProtocol uint8

// USBClassDevicePathNode corresponds to a USB class device path node.
type USBClassDevicePathNode struct {
	VendorId       uint16
	ProductId      uint16
	DeviceClass    USBClass
	DeviceSubClass USBSubClass
	DeviceProtocol USBProtocol
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *USBClassDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeUSBClassType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *USBClassDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *USBClassDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	switch n.DeviceClass {
	case USBClassAudio, USBClassCDCControl, USBClassHID, USBClassImage,
		USBClassPrinter, USBClassMassStorage, USBClassHub, USBClassCDCData,
		USBClassSmartCard, USBClassVideo, USBClassDiagnostic, USBClassWireless:
		return fmt.Sprintf("%s(%#x,%#x,%#x,%#x)", n.DeviceClass, n.VendorId, n.ProductId, n.DeviceSubClass, n.DeviceProtocol)
	case USBClassAppSpecific:
		switch n.DeviceSubClass {
		case USBSubClassAppSpecificFWUpdate:
			return fmt.Sprintf("UsbDeviceFirmwareUpdate(%#x,%#x,%#x)", n.VendorId, n.ProductId, n.DeviceProtocol)
		case USBSubClassAppSpecificIRDABridge:
			return fmt.Sprintf("UsbIrdaBridge(%#x,%#x,%#x)", n.VendorId, n.ProductId, n.DeviceProtocol)
		case USBSubClassAppSpecificTestAndMeasurement:
			return fmt.Sprintf("UsbTestAndMeasurement(%#x,%#x,%#x)", n.VendorId, n.ProductId, n.DeviceProtocol)
		}
	}

	return fmt.Sprintf("UsbClass(%#x,%#x,%#x,%#x,%#x)", n.VendorId, n.ProductId, n.DeviceClass, n.DeviceSubClass, n.DeviceProtocol)
}

// String implements [fmt.Stringer].
func (n *USBClassDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *USBClassDevicePathNode) Write(w io.Writer) error {
	node := uefi.USB_CLASS_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		VendorId:       n.VendorId,
		ProductId:      n.ProductId,
		DeviceClass:    uint8(n.DeviceClass),
		DeviceSubClass: uint8(n.DeviceSubClass),
		DeviceProtocol: uint8(n.DeviceProtocol)}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// MACAddrDevicePathNode corresponds to a MAC address device path node.
type MACAddrDevicePathNode struct {
	MACAddress MACAddress
	IfType     NetworkInterfaceType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *MACAddrDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *MACAddrDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeMACAddrType
}

// ToString implements [DevicePathNode.ToString].
func (n *MACAddrDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	var addr [32]uint8
	if n.MACAddress != nil {
		addr = n.MACAddress.Bytes32()
	}

	sz := unsafe.Sizeof(addr)
	if n.IfType == NetworkInterfaceTypeReserved || n.IfType == NetworkInterfaceTypeEthernet {
		sz = 6
	}

	return fmt.Sprintf("MacAddr(%02x,%#x)", addr[:sz], n.IfType)
}

// String implements [fmt.Stringer].
func (n *MACAddrDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *MACAddrDevicePathNode) Write(w io.Writer) error {
	node := uefi.MAC_ADDR_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType()),
		},
		IfType: uint8(n.IfType),
	}
	node.Header.Length = uint16(binary.Size(node))

	if n.MACAddress != nil {
		node.MacAddress = uefi.EFI_MAC_ADDRESS{
			Addr: n.MACAddress.Bytes32(),
		}
	}

	return binary.Write(w, binary.LittleEndian, &node)
}

// IPv4DevicePathNode corresponds to an IPv4 address device path node.
type IPv4DevicePathNode struct {
	LocalAddress       IPv4Address       // Local IP address
	RemoteAddress      IPv4Address       // Remote IP address
	LocalPort          uint16            // Local port (if Protocol is IPProtocolTCP or IPProtocolUDP)
	RemotePort         uint16            // Remote port (if Protocol is IPProtocolTCP or IPProtocolUDP)
	Protocol           IPProtocol        // IP protocol
	LocalAddressOrigin IPv4AddressOrigin // The origin of the local address (static or DHCP)
	GatewayAddress     IPv4Address       // Gateway IP address
	SubnetMask         IPv4Address       // Used to determine the range of IP addresses on the subnet associated with LocalAddress
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *IPv4DevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeIPv4Type
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *IPv4DevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *IPv4DevicePathNode) ToString(flags DevicePathToStringFlags) string {
	var b strings.Builder
	fmt.Fprintf(&b, "IPv4(%s", n.RemoteAddress)

	if flags.DisplayOnly() {
		b.WriteString(")")
		return b.String()
	}

	fmt.Fprintf(&b, ",%s,%s,%s,%s,%s)", n.Protocol, n.LocalAddressOrigin, n.LocalAddress, n.GatewayAddress, n.SubnetMask)
	return b.String()
}

// String implements [fmt.Stringer].
func (n *IPv4DevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *IPv4DevicePathNode) Write(w io.Writer) error {
	node := uefi.IPv4_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType()),
		},
		LocalIpAddress:   uefi.EFI_IPv4_ADDRESS{Addr: [4]uint8(n.LocalAddress)},
		RemoteIpAddress:  uefi.EFI_IPv4_ADDRESS{Addr: [4]uint8(n.RemoteAddress)},
		LocalPort:        n.LocalPort,
		RemotePort:       n.RemotePort,
		Protocol:         uint16(n.Protocol),
		StaticIpAddress:  bool(n.LocalAddressOrigin),
		GatewayIpAddress: uefi.EFI_IPv4_ADDRESS{Addr: [4]uint8(n.GatewayAddress)},
		SubnetMask:       uefi.EFI_IPv4_ADDRESS{Addr: [4]uint8(n.SubnetMask)},
	}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// IPv6DevicePathNode corresponds to an IPv6 address device path node.
type IPv6DevicePathNode struct {
	LocalAddress       IPv6Address       // Local IP address
	RemoteAddress      IPv6Address       // Remote IP address
	LocalPort          uint16            // Local port (if Protocol is IPProtocolTCP or IPProtocolUDP)
	RemotePort         uint16            // Remote port (if Protocol is IPProtocolTCP or IPProtocolUDP)
	Protocol           IPProtocol        // IP protocol
	LocalAddressOrigin IPv6AddressOrigin // The origin of the local address (static, SLAAC, or DHCPv6)
	PrefixLength       uint8             // Prefix length in bits - used to determine the range of IP addresses on the subnet associated with LocalAddress
	GatewayAddress     IPv6Address       // Gateway IP address
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *IPv6DevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeIPv6Type
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *IPv6DevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *IPv6DevicePathNode) ToString(flags DevicePathToStringFlags) string {
	var b strings.Builder
	fmt.Fprintf(&b, "IPv6(%s", n.RemoteAddress)

	if flags.DisplayOnly() {
		b.WriteString(")")
		return b.String()
	}

	fmt.Fprintf(&b, ",%s,%s,%s,%#x,%s)", n.Protocol, n.LocalAddressOrigin, n.LocalAddress, n.PrefixLength, n.GatewayAddress)
	return b.String()
}

// String implements [fmt.Stringer].
func (n *IPv6DevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *IPv6DevicePathNode) Write(w io.Writer) error {
	node := uefi.IPv6_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType()),
		},
		LocalIpAddress:   uefi.EFI_IPv6_ADDRESS{Addr: [16]uint8(n.LocalAddress)},
		RemoteIpAddress:  uefi.EFI_IPv6_ADDRESS{Addr: [16]uint8(n.RemoteAddress)},
		LocalPort:        n.LocalPort,
		RemotePort:       n.RemotePort,
		Protocol:         uint16(n.Protocol),
		IpAddressOrigin:  uint8(n.LocalAddressOrigin),
		PrefixLength:     n.PrefixLength,
		GatewayIpAddress: uefi.EFI_IPv6_ADDRESS{Addr: [16]uint8(n.GatewayAddress)},
	}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// USBWWIDDevicePathNode corresponds to a USB WWID device path node.
type USBWWIDDevicePathNode struct {
	InterfaceNumber uint16
	VendorId        uint16
	ProductId       uint16
	SerialNumber    string
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *USBWWIDDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeUSBWWIDType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *USBWWIDDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *USBWWIDDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return fmt.Sprintf("UsbWwid(%#x,%#x,%#x,\"%s\"", n.VendorId, n.ProductId, n.InterfaceNumber, n.SerialNumber)
}

// String implements [fmt.Stringer].
func (n *USBWWIDDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *USBWWIDDevicePathNode) Write(w io.Writer) error {
	node := uefi.USB_WWID_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		InterfaceNumber: n.InterfaceNumber,
		VendorId:        n.VendorId,
		ProductId:       n.ProductId,
		SerialNumber:    ConvertUTF8ToUTF16(n.SerialNumber)}

	length := binary.Size(node.Header) + int(unsafe.Sizeof(node.InterfaceNumber)+unsafe.Sizeof(node.VendorId)+unsafe.Sizeof(node.ProductId))
	serialNumSz := binary.Size(node.SerialNumber)
	if serialNumSz > math.MaxUint16-length {
		return errors.New("SerialNumber too long")
	}
	node.Header.Length = uint16(length + serialNumSz)

	return binary.Write(w, binary.LittleEndian, &node)
}

type DeviceLogicalUnitDevicePathNode struct {
	LUN uint8
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *DeviceLogicalUnitDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeDeviceLogicalUnitType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *DeviceLogicalUnitDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *DeviceLogicalUnitDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return fmt.Sprintf("Unit(%#x)", n.LUN)
}

// String implements [fmt.Stringer].
func (n *DeviceLogicalUnitDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *DeviceLogicalUnitDevicePathNode) Write(w io.Writer) error {
	node := uefi.DEVICE_LOGICAL_UNIT_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		Lun: n.LUN}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// SATADevicePathNode corresponds to a SATA device path node.
type SATADevicePathNode struct {
	HBAPortNumber            uint16 // The zero indexed port number on the HBA
	PortMultiplierPortNumber uint16 // The port multiplier (or 0xFFFF if the device is connected directly to the HBA)
	LUN                      uint16 // Logical unit number
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *SATADevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeSATAType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *SATADevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *SATADevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return fmt.Sprintf("Sata(%#x,%#x,%#x)", n.HBAPortNumber, n.PortMultiplierPortNumber, n.LUN)
}

// String implements [fmt.Stringer].
func (n *SATADevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *SATADevicePathNode) Write(w io.Writer) error {
	node := uefi.SATA_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		HBAPortNumber:            n.HBAPortNumber,
		PortMultiplierPortNumber: n.PortMultiplierPortNumber,
		Lun:                      n.LUN}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// NVMENamespaceDevicePathNode corresponds to a NVME namespace device path node.
type NVMENamespaceDevicePathNode struct {
	NamespaceID   uint32 // Namespace identifier
	NamespaceUUID EUI64  // EUI-64 unique identifier. This is set to 0 where not supported
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *NVMENamespaceDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeNVMENamespaceType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *NVMENamespaceDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *NVMENamespaceDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return fmt.Sprintf("NVMe(%#x,%s)", n.NamespaceID, n.NamespaceUUID)
}

// String implements [fmt.Stringer].
func (n *NVMENamespaceDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *NVMENamespaceDevicePathNode) Write(w io.Writer) error {
	// Convert the UUID back from EUI64 to uint64, big-endian.
	uuid := binary.BigEndian.Uint64(n.NamespaceUUID[:])
	node := uefi.NVME_NAMESPACE_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		NamespaceId:   n.NamespaceID,
		NamespaceUuid: uuid}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// MBRType describes a disk header type
type MBRType uint8

// String implements [fmt.Stringer].
func (t MBRType) String() string {
	switch t {
	case LegacyMBR:
		return "MBR"
	case GPT:
		return "GPT"
	default:
		return strconv.FormatUint(uint64(t), 10)
	}
}

const (
	// LegacyMBR indicates that a disk has a MBR header.
	LegacyMBR MBRType = uefi.MBR_TYPE_PCAT

	// GPT indicates that a disk has a GPT header.
	GPT MBRType = uefi.MBR_TYPE_EFI_PARTITION_TABLE_HEADER
)

// HardDriveSignatureType describes the type of unique identifier associated
// with a hard drive.
type HardDriveSignatureType uint8

const (
	// NoHardDriveSignature indicates there is no signature. This can
	// be represented by the value EmptyHardDriveSignature.
	NoHardDriveSignature HardDriveSignatureType = uefi.NO_DISK_SIGNATURE

	// HardDriveSignatureTypeMBR indicates that the unique identifier
	// is a MBR unique signature. This is represented by the
	// MBRHardDriveSignature type.
	HardDriveSignatureTypeMBR HardDriveSignatureType = uefi.SIGNATURE_TYPE_MBR

	// HardDriveSignatureTypeGUID indicates that the unique identifier
	// is a GUID. This is represented by the GUIDHardDriveSignature type.
	HardDriveSignatureTypeGUID HardDriveSignatureType = uefi.SIGNATURE_TYPE_GUID
)

// String implements [fmt.Stringer].
func (t HardDriveSignatureType) String() string {
	switch t {
	case HardDriveSignatureTypeMBR:
		return "MBR"
	case HardDriveSignatureTypeGUID:
		return "GPT"
	default:
		return strconv.FormatUint(uint64(t), 10)
	}
}

// HardDriveSignature is an abstraction for a unique hard drive identifier.
type HardDriveSignature interface {
	fmt.Stringer
	Data() [16]uint8              // the raw signature data
	Type() HardDriveSignatureType // Signature type
}

type emptyHardDriveSignatureType struct{}

func (emptyHardDriveSignatureType) String() string {
	return ""
}

func (emptyHardDriveSignatureType) Data() [16]uint8 {
	var emptySignature [16]uint8
	return emptySignature
}

func (emptyHardDriveSignatureType) Type() HardDriveSignatureType {
	return NoHardDriveSignature
}

// EmptyHardDriveSignature is an empty [HardDriveSignature].
var EmptyHardDriveSignature = emptyHardDriveSignatureType{}

// GUIDHardDriveSignature is a [HardDriveSignature] for GPT drives.
type GUIDHardDriveSignature GUID

// String implements [fmt.Stringer].
func (s GUIDHardDriveSignature) String() string {
	return GUID(s).String()
}

// Data implements [HardDriveSignature.Data].
func (s GUIDHardDriveSignature) Data() (out [16]uint8) {
	copy(out[:], s[:])
	return out
}

// Type implements [HardDriveSignature.Type].
func (GUIDHardDriveSignature) Type() HardDriveSignatureType {
	return HardDriveSignatureTypeGUID
}

// MBRHardDriveSignature is a [HardDriveSignature] for legacy MBR drives.
type MBRHardDriveSignature uint32

// String implements [fmt.Stringer].
func (s MBRHardDriveSignature) String() string {
	return fmt.Sprintf("%#08x", uint32(s))
}

// Data implements [HardDriveSignature.Data].
func (s MBRHardDriveSignature) Data() (out [16]uint8) {
	binary.LittleEndian.PutUint32(out[:], uint32(s))
	return out
}

// Type implements [HardDriveSignature.Type].
func (s MBRHardDriveSignature) Type() HardDriveSignatureType {
	return HardDriveSignatureTypeMBR
}

type unknownHardDriveSignature struct {
	typ  HardDriveSignatureType
	data [16]uint8
}

func (s *unknownHardDriveSignature) String() string {
	return fmt.Sprintf("%x", s.data)
}

func (s *unknownHardDriveSignature) Data() [16]uint8 {
	return s.data
}

func (s *unknownHardDriveSignature) Type() HardDriveSignatureType {
	return s.typ
}

// HardDriveDevicePathNode corresponds to a hard drive device path node.
type HardDriveDevicePathNode struct {
	PartitionNumber uint32             // 1-indexed partition number
	PartitionStart  uint64             // Starting LBA
	PartitionSize   uint64             // Size in number of LBAs
	Signature       HardDriveSignature // Signature, the type of which is implementation specific (GPT vs MBR)
	MBRType         MBRType            // Legacy MBR or GPT
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *HardDriveDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeHardDriveType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *HardDriveDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *HardDriveDevicePathNode) ToString(flags DevicePathToStringFlags) string {
	var b strings.Builder

	signature := n.Signature
	if signature == nil {
		signature = EmptyHardDriveSignature
	}

	fmt.Fprintf(&b, "HD(%d,%s,", n.PartitionNumber, signature.Type())
	switch signature.Type() {
	case NoHardDriveSignature:
		b.WriteString("0")
	case HardDriveSignatureTypeMBR, HardDriveSignatureTypeGUID:
		fmt.Fprintf(&b, "%s", signature)
	default:
		fmt.Fprintf(&b, "%x", signature.Data())
	}

	if !flags.DisplayOnly() {
		fmt.Fprintf(&b, ",%#x,%#x", n.PartitionStart, n.PartitionSize)
	}
	b.WriteString(")")

	return b.String()
}

// ToString implements [DevicePathNode.ToString].
func (n *HardDriveDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *HardDriveDevicePathNode) Write(w io.Writer) error {
	node := uefi.HARDDRIVE_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		PartitionNumber: n.PartitionNumber,
		PartitionStart:  n.PartitionStart,
		PartitionSize:   n.PartitionSize,
		MBRType:         uint8(n.MBRType)}
	node.Header.Length = uint16(binary.Size(node))

	signature := n.Signature
	if signature == nil {
		signature = EmptyHardDriveSignature
	}
	node.SignatureType = uint8(signature.Type())

	if signature.Type() == NoHardDriveSignature && signature.Data() != EmptyHardDriveSignature.Data() {
		return errors.New("inconsistent signature and signature type: expected empty signature for NoHardDriveSignature signature type")
	}
	node.Signature = signature.Data()

	return binary.Write(w, binary.LittleEndian, &node)
}

// NewHardDriveDevicePathNodeFromDevice constructs a HardDriveDevicePathNode for the
// specified partition on the supplied device reader. The device's total size and
// logical block size must be supplied.
func NewHardDriveDevicePathNodeFromDevice(r io.ReaderAt, totalSz, blockSz int64, part int) (*HardDriveDevicePathNode, error) {
	if part < 1 {
		return nil, errors.New("invalid partition number")
	}

	table, err := ReadPartitionTable(r, totalSz, blockSz, PrimaryPartitionTable, true)
	switch {
	case err == ErrStandardMBRFound:
		record, err := mbr.ReadRecord(io.NewSectionReader(r, 0, totalSz))
		if err != nil {
			return nil, err
		}
		if part > 4 {
			return nil, fmt.Errorf("invalid partition number %d for MBR", part)
		}

		entry := record.Partitions[part-1]

		return &HardDriveDevicePathNode{
			PartitionNumber: uint32(part),
			PartitionStart:  uint64(entry.StartingLBA),
			PartitionSize:   uint64(entry.NumberOfSectors),
			Signature:       MBRHardDriveSignature(record.UniqueSignature),
			MBRType:         LegacyMBR}, nil
	case err != nil:
		return nil, err
	default:
		if part > len(table.Entries) {
			return nil, fmt.Errorf("invalid partition number %d: device only has %d partitions", part, len(table.Entries))
		}

		entry := table.Entries[part-1]

		if entry.PartitionTypeGUID == UnusedPartitionType {
			return nil, errors.New("requested partition is unused")
		}

		return &HardDriveDevicePathNode{
			PartitionNumber: uint32(part),
			PartitionStart:  uint64(entry.StartingLBA),
			PartitionSize:   uint64(entry.EndingLBA - entry.StartingLBA + 1),
			Signature:       GUIDHardDriveSignature(entry.UniquePartitionGUID),
			MBRType:         GPT}, nil
	}
}

// CDROMDevicePathNode corresponds to a CDROM device path node.
type CDROMDevicePathNode struct {
	BootEntry      uint32
	PartitionStart uint64
	PartitionSize  uint64
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *CDROMDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeCDROMType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *CDROMDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *CDROMDevicePathNode) ToString(flags DevicePathToStringFlags) string {
	if flags.DisplayOnly() {
		return fmt.Sprintf("CDROM(%#x)", n.BootEntry)
	}
	return fmt.Sprintf("CDROM(%#x,%#x,%#x)", n.BootEntry, n.PartitionStart, n.PartitionSize)
}

// String implements [fmt.Stringer].
func (n *CDROMDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *CDROMDevicePathNode) Write(w io.Writer) error {
	node := uefi.CDROM_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		BootEntry:      n.BootEntry,
		PartitionStart: n.PartitionStart,
		PartitionSize:  n.PartitionSize}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

// FilePathDevicePathNode corresponds to a file path device path node.
type FilePathDevicePathNode string

// CompoundType implements [DevicePathNode.CompoundType].
func (n FilePathDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeFilePathType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n FilePathDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n FilePathDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return string(n)
}

// String implements [fmt.Stringer].
func (n FilePathDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n FilePathDevicePathNode) Write(w io.Writer) error {
	node := uefi.FILEPATH_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		PathName: ConvertUTF8ToUTF16(string(n) + "\x00")}

	length := int(binary.Size(node.Header))
	pathSz := binary.Size(node.PathName)
	if pathSz > math.MaxUint16-length {
		return errors.New("PathName too large")
	}
	node.Header.Length = uint16(length + pathSz)

	return node.Write(w)
}

// NewFilePathDevicePathNode constructs a new FilePathDevicePathNode from the supplied
// path, converting the OS native separators to EFI separators ("\") and prepending
// a separator to the start of the path if one doesn't already exist.
func NewFilePathDevicePathNode(path string) (out FilePathDevicePathNode) {
	components := strings.Split(path, string(os.PathSeparator))
	if !filepath.IsAbs(path) {
		out = FilePathDevicePathNode("\\")
	}
	return out + FilePathDevicePathNode(strings.Join(components, "\\"))
}

// MediaFvFileDevicePathNode corresponds to a firmware volume file device path node.
//
// Deprecated: use [FWFileDevicePathNode].
type MediaFvFileDevicePathNode = FWFileDevicePathNode

// FWFileDevicePathNode corresponds to a firmware volume file device path node.
type FWFileDevicePathNode GUID

// CompoundType implements [DevicePathNode.CompoundType].
func (n FWFileDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeFwFileType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n FWFileDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n FWFileDevicePathNode) ToString(flags DevicePathToStringFlags) string {
	if flags.DisplayFWGUIDNames() {
		fvFileNameLookupMu.Lock()
		defer fvFileNameLookupMu.Unlock()

		if fvFileNameLookup != nil {
			name, known := fvFileNameLookup(GUID(n))
			if known {
				return fmt.Sprintf("FvFile(%s)", name)
			}
		}
	}
	return fmt.Sprintf("FvFile(%s)", GUID(n))
}

// String implements [fmt.Stringer].
func (n FWFileDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n FWFileDevicePathNode) Write(w io.Writer) error {
	node := uefi.MEDIA_FW_VOL_FILEPATH_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		FvFileName: uefi.EFI_GUID(n)}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

var (
	fvFileNameLookupMu sync.Mutex
	fvFileNameLookup   func(GUID) (string, bool)
)

// RegisterMediaFvFileNameLookup registers a function that can map guids to
// strings for well known names, and these will be displayed by
// [FWFileDevicePathNode.String] and [FWFileDevicePathNode.ToString]
// if the flags argument is marked as display only. Note that this does make the
// string representation of the path unparseable, if the string is being used
// in such a way (this package doesn't yet have any ways of parsing device paths
// that are in string form).
//
// Just importing [github.com/canonical/go-efilib/guids] is sufficient to register
// a function that does this. It's included in a separate and optional package for
// systems that are concerned about binary size.
//
// Deprecated: use [RegisterFWFileNameLookup].
func RegisterMediaFvFileNameLookup(fn func(GUID) (string, bool)) {
	RegisterFWFileNameLookup(fn)
}

// RegisterFWFileNameLookup registers a function that can map guids to
// strings for well known names, and these will be displayed by
// [FWFileDevicePathNode.String] and [FWFileDevicePathNode.ToString]
// if the flags argument is marked as display only. Note that this does make the
// string representation of the path unparseable, if the string is being used
// in such a way (this package doesn't yet have any ways of parsing device paths
// that are in string form).
//
// Just importing [github.com/canonical/go-efilib/guids] is sufficient to register
// a function that does this. It's included in a separate and optional package for
// systems that are concerned about binary size.
func RegisterFWFileNameLookup(fn func(GUID) (string, bool)) {
	fvFileNameLookupMu.Lock()
	defer fvFileNameLookupMu.Unlock()

	fvFileNameLookup = fn
}

// MediaFvDevicePathNode corresponds to a firmware volume device path node.
//
// Deprecated: use [FWVolDevicePathNode].
type MediaFvDevicePathNode = FWVolDevicePathNode

// FWVolDevicePathNode corresponds to a firmware volume device path node.
type FWVolDevicePathNode GUID

// CompoundType implements [DevicePathNode.CompoundType].
func (n FWVolDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeFwVolType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n FWVolDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n FWVolDevicePathNode) ToString(flags DevicePathToStringFlags) string {
	if flags.DisplayFWGUIDNames() {
		fvNameLookupMu.Lock()
		defer fvNameLookupMu.Unlock()

		if fvNameLookup != nil {
			name, known := fvNameLookup(GUID(n))
			if known {
				return fmt.Sprintf("Fv(%s)", name)
			}
		}
	}
	return fmt.Sprintf("Fv(%s)", GUID(n))
}

// String implements [fmt.Stringer].
func (n FWVolDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n FWVolDevicePathNode) Write(w io.Writer) error {
	node := uefi.MEDIA_FW_VOL_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		FvName: uefi.EFI_GUID(n)}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

var (
	fvNameLookupMu sync.Mutex
	fvNameLookup   func(GUID) (string, bool)
)

// RegisterMediaFvNameLookup registers a function that can map guids to
// strings for well known names, and these will be displayed by
// [FWVolDevicePathNode.String] and [FWVolDevicePathNode.ToString]
// if the flags argument is marked as display only. Note that this does make the
// string representation of the path unparseable, if the string is being used
// in such a way (this package doesn't yet have any ways of parsing device paths
// that are in string form).
//
// Just importing [github.com/canonical/go-efilib/guids] is sufficient to register
// a function that does this. It's included in a separate and optional package for
// systems that are concerned about binary size.
//
// Deprecated: use [RegisterFWVolNameLookup].
func RegisterMediaFvNameLookup(fn func(GUID) (string, bool)) {
	RegisterFWVolNameLookup(fn)
}

// RegisterFWVolNameLookup registers a function that can map guids to
// strings for well known names, and these will be displayed by
// [FWVolDevicePathNode.String] and [FWVolDevicePathNode.ToString]
// if the flags argument is marked as display only. Note that this does make the
// string representation of the path unparseable, if the string is being used
// in such a way (this package doesn't yet have any ways of parsing device paths
// that are in string form).
//
// Just importing [github.com/canonical/go-efilib/guids] is sufficient to register
// a function that does this. It's included in a separate and optional package for
// systems that are concerned about binary size.
func RegisterFWVolNameLookup(fn func(GUID) (string, bool)) {
	fvNameLookupMu.Lock()
	defer fvNameLookupMu.Unlock()

	fvNameLookup = fn
}

type MediaRelOffsetRangeDevicePathNode struct {
	StartingOffset uint64
	EndingOffset   uint64
}

// CompoundType implements [DevicePathNode.CompoundType].
func (n *MediaRelOffsetRangeDevicePathNode) CompoundType() DevicePathNodeCompoundType {
	return DevicePathNodeRelativeOffsetRangeType
}

// AsGenericDevicePathNode implements [DevicePathNode.AsGenericDevicePathNode].
func (n *MediaRelOffsetRangeDevicePathNode) AsGenericDevicePathNode() (*GenericDevicePathNode, error) {
	return convertToGenericDevicePathNode(n)
}

// ToString implements [DevicePathNode.ToString].
func (n *MediaRelOffsetRangeDevicePathNode) ToString(_ DevicePathToStringFlags) string {
	return fmt.Sprintf("Offset(%#x,%#x)", n.StartingOffset, n.EndingOffset)
}

// String implements [fmt.Stringer].
func (n *MediaRelOffsetRangeDevicePathNode) String() string {
	return devicePathString(n)
}

// Write implements [DevicePathNode.Write].
func (n *MediaRelOffsetRangeDevicePathNode) Write(w io.Writer) error {
	node := uefi.MEDIA_RELATIVE_OFFSET_RANGE_DEVICE_PATH{
		Header: uefi.EFI_DEVICE_PATH_PROTOCOL{
			Type:    uint8(n.CompoundType().Type()),
			SubType: uint8(n.CompoundType().SubType())},
		StartingOffset: n.StartingOffset,
		EndingOffset:   n.EndingOffset}
	node.Header.Length = uint16(binary.Size(node))

	return binary.Write(w, binary.LittleEndian, &node)
}

func decodeDevicePathNode(r io.Reader) (out DevicePathNode, err error) {
	var hdrBuf bytes.Buffer
	hdrReader := io.TeeReader(r, &hdrBuf)

	var hdr uefi.EFI_DEVICE_PATH_PROTOCOL
	if err := binary.Read(hdrReader, binary.LittleEndian, &hdr); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	if hdr.Length < 4 {
		return nil, fmt.Errorf("invalid length %d bytes (too small)", hdr.Length)
	}

	dataReader := io.LimitReader(r, int64(hdr.Length-4))

	defer func() {
		switch {
		case err == io.EOF:
			fallthrough
		case errors.Is(err, io.ErrUnexpectedEOF):
			err = fmt.Errorf("invalid length %d bytes (too small)", hdr.Length)
		case err != nil:
			// Unexpected error should be returned untouched.
		case dataReader.(*io.LimitedReader).N > 0:
			err = fmt.Errorf("invalid length %d bytes (too large)", hdr.Length)
		}
	}()

	nodeReader := io.MultiReader(&hdrBuf, dataReader)

	switch hdr.Type {
	case uefi.HARDWARE_DEVICE_PATH:
		switch hdr.SubType {
		case uefi.HW_PCI_DP:
			var n uefi.PCI_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &PCIDevicePathNode{Function: n.Function, Device: n.Device}, nil
		case uefi.HW_VENDOR_DP:
			return readVendorDevicePathNode(nodeReader)
		}
	case uefi.ACPI_DEVICE_PATH:
		switch hdr.SubType {
		case uefi.ACPI_DP:
			var n uefi.ACPI_HID_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &ACPIDevicePathNode{HID: EISAID(n.HID), UID: n.UID}, nil
		case uefi.ACPI_EXTENDED_DP:
			var n uefi.ACPI_EXTENDED_HID_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			node := &ACPIExtendedDevicePathNode{HID: EISAID(n.HID), UID: n.UID, CID: EISAID(n.CID)}

			r := bufio.NewReader(nodeReader)
			for _, s := range []*string{&node.HIDStr, &node.UIDStr, &node.CIDStr} {
				v, err := r.ReadString('\x00')
				if err != nil {
					return nil, err
				}
				*s = v[:len(v)-1]
			}
			return node, nil
		}
	case uefi.MESSAGING_DEVICE_PATH:
		switch hdr.SubType {
		case uefi.MSG_ATAPI_DP:
			var n uefi.ATAPI_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &ATAPIDevicePathNode{
				Controller: ATAPIControllerRole(n.PrimarySecondary),
				Drive:      ATAPIDriveRole(n.SlaveMaster),
				LUN:        n.Lun}, nil
		case uefi.MSG_SCSI_DP:
			var n uefi.SCSI_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &SCSIDevicePathNode{PUN: n.Pun, LUN: n.Lun}, nil
		case uefi.MSG_USB_DP:
			var n uefi.USB_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &USBDevicePathNode{
				ParentPortNumber: n.ParentPortNumber,
				InterfaceNumber:  n.InterfaceNumber}, nil
		case uefi.MSG_USB_CLASS_DP:
			var n uefi.USB_CLASS_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &USBClassDevicePathNode{
				VendorId:       n.VendorId,
				ProductId:      n.ProductId,
				DeviceClass:    USBClass(n.DeviceClass),
				DeviceSubClass: USBSubClass(n.DeviceSubClass),
				DeviceProtocol: USBProtocol(n.DeviceProtocol)}, nil
		case uefi.MSG_MAC_ADDR_DP:
			var n uefi.MAC_ADDR_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}

			node := &MACAddrDevicePathNode{
				IfType: NetworkInterfaceType(n.IfType),
			}
			switch node.IfType {
			case NetworkInterfaceTypeReserved, NetworkInterfaceTypeEthernet:
				var addr EUI48
				copy(addr[:], n.MacAddress.Addr[:])
				node.MACAddress = addr
			default:
				node.MACAddress = unknownMACAddress(n.MacAddress.Addr)
			}

			return node, nil
		case uefi.MSG_IPv4_DP:
			var n uefi.IPv4_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &IPv4DevicePathNode{
				LocalAddress:       IPv4Address(n.LocalIpAddress.Addr),
				RemoteAddress:      IPv4Address(n.RemoteIpAddress.Addr),
				LocalPort:          n.LocalPort,
				RemotePort:         n.RemotePort,
				Protocol:           IPProtocol(n.Protocol),
				LocalAddressOrigin: IPv4AddressOrigin(n.StaticIpAddress),
				GatewayAddress:     IPv4Address(n.GatewayIpAddress.Addr),
				SubnetMask:         IPv4Address(n.SubnetMask.Addr)}, nil
		case uefi.MSG_IPv6_DP:
			var n uefi.IPv6_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &IPv6DevicePathNode{
				LocalAddress:       IPv6Address(n.LocalIpAddress.Addr),
				RemoteAddress:      IPv6Address(n.RemoteIpAddress.Addr),
				LocalPort:          n.LocalPort,
				RemotePort:         n.RemotePort,
				Protocol:           IPProtocol(n.Protocol),
				LocalAddressOrigin: IPv6AddressOrigin(n.IpAddressOrigin),
				PrefixLength:       n.PrefixLength,
				GatewayAddress:     IPv6Address(n.GatewayIpAddress.Addr)}, nil
		case uefi.MSG_VENDOR_DP:
			return readVendorDevicePathNode(nodeReader)
		case uefi.MSG_USB_WWID_DP:
			n, err := uefi.Read_USB_WWID_DEVICE_PATH(nodeReader)
			if err != nil {
				return nil, err
			}
			return &USBWWIDDevicePathNode{
				InterfaceNumber: n.InterfaceNumber,
				VendorId:        n.VendorId,
				ProductId:       n.ProductId,
				SerialNumber:    ConvertUTF16ToUTF8(n.SerialNumber)}, nil
		case uefi.MSG_DEVICE_LOGICAL_UNIT_DP:
			var n uefi.DEVICE_LOGICAL_UNIT_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &DeviceLogicalUnitDevicePathNode{LUN: n.Lun}, nil
		case uefi.MSG_SATA_DP:
			var n uefi.SATA_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &SATADevicePathNode{
				HBAPortNumber:            n.HBAPortNumber,
				PortMultiplierPortNumber: n.PortMultiplierPortNumber,
				LUN:                      n.Lun}, nil
		case uefi.MSG_NVME_NAMESPACE_DP:
			var n uefi.NVME_NAMESPACE_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}

			// Convert to the UUID to the EUI64 type, which is an 8-byte
			// array. It goes big-endian into the array - the MSB is the
			// first byte of the UUID. At least that's the way that EDK2
			// prints it. The UEFI spec is a bit vague here.
			var uuid EUI64
			binary.BigEndian.PutUint64(uuid[:], n.NamespaceUuid)

			return &NVMENamespaceDevicePathNode{
				NamespaceID:   n.NamespaceId,
				NamespaceUUID: uuid}, nil
		}
	case uefi.MEDIA_DEVICE_PATH:
		switch hdr.SubType {
		case uefi.MEDIA_HARDDRIVE_DP:
			var n uefi.HARDDRIVE_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}

			var signature HardDriveSignature
			switch n.SignatureType {
			case uefi.NO_DISK_SIGNATURE:
				if n.Signature != EmptyHardDriveSignature.Data() {
					return nil, errors.New("inconsistent signature and signature type: expected empty signature for NO_DISK_SIGNATURE")
				}
				signature = EmptyHardDriveSignature
			case uefi.SIGNATURE_TYPE_MBR:
				signature = MBRHardDriveSignature(binary.LittleEndian.Uint32(n.Signature[:]))
			case uefi.SIGNATURE_TYPE_GUID:
				signature = GUIDHardDriveSignature(n.Signature)
			default:
				signature = &unknownHardDriveSignature{
					typ:  HardDriveSignatureType(n.SignatureType),
					data: n.Signature}
			}
			return &HardDriveDevicePathNode{
				PartitionNumber: n.PartitionNumber,
				PartitionStart:  n.PartitionStart,
				PartitionSize:   n.PartitionSize,
				Signature:       signature,
				MBRType:         MBRType(n.MBRType)}, nil
		case uefi.MEDIA_CDROM_DP:
			var n uefi.CDROM_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &CDROMDevicePathNode{
				BootEntry:      n.BootEntry,
				PartitionStart: n.PartitionStart,
				PartitionSize:  n.PartitionSize}, nil
		case uefi.MEDIA_VENDOR_DP:
			return readVendorDevicePathNode(nodeReader)
		case uefi.MEDIA_FILEPATH_DP:
			n, err := uefi.Read_FILEPATH_DEVICE_PATH(nodeReader)
			if err != nil {
				return nil, err
			}
			return FilePathDevicePathNode(ConvertUTF16ToUTF8(n.PathName)), nil
		case uefi.MEDIA_PIWG_FW_FILE_DP:
			var n uefi.MEDIA_FW_VOL_FILEPATH_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return FWFileDevicePathNode(GUID(n.FvFileName)), nil
		case uefi.MEDIA_PIWG_FW_VOL_DP:
			var n uefi.MEDIA_FW_VOL_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return FWVolDevicePathNode(GUID(n.FvName)), nil
		case uefi.MEDIA_RELATIVE_OFFSET_RANGE_DP:
			var n uefi.MEDIA_RELATIVE_OFFSET_RANGE_DEVICE_PATH
			if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
				return nil, err
			}
			return &MediaRelOffsetRangeDevicePathNode{StartingOffset: n.StartingOffset, EndingOffset: n.EndingOffset}, nil
		}
	case uefi.END_DEVICE_PATH_TYPE:
		if hdr.SubType != uefi.END_ENTIRE_DEVICE_PATH_SUBTYPE {
			return nil, fmt.Errorf("unrecognized END_DEVICE_PATH_TYPE subtype (%#x)", hdr.SubType)
		}
		var n uefi.EFI_DEVICE_PATH_PROTOCOL
		if err := binary.Read(nodeReader, binary.LittleEndian, &n); err != nil {
			return nil, err
		}
		return nil, nil
	}

	return readGenericDevicePathNode(nodeReader)
}

// ReadDevicePath decodes a device path from the supplied io.Reader. It will read
// until it finds a termination node or an error occurs.
func ReadDevicePath(r io.Reader) (out DevicePath, err error) {
	for i := 0; ; i++ {
		node, err := decodeDevicePathNode(r)
		switch {
		case err != nil && i == 0:
			return nil, ioerr.PassRawEOF("cannot decode node %d: %w", i, err)
		case err != nil:
			return nil, ioerr.EOFIsUnexpected("cannot decode node: %d: %w", i, err)
		}
		if node == nil {
			break
		}
		out = append(out, node)
	}
	return out, nil
}
