// Copyright 2025 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package efi

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/canonical/go-efilib/internal/uefi"
)

// NetworkInterfaceType describes the type of network hardware.
type NetworkInterfaceType uint8

const (
	NetworkInterfaceTypeReserved NetworkInterfaceType = 0
	NetworkInterfaceTypeEthernet NetworkInterfaceType = 1
)

// IPProtocol describes an IP protocol
type IPProtocol uint16

const (
	IPProtocolTCP IPProtocol = uefi.RFC_1700_TCP_PROTOCOL
	IPProtocolUDP IPProtocol = uefi.RFC_1700_UDP_PROTOCOL
)

// String implements [fmt.Stringer].
func (p IPProtocol) String() string {
	switch p {
	case IPProtocolTCP:
		return "TCP"
	case IPProtocolUDP:
		return "UDP"
	default:
		return fmt.Sprintf("%#x", uint16(p))
	}
}

// IPv4Address corresponds to an IP v4 address.
type IPv4Address [4]uint8

// String implements [fmt.Stringer].
func (a IPv4Address) String() string {
	return fmt.Sprintf("%d:%d:%d:%d", a[0], a[1], a[2], a[3])
}

// AsNetIPAddr returns the address as a [netip.Addr].
func (a IPv4Address) AsNetIPAddr() netip.Addr {
	return netip.AddrFrom4([4]uint8(a))
}

// IPv4AddressOrigin descibes how an IP v4 address was assigned.
type IPv4AddressOrigin bool

const (
	IPv4AddressDHCPAssigned IPv4AddressOrigin = false // Assigned by a DHCP server.
	StaticIPv4Address       IPv4AddressOrigin = true  // Statically assigned.
)

// String implements [fmt.Stringer].
func (o IPv4AddressOrigin) String() string {
	switch o {
	case IPv4AddressDHCPAssigned:
		return "DHCP"
	case StaticIPv4Address:
		return "Static"
	}
	panic("not reached")
}

// IPv6Address corresponds to an IP v6 address.
type IPv6Address [16]uint8

// String implements [fmt.Stringer].
func (a IPv6Address) String() string {
	return fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x",
		a[0], a[1], a[2], a[3], a[4], a[5], a[6], a[7], a[8],
		a[9], a[10], a[11], a[12], a[13], a[14], a[15])
}

// AsNetIPAddr returns the address as a [netip.Addr].
func (a IPv6Address) AsNetIPAddr() netip.Addr {
	return netip.AddrFrom16([16]uint8(a))
}

// IPv6AddressOrigin descibes how an IP v6 address was assigned.
type IPv6AddressOrigin uint8

const (
	StaticIPv6Address        IPv6AddressOrigin = 0 // Statically assigned.
	IPv6AddressSLAACAssigned IPv6AddressOrigin = 1 // Assigned using SLAAC.
	IPv6AddressDHCPAssigned  IPv6AddressOrigin = 2 // Assigned by a DHCPv6 server.
)

// String implements [fmt.Stringer].
func (o IPv6AddressOrigin) String() string {
	switch o {
	case StaticIPv6Address:
		return "Static"
	case IPv6AddressSLAACAssigned:
		return "StatelessAutoConfigure"
	case IPv6AddressDHCPAssigned:
		return "StatefulAutoConfigure"
	default:
		return fmt.Sprintf("%#x", uint8(o))
	}
}

// MACAddressType describes the type of a MAC address.
type MACAddressType int

const (
	MACAddressTypeUnknown MACAddressType = iota // an unknown address type
	MACAddressTypeEUI48                         // EUI-48 address type
	MACAddressTypeEUI64                         // EUI-64 address type
)

// MACAddress is an abstraction for a MAC address.
type MACAddress interface {
	fmt.Stringer

	// Bytes32 returns the address as a 32-byte left-aligned, zero padded array,
	// which is how MAC addresses are represented in UEFI.
	Bytes32() [32]uint8

	Type() MACAddressType // Address type
}

// EUI64 represents a EUI-64 (64-bit Extended Unique Identifier).
type EUI64 [8]uint8

// String implements [fmt.Stringer].
func (id EUI64) String() string {
	return fmt.Sprintf("%02x-%02x-%02x-%02x-%02x-%02x-%02x-%02x",
		id[0], id[1], id[2], id[3], id[4], id[5], id[6], id[7])
}

// Bytes implements [MACAddress.Bytes32].
func (id EUI64) Bytes32() [32]uint8 {
	var out [32]uint8
	copy(out[:], id[:])
	return out
}

// Type implements [MACAddress.Type].
func (EUI64) Type() MACAddressType {
	return MACAddressTypeEUI64
}

// AsEUI48 returns this identifier as EUI-48, if it is a valid EUI-48.
func (id EUI64) AsEUI48() (EUI48, error) {
	if id[3] != 0xFF || id[4] != 0xFE {
		return EUI48{}, errors.New("EUI64 doesn't represent a EUI48 address")
	}

	var out EUI48
	copy(out[0:], id[:3])
	copy(out[3:], id[5:])
	return out, nil
}

// EUI48 represents a EUI-48 (48-bit Extended Unique Identifier).
type EUI48 [6]uint8

// String implements [fmt.Stringer].
func (id EUI48) String() string {
	return fmt.Sprintf("%02x-%02x-%02x-%02x-%02x-%02x",
		id[0], id[1], id[2], id[3], id[4], id[5])
}

// Bytes32 implements [MACAddress.Bytes32].
func (id EUI48) Bytes32() [32]byte {
	var out [32]uint8
	copy(out[:], id[:])
	return out
}

// Type implements [MACAddress.Type].
func (EUI48) Type() MACAddressType {
	return MACAddressTypeEUI64
}

// AsEUI64 returns this identifier as EUI-64.
func (id EUI48) AsEUI64() EUI64 {
	var out EUI64
	copy(out[0:], id[:3])
	out[3] = 0xFF
	out[4] = 0xFE
	copy(out[5:], id[3:])
	return out
}

type unknownMACAddress [32]uint8

func (address unknownMACAddress) String() string {
	return fmt.Sprintf("%x", [32]byte(address))
}

func (address unknownMACAddress) Bytes32() [32]uint8 {
	var out [32]uint8
	copy(out[:], address[:])
	return out
}

func (unknownMACAddress) Type() MACAddressType {
	return MACAddressTypeUnknown
}
