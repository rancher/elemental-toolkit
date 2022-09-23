// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"math"
)

const (
	EventTypePrebootCert EventType = 0x00000000 // EV_PREBOOT_CERT
	EventTypePostCode    EventType = 0x00000001 // EV_POST_CODE
	// EventTypeUnused = 0x00000002
	EventTypeNoAction             EventType = 0x00000003 // EV_NO_ACTION
	EventTypeSeparator            EventType = 0x00000004 // EV_SEPARATOR
	EventTypeAction               EventType = 0x00000005 // EV_ACTION
	EventTypeEventTag             EventType = 0x00000006 // EV_EVENT_TAG
	EventTypeSCRTMContents        EventType = 0x00000007 // EV_S_CRTM_CONTENTS
	EventTypeSCRTMVersion         EventType = 0x00000008 // EV_S_CRTM_VERSION
	EventTypeCPUMicrocode         EventType = 0x00000009 // EV_CPU_MICROCODE
	EventTypePlatformConfigFlags  EventType = 0x0000000a // EV_PLATFORM_CONFIG_FLAGS
	EventTypeTableOfDevices       EventType = 0x0000000b // EV_TABLE_OF_DEVICES
	EventTypeCompactHash          EventType = 0x0000000c // EV_COMPACT_HASH
	EventTypeIPL                  EventType = 0x0000000d // EV_IPL
	EventTypeIPLPartitionData     EventType = 0x0000000e // EV_IPL_PARTITION_DATA
	EventTypeNonhostCode          EventType = 0x0000000f // EV_NONHOST_CODE
	EventTypeNonhostConfig        EventType = 0x00000010 // EV_NONHOST_CONFIG
	EventTypeNonhostInfo          EventType = 0x00000011 // EV_NONHOST_INFO
	EventTypeOmitBootDeviceEvents EventType = 0x00000012 // EV_OMIT_BOOT_DEVICE_EVENTS

	EventTypeEFIEventBase               EventType = 0x80000000 // EV_EFI_EVENT_BASE
	EventTypeEFIVariableDriverConfig    EventType = 0x80000001 // EV_EFI_VARIABLE_DRIVER_CONFIG
	EventTypeEFIVariableBoot            EventType = 0x80000002 // EV_EFI_VARIABLE_BOOT
	EventTypeEFIBootServicesApplication EventType = 0x80000003 // EV_EFI_BOOT_SERVICES_APPLICATION
	EventTypeEFIBootServicesDriver      EventType = 0x80000004 // EV_EFI_BOOT_SERVICES_DRIVER
	EventTypeEFIRuntimeServicesDriver   EventType = 0x80000005 // EV_EFI_RUNTIME_SERVICES_DRIVER
	EventTypeEFIGPTEvent                EventType = 0x80000006 // EV_EFI_GPT_EVENT
	EventTypeEFIAction                  EventType = 0x80000007 // EV_EFI_ACTION
	EventTypeEFIPlatformFirmwareBlob    EventType = 0x80000008 // EV_EFI_PLATFORM_FIRMWARE_BLOB
	EventTypeEFIHandoffTables           EventType = 0x80000009 // EV_EFI_HANDOFF_TABLES
	EventTypeEFIPlatformFirmwareBlob2   EventType = 0x8000000a // EV_EFI_PLATFORM_FIRMWARE_BLOB2
	EventTypeEFIHandoffTables2          EventType = 0x8000000b // EV_EFI_HANDOFF_TABLES2
	EventTypeEFIVariableBoot2           EventType = 0x8000000c // EV_EFI_VARIABLE_BOOT2
	EventTypeEFIHCRTMEvent              EventType = 0x80000010 // EV_EFI_HCRTM_EVENT
	EventTypeEFIVariableAuthority       EventType = 0x800000e0 // EV_EFI_VARIABLE_AUTHORITY
	EventTypeEFISPDMFirmwareBlob        EventType = 0x800000e1 // EV_EFI_SPDM_FIRMWARE_BLOB
	EventTypeEFISPDMFirmwareConfig      EventType = 0x800000e2 // EV_EFI_SPDM_FIRMWARE_CONFIG
)

const (
	SeparatorEventNormalValue    uint32 = 0
	SeparatorEventErrorValue     uint32 = 1
	SeparatorEventAltNormalValue uint32 = math.MaxUint32
)

var (
	EFICallingEFIApplicationEvent       = StringEventData("Calling EFI Application from Boot Option")
	EFIReturningFromEFIApplicationEvent = StringEventData("Returning from EFI Application from Boot Option")
	EFIExitBootServicesInvocationEvent  = StringEventData("Exit Boot Services Invocation")
	EFIExitBootServicesFailedEvent      = StringEventData("Exit Boot Services Returned with Failure")
	EFIExitBootServicesSucceededEvent   = StringEventData("Exit Boot Services Returned with Success")
	FirmwareDebuggerEvent               = StringEventData("UEFI Debug Mode")
)
