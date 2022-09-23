// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

const (
	EFI_VARIABLE_NON_VOLATILE                          = 1 << 0
	EFI_VARIABLE_BOOTSERVICE_ACCESS                    = 1 << 1
	EFI_VARIABLE_RUNTIME_ACCESS                        = 1 << 2
	EFI_VARIABLE_HARDWARE_ERROR_RECORD                 = 1 << 3
	EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS            = 1 << 4
	EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS = 1 << 5
	EFI_VARIABLE_APPEND_WRITE                          = 1 << 6
	EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS         = 1 << 7
)
