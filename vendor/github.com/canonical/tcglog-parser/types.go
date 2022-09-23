// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"fmt"

	"github.com/canonical/go-tpm2"
)

// PCRIndex corresponds to the index of a PCR on the TPM.
type PCRIndex uint32

// EventType corresponds to the type of an event in an event log.
type EventType uint32

// Digest is the result of hashing some data.
type Digest []byte

// DigestMap is a map of algorithms to digests.
type DigestMap map[tpm2.HashAlgorithmId]Digest

func (e EventType) String() string {
	switch e {
	case EventTypePrebootCert:
		return "EV_PREBOOT_CERT"
	case EventTypePostCode:
		return "EV_POST_CODE"
	case EventTypeNoAction:
		return "EV_NO_ACTION"
	case EventTypeSeparator:
		return "EV_SEPARATOR"
	case EventTypeAction:
		return "EV_ACTION"
	case EventTypeEventTag:
		return "EV_EVENT_TAG"
	case EventTypeSCRTMContents:
		return "EV_S_CRTM_CONTENTS"
	case EventTypeSCRTMVersion:
		return "EV_S_CRTM_VERSION"
	case EventTypeCPUMicrocode:
		return "EV_CPU_MICROCODE"
	case EventTypePlatformConfigFlags:
		return "EV_PLATFORM_CONFIG_FLAGS"
	case EventTypeTableOfDevices:
		return "EV_TABLE_OF_DEVICES"
	case EventTypeCompactHash:
		return "EV_COMPACT_HASH"
	case EventTypeIPL:
		return "EV_IPL"
	case EventTypeIPLPartitionData:
		return "EV_IPL_PARTITION_DATA"
	case EventTypeNonhostCode:
		return "EV_NONHOST_CODE"
	case EventTypeNonhostConfig:
		return "EV_NONHOST_CONFIG"
	case EventTypeNonhostInfo:
		return "EV_NONHOST_INFO"
	case EventTypeOmitBootDeviceEvents:
		return "EV_OMIT_BOOT_DEVICE_EVENTS"
	case EventTypeEFIVariableDriverConfig:
		return "EV_EFI_VARIABLE_DRIVER_CONFIG"
	case EventTypeEFIVariableBoot:
		return "EV_EFI_VARIABLE_BOOT"
	case EventTypeEFIBootServicesApplication:
		return "EV_EFI_BOOT_SERVICES_APPLICATION"
	case EventTypeEFIBootServicesDriver:
		return "EV_EFI_BOOT_SERVICES_DRIVER"
	case EventTypeEFIRuntimeServicesDriver:
		return "EV_EFI_RUNTIME_SERVICES_DRIVER"
	case EventTypeEFIGPTEvent:
		return "EF_EFI_GPT_EVENT"
	case EventTypeEFIAction:
		return "EV_EFI_ACTION"
	case EventTypeEFIPlatformFirmwareBlob:
		return "EV_EFI_PLATFORM_FIRMWARE_BLOB"
	case EventTypeEFIHandoffTables:
		return "EV_EFI_HANDOFF_TABLES"
	case EventTypeEFIPlatformFirmwareBlob2:
		return "EV_EFI_PLATFORM_FIRMWARE_BLOB2"
	case EventTypeEFIHandoffTables2:
		return "EV_EFI_HANDOFF_TABLES2"
	case EventTypeEFIVariableBoot2:
		return "EV_EFI_VARIABLE_BOOT2"
	case EventTypeEFIHCRTMEvent:
		return "EV_EFI_HCRTM_EVENT"
	case EventTypeEFIVariableAuthority:
		return "EV_EFI_VARIABLE_AUTHORITY"
	case EventTypeEFISPDMFirmwareBlob:
		return "EV_EFI_SPDM_FIRMWARE_BLOB"
	case EventTypeEFISPDMFirmwareConfig:
		return "EV_EFI_SPDM_FIRMWARE_CONFIG"
	default:
		return fmt.Sprintf("%08x", uint32(e))
	}
}

func (e EventType) Format(s fmt.State, f rune) {
	switch f {
	case 's', 'v':
		fmt.Fprintf(s, "%s", e.String())
	default:
		fmt.Fprintf(s, makeDefaultFormatter(s, f), uint32(e))
	}
}

// AlgorithmListId is a slice of tpm2.HashAlgorithmId values,
type AlgorithmIdList []tpm2.HashAlgorithmId

func (l AlgorithmIdList) Contains(a tpm2.HashAlgorithmId) bool {
	for _, alg := range l {
		if alg == a {
			return true
		}
	}
	return false
}
