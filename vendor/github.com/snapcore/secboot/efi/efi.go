// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package efi

import (
	"fmt"
	"github.com/snapcore/snapd/snap"
	"io"
	"os"

	"github.com/canonical/go-efilib"
	"github.com/canonical/tcglog-parser"
)

const (
	bootManagerCodePCR = 4 // Boot Manager Code and Boot Attempts PCR

	certTableIndex = 4 // Index of the Certificate Table entry in the Data Directory of a PE image optional header
)

var (
	eventLogPath = "/sys/kernel/security/tpm0/binary_bios_measurements" // Path of the TCG event log for the default TPM, in binary form
)

// HostEnvironment is an interface that abstracts out an EFI environment, so that
// consumers of the API can provide a custom mechanism to read EFI variables or parse
// the TCG event log.
type HostEnvironment interface {
	ReadVar(name string, guid efi.GUID) ([]byte, efi.VariableAttributes, error)

	ReadEventLog() (*tcglog.Log, error)
}

// Image corresponds to a binary that is loaded, verified and executed before ExitBootServices.
type Image interface {
	fmt.Stringer
	Open() (interface {
		io.ReaderAt
		io.Closer
		Size() int64
	}, error) // Open a handle to the image for reading
}

// SnapFileImage corresponds to a binary contained within a snap file that is loaded, verified and executed before ExitBootServices.
type SnapFileImage struct {
	Container snap.Container
	FileName  string // The filename within the snap squashfs
}

func (f SnapFileImage) String() string {
	return fmt.Sprintf("%#v:%s", f.Container, f.FileName)
}

func (f SnapFileImage) Open() (interface {
	io.ReaderAt
	io.Closer
	Size() int64
}, error) {
	return f.Container.RandomAccessFile(f.FileName)
}

type fileImageHandle struct {
	*os.File
	size int64
}

func (h *fileImageHandle) Size() int64 {
	return h.size
}

// FileImage corresponds to a file on disk that is loaded, verified and executed before ExitBootServices.
type FileImage string

func (p FileImage) String() string {
	return string(p)
}

func (p FileImage) Open() (interface {
	io.ReaderAt
	io.Closer
	Size() int64
}, error) {
	f, err := os.Open(string(p))
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &fileImageHandle{File: f, size: fi.Size()}, nil
}

// ImageLoadEventSource corresponds to the source of a ImageLoadEvent.
type ImageLoadEventSource int

const (
	// Firmware indicates that the source of a ImageLoadEvent was platform firmware, via the EFI_BOOT_SERVICES.LoadImage()
	// and EFI_BOOT_SERVICES.StartImage() functions, with the subsequently executed image being verified against the signatures
	// in the EFI authorized signature database.
	Firmware ImageLoadEventSource = iota

	// Shim indicates that the source of a ImageLoadEvent was shim, without relying on EFI boot services for loading, verifying
	// and executing the subsequently executed image. The image is verified by shim against the signatures in the EFI authorized
	// signature database, the MOK database or shim's built-in vendor certificate before being executed directly.
	Shim
)

// ImageLoadEvent corresponds to the execution of a verified Image.
type ImageLoadEvent struct {
	Source ImageLoadEventSource // The source of the event
	Image  Image                // The image
	Next   []*ImageLoadEvent    // A list of possible subsequent ImageLoadEvents
}
