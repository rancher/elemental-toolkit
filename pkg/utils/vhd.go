/*
Copyright © 2022 - 2025 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"
	"time"

	uuidPkg "github.com/distribution/distribution/uuid"
)

// This file contains utils to work with VHD disks

type VHDHeader struct {
	Cookie   [8]byte // Cookies are used to uniquely identify the original creator of the hard disk image
	Features [4]byte // This is a bit field used to indicate specific feature support.
	// Can be 0x00000000 (no features), 0x00000001 (Temporary, candidate for deletion on shutdown) or 0x00000002 (Reserved)
	FileFormatVersion  [4]byte   // Divided into a major/minor version and matches the version of the specification used in creating the file.
	DataOffset         [8]byte   // For fixed disks, this field should be set to 0xFFFFFFFF.
	Timestamp          [4]byte   // Sstores the creation time of a hard disk image. This is the number of seconds since January 1, 2000 12:00:00 AM in UTC/GMT.
	CreatorApplication [4]byte   // Used to document which application created the hard disk.
	CreatorVersion     [4]byte   // This field holds the major/minor version of the application that created the hard disk image.
	CreatorHostOS      [4]byte   // This field stores the type of host operating system this disk image is created on.
	OriginalSize       [8]byte   // This field stores the size of the hard disk in bytes, from the perspective of the virtual machine, at creation time. Info only
	CurrentSize        [8]byte   // This field stores the current size of the hard disk, in bytes, from the perspective of the virtual machine.
	DiskGeometry       [4]byte   // This field stores the cylinder, heads, and sectors per track value for the hard disk.
	DiskType           [4]byte   // Fixed = 2, Dynamic = 3, Differencing = 4
	Checksum           [4]byte   // This field holds a basic checksum of the hard disk footer. It is just a one’s complement of the sum of all the bytes in the footer without the checksum field.
	UniqueID           [16]byte  // This is a 128-bit universally unique identifier (UUID).
	SavedState         [1]byte   // This field holds a one-byte flag that describes whether the system is in saved state. If the hard disk is in the saved state the value is set to 1
	Reserved           [427]byte // This field contains zeroes.
}

func newVHDFixed(size uint64) VHDHeader {
	header := VHDHeader{}
	hexToField("00000002", header.Features[:])
	hexToField("00010000", header.FileFormatVersion[:])
	hexToField("ffffffffffffffff", header.DataOffset[:])
	t := uint32(time.Now().Unix() - 946684800)
	binary.BigEndian.PutUint32(header.Timestamp[:], t)
	hexToField("656c656d", header.CreatorApplication[:]) // Cos
	hexToField("73757365", header.CreatorHostOS[:])      // SUSE
	binary.BigEndian.PutUint64(header.OriginalSize[:], size)
	binary.BigEndian.PutUint64(header.CurrentSize[:], size)
	// Divide size into 512 to get the total sectors
	totalSectors := float64(size / 512)
	geometry := chsCalculation(uint64(totalSectors))
	binary.BigEndian.PutUint16(header.DiskGeometry[:2], uint16(geometry.cylinders))
	header.DiskGeometry[2] = uint8(geometry.heads)
	header.DiskGeometry[3] = uint8(geometry.sectorsPerTrack)
	hexToField("00000002", header.DiskType[:]) // Fixed 0x00000002
	hexToField("00000000", header.Checksum[:])
	uuid := uuidPkg.Generate()
	copy(header.UniqueID[:], uuid.String())
	generateChecksum(&header)
	return header
}

// generateChecksum generates the checksum of the vhd header
// Lifted from the official VHD Format Spec
func generateChecksum(header *VHDHeader) {
	buffer := new(bytes.Buffer)
	_ = binary.Write(buffer, binary.BigEndian, header)
	checksum := 0
	bb := buffer.Bytes()
	for counter := 0; counter < 512; counter++ {
		checksum += int(bb[counter])
	}
	binary.BigEndian.PutUint32(header.Checksum[:], uint32(^checksum))
}

// hexToField decodes an hex to bytes and copies it to the given header field
func hexToField(hexs string, field []byte) {
	h, _ := hex.DecodeString(hexs)
	copy(field, h)
}

// chs is a simple struct to represent the cylinders/heads/sectors for a given sector count
type chs struct {
	cylinders       uint
	heads           uint
	sectorsPerTrack uint
}

// chsCalculation calculates the cylinders, headers and sectors per track for a given sector count
// Exactly the same code on the official VHD format spec
func chsCalculation(sectors uint64) chs {
	var sectorsPerTrack,
		heads,
		cylinderTimesHeads,
		cylinders float64
	totalSectors := float64(sectors)

	if totalSectors > 65535*16*255 {
		totalSectors = 65535 * 16 * 255
	}

	if totalSectors >= 65535*16*63 {
		sectorsPerTrack = 255
		heads = 16
		cylinderTimesHeads = math.Floor(totalSectors / sectorsPerTrack)
	} else {
		sectorsPerTrack = 17
		cylinderTimesHeads = math.Floor(totalSectors / sectorsPerTrack)
		heads = math.Floor((cylinderTimesHeads + 1023) / 1024)
		if heads < 4 {
			heads = 4
		}
		if (cylinderTimesHeads >= (heads * 1024)) || heads > 16 {
			sectorsPerTrack = 31
			heads = 16
			cylinderTimesHeads = math.Floor(totalSectors / sectorsPerTrack)
		}
		if cylinderTimesHeads >= (heads * 1024) {
			sectorsPerTrack = 63
			heads = 16
			cylinderTimesHeads = math.Floor(totalSectors / sectorsPerTrack)
		}
	}

	cylinders = cylinderTimesHeads / heads

	return chs{
		cylinders:       uint(cylinders),
		heads:           uint(heads),
		sectorsPerTrack: uint(sectorsPerTrack),
	}
}

// RawDiskToFixedVhd will write the proper header to a given os.File to convert it from a simple raw disk to a Fixed VHD
// RawDiskToFixedVhd makes no effort into opening/closing/checking if the file exists
func RawDiskToFixedVhd(diskFile *os.File) {
	info, _ := diskFile.Stat()
	size := uint64(info.Size())
	header := newVHDFixed(size)
	_ = binary.Write(diskFile, binary.BigEndian, header)
}
