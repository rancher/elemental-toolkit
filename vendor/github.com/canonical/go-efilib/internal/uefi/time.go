// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package uefi

import (
	"time"
)

type EFI_TIME struct {
	Year       uint16
	Month      uint8
	Day        uint8
	Hour       uint8
	Minute     uint8
	Second     uint8
	Pad1       uint8
	Nanosecond uint32
	Timezone   int16
	Daylight   uint8
	Pad2       uint8
}

func (t *EFI_TIME) GoTime() time.Time {
	return time.Date(int(t.Year), time.Month(t.Month), int(t.Day), int(t.Hour), int(t.Minute), int(t.Second), int(t.Nanosecond), time.FixedZone("", -int(t.Timezone)*60))
}

func New_EFI_TIME(t time.Time) *EFI_TIME {
	_, offset := t.Zone()
	return &EFI_TIME{
		Year:       uint16(t.Year()),
		Month:      uint8(t.Month()),
		Day:        uint8(t.Day()),
		Hour:       uint8(t.Hour()),
		Minute:     uint8(t.Minute()),
		Second:     uint8(t.Second()),
		Nanosecond: uint32(t.Nanosecond()),
		Timezone:   -int16(offset / 60)}
}
