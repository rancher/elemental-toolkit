// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"bytes"
	"crypto"
	"encoding/binary"
	"io"
)

// SystemdEFIStubCommandline represents a kernel commandline measured by the
// systemd EFI stub linux loader.
type SystemdEFIStubCommandline struct {
	rawEventData
	Str string
}

func (e *SystemdEFIStubCommandline) String() string {
	return "kernel commandline: " + e.Str
}

func (e *SystemdEFIStubCommandline) Write(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, convertStringToUtf16(e.Str)); err != nil {
		return err
	}
	_, err := w.Write([]byte{0x00})
	return err
}

// ComputeSystemdEFIStubCommandlineDigest computes the digest measured by the systemd EFI stub
// linux loader for the specified kernel commandline. The commandline is supplied to the stub
// via the LoadOptions as a UTF-16 or UCS-2 string and is measured as such before being converted
// to ASCII and passed to the kernel. Note that it assumes that the calling bootloader includes
// a UTF-16 NULL terminator at the end of LoadOptions and sets LoadOptionsSize to StrLen(LoadOptions)+1.
func ComputeSystemdEFIStubCommandlineDigest(alg crypto.Hash, commandline string) []byte {
	h := alg.New()

	// Both GRUB's chainloader and systemd's EFI bootloader include a UTF-16 NULL terminator at
	// the end of LoadOptions and set LoadOptionsSize to StrLen(LoadOptions)+1. The EFI stub loader
	// measures LoadOptionsSize number of bytes, meaning that the 2 NULL bytes are measured.
	// Include those here.
	binary.Write(h, binary.LittleEndian, append(convertStringToUtf16(commandline), 0))
	return h.Sum(nil)
}

func decodeEventDataSystemdEFIStub(data []byte, eventType EventType) *SystemdEFIStubCommandline {
	if eventType != EventTypeIPL {
		return nil
	}

	// data is a UTF-16 string in little-endian form terminated with a single zero byte,
	// so we should have an odd number of bytes.
	if len(data)%2 != 1 {
		return nil
	}
	if data[len(data)-1] != 0x00 {
		return nil
	}

	// Omit the zero byte added by the EFI stub and then convert to native byte order.
	reader := bytes.NewReader(data[:len(data)-1])

	utf16Str := make([]uint16, len(data)/2)
	binary.Read(reader, binary.LittleEndian, &utf16Str)

	return &SystemdEFIStubCommandline{rawEventData: data, Str: convertUtf16ToString(utf16Str)}
}
