// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"fmt"
	"io"
	"strings"
)

var (
	kernelCmdlinePrefix = "kernel_cmdline: "
	grubCmdPrefix       = "grub_cmd: "
)

// GrubStringEventType indicates the type of data measured by GRUB in to a log by GRUB.
type GrubStringEventType string

const (
	// GrubCmd indicates that the data measured by GRUB is associated with a GRUB command.
	GrubCmd GrubStringEventType = "grub_cmd"

	// KernelCmdline indicates that the data measured by GRUB is associated with a kernel commandline.
	KernelCmdline = "kernel_cmdline"
)

// GrubStringEventData represents the data associated with an event measured by GRUB.
type GrubStringEventData struct {
	rawEventData
	Type GrubStringEventType
	Str  string
}

func (e *GrubStringEventData) String() string {
	return fmt.Sprintf("%s{ %s }", string(e.Type), e.Str)
}

func (e *GrubStringEventData) Write(w io.Writer) error {
	_, err := io.WriteString(w, fmt.Sprintf("%s: %s\x00", string(e.Type), e.Str))
	return err
}

func decodeEventDataGRUB(data []byte, pcrIndex PCRIndex, eventType EventType) EventData {
	if eventType != EventTypeIPL {
		return nil
	}

	switch pcrIndex {
	case 8:
		str := string(data)
		switch {
		case strings.HasPrefix(str, kernelCmdlinePrefix):
			return &GrubStringEventData{rawEventData: data, Type: KernelCmdline, Str: strings.TrimSuffix(strings.TrimPrefix(str, kernelCmdlinePrefix), "\x00")}
		case strings.HasPrefix(str, grubCmdPrefix):
			return &GrubStringEventData{rawEventData: data, Type: GrubCmd, Str: strings.TrimSuffix(strings.TrimPrefix(str, grubCmdPrefix), "\x00")}
		default:
			return nil
		}
	case 9:
		return StringEventData(data)
	default:
		panic("unhandled PCR index")
	}
}
