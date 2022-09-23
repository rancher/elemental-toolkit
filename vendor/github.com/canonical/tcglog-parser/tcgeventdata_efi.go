// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tcglog

import (
	"bytes"
	"crypto"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/canonical/go-efilib"
	"github.com/canonical/go-tpm2"

	"golang.org/x/xerrors"

	"github.com/canonical/tcglog-parser/internal/ioerr"
)

var (
	surr1 uint16 = 0xd800
	surr2 uint16 = 0xdc00
	surr3 uint16 = 0xe000
)

// UEFI_VARIABLE_DATA specifies the number of *characters* for a UTF-16 sequence rather than the size of
// the buffer. Extract a UTF-16 sequence of the correct length, given a buffer and the number of characters.
// The returned buffer can be passed to utf16.Decode.
func extractUTF16Buffer(r io.ReadSeeker, nchars uint64) ([]uint16, error) {
	var out []uint16

	for i := nchars; i > 0; i-- {
		var c uint16
		if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
		if c >= surr1 && c < surr2 {
			if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
				return nil, err
			}
			if c < surr2 || c >= surr3 {
				// Invalid surrogate sequence. utf16.Decode doesn't consume this
				// byte when inserting the replacement char
				if _, err := r.Seek(-1, io.SeekCurrent); err != nil {
					return nil, err
				}
				continue
			}
			// Valid surrogate sequence
			out = append(out, c)
		}
	}

	return out, nil
}

type rawSpecIdEvent02Hdr struct {
	PlatformClass    uint32
	SpecVersionMinor uint8
	SpecVersionMajor uint8
	SpecErrata       uint8
	UintnSize        uint8
	VendorInfoSize   uint8
}

// SpecIdEvent02 corresponds to the TCG_EfiSpecIdEventStruct type and is the
// event data for a Specification ID Version EV_NO_ACTION event on EFI platforms
// for TPM family 1.2.
type SpecIdEvent02 struct {
	rawEventData
	PlatformClass    uint32
	SpecVersionMinor uint8
	SpecVersionMajor uint8
	SpecErrata       uint8
	UintnSize        uint8
	VendorInfo       []byte
}

func (e *SpecIdEvent02) String() string {
	return fmt.Sprintf("EfiSpecIdEvent{ platformClass=%d, specVersionMinor=%d, specVersionMajor=%d, specErrata=%d, uintnSize=%d }",
		e.PlatformClass, e.SpecVersionMinor, e.SpecVersionMajor, e.SpecErrata, e.UintnSize)
}

func (e *SpecIdEvent02) Write(w io.Writer) error {
	vendorInfoSize := len(e.VendorInfo)
	if vendorInfoSize > math.MaxUint8 {
		return errors.New("VendorInfo too large")
	}

	var signature [16]byte
	copy(signature[:], []byte("Spec ID Event02"))
	if _, err := w.Write(signature[:]); err != nil {
		return err
	}

	spec := rawSpecIdEvent02Hdr{
		PlatformClass:    e.PlatformClass,
		SpecVersionMinor: e.SpecVersionMinor,
		SpecVersionMajor: e.SpecVersionMajor,
		SpecErrata:       e.SpecErrata,
		UintnSize:        e.UintnSize,
		VendorInfoSize:   uint8(vendorInfoSize)}
	if err := binary.Write(w, binary.LittleEndian, &spec); err != nil {
		return err
	}

	_, err := w.Write(e.VendorInfo)
	return err
}

// https://trustedcomputinggroup.org/wp-content/uploads/TCG_EFI_Platform_1_22_Final_-v15.pdf
//  (section 7.4 "EV_NO_ACTION Event Types")
func decodeSpecIdEvent02(data []byte, r io.Reader) (out *SpecIdEvent02, err error) {
	var spec rawSpecIdEvent02Hdr
	if err := binary.Read(r, binary.LittleEndian, &spec); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	out = &SpecIdEvent02{
		rawEventData:     data,
		PlatformClass:    spec.PlatformClass,
		SpecVersionMinor: spec.SpecVersionMinor,
		SpecVersionMajor: spec.SpecVersionMajor,
		SpecErrata:       spec.SpecErrata,
		UintnSize:        spec.UintnSize,
		VendorInfo:       make([]byte, spec.VendorInfoSize)}
	if _, err := io.ReadFull(r, out.VendorInfo); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return out, nil
}

// EFISpecIdEventAlgorithmSize represents a digest algorithm and its length and corresponds to the
// TCG_EfiSpecIdEventAlgorithmSize type.
type EFISpecIdEventAlgorithmSize struct {
	AlgorithmId tpm2.HashAlgorithmId
	DigestSize  uint16
}

type rawSpecIdEvent03Hdr struct {
	PlatformClass      uint32
	SpecVersionMinor   uint8
	SpecVersionMajor   uint8
	SpecErrata         uint8
	UintnSize          uint8
	NumberOfAlgorithms uint32
}

// SpecIdEvent03 corresponds to the TCG_EfiSpecIdEvent type and is the
// event data for a Specification ID Version EV_NO_ACTION event on EFI platforms
// for TPM family 2.0.
type SpecIdEvent03 struct {
	rawEventData
	PlatformClass    uint32
	SpecVersionMinor uint8
	SpecVersionMajor uint8
	SpecErrata       uint8
	UintnSize        uint8
	DigestSizes      []EFISpecIdEventAlgorithmSize // The digest algorithms contained within this log
	VendorInfo       []byte
}

func (e *SpecIdEvent03) String() string {
	var builder bytes.Buffer
	fmt.Fprintf(&builder, "EfiSpecIdEvent{ platformClass=%d, specVersionMinor=%d, specVersionMajor=%d, specErrata=%d, uintnSize=%d, digestSizes=[",
		e.PlatformClass, e.SpecVersionMinor, e.SpecVersionMajor, e.SpecErrata, e.UintnSize)
	for i, algSize := range e.DigestSizes {
		if i > 0 {
			builder.WriteString(", ")
		}
		fmt.Fprintf(&builder, "{ algorithmId=0x%04x, digestSize=%d }",
			uint16(algSize.AlgorithmId), algSize.DigestSize)
	}
	builder.WriteString("] }")
	return builder.String()
}

func (e *SpecIdEvent03) Write(w io.Writer) error {
	vendorInfoSize := len(e.VendorInfo)
	if vendorInfoSize > math.MaxUint8 {
		return errors.New("VendorInfo too large")
	}

	var signature [16]byte
	copy(signature[:], []byte("Spec ID Event03"))
	if _, err := w.Write(signature[:]); err != nil {
		return err
	}

	spec := rawSpecIdEvent03Hdr{
		PlatformClass:      e.PlatformClass,
		SpecVersionMinor:   e.SpecVersionMinor,
		SpecVersionMajor:   e.SpecVersionMajor,
		SpecErrata:         e.SpecErrata,
		UintnSize:          e.UintnSize,
		NumberOfAlgorithms: uint32(len(e.DigestSizes))}
	if err := binary.Write(w, binary.LittleEndian, &spec); err != nil {
		return err
	}

	if err := binary.Write(w, binary.LittleEndian, e.DigestSizes); err != nil {
		return err
	}

	if err := binary.Write(w, binary.LittleEndian, uint8(vendorInfoSize)); err != nil {
		return err
	}
	_, err := w.Write(e.VendorInfo)
	return err
}

// https://trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientSpecPlat_TPM_2p0_1p04_pub.pdf
//  (secion 9.4.5.1 "Specification ID Version Event")
func decodeSpecIdEvent03(data []byte, r io.Reader) (out *SpecIdEvent03, err error) {
	var spec rawSpecIdEvent03Hdr
	if err := binary.Read(r, binary.LittleEndian, &spec); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	out = &SpecIdEvent03{
		rawEventData:     data,
		PlatformClass:    spec.PlatformClass,
		SpecVersionMinor: spec.SpecVersionMinor,
		SpecVersionMajor: spec.SpecVersionMajor,
		SpecErrata:       spec.SpecErrata,
		UintnSize:        spec.UintnSize}

	if spec.NumberOfAlgorithms < 1 {
		return nil, errors.New("numberOfAlgorithms is zero")
	}

	out.DigestSizes = make([]EFISpecIdEventAlgorithmSize, spec.NumberOfAlgorithms)
	if err := binary.Read(r, binary.LittleEndian, out.DigestSizes); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	for _, d := range out.DigestSizes {
		if d.AlgorithmId.IsValid() && d.AlgorithmId.Size() != int(d.DigestSize) {
			return nil, fmt.Errorf("digestSize for algorithmId %v does not match expected size", d.AlgorithmId)
		}
	}
	var vendorInfoSize uint8
	if err := binary.Read(r, binary.LittleEndian, &vendorInfoSize); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	out.VendorInfo = make([]byte, vendorInfoSize)
	if _, err := io.ReadFull(r, out.VendorInfo); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return out, nil
}

// StartupLocalityEventData is the event data for a StartupLocality EV_NO_ACTION event.
type StartupLocalityEventData struct {
	rawEventData
	StartupLocality uint8
}

func (e *StartupLocalityEventData) String() string {
	return fmt.Sprintf("EfiStartupLocalityEvent{ StartupLocality: %d }", e.StartupLocality)
}

func (e *StartupLocalityEventData) Write(w io.Writer) error {
	var signature [16]byte
	copy(signature[:], []byte("StartupLocality"))
	if _, err := w.Write(signature[:]); err != nil {
		return err
	}

	return binary.Write(w, binary.LittleEndian, e.StartupLocality)
}

// https://trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientSpecPlat_TPM_2p0_1p04_pub.pdf
//  (section 9.4.5.3 "Startup Locality Event")
func decodeStartupLocalityEvent(data []byte, r io.Reader) (*StartupLocalityEventData, error) {
	var locality uint8
	if err := binary.Read(r, binary.LittleEndian, &locality); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return &StartupLocalityEventData{rawEventData: data, StartupLocality: locality}, nil
}

// SP800_155_PlatformIdEventData corresponds to the event data for a SP800-155-Event
// EV_NO_ACTION event
type SP800_155_PlatformIdEventData struct {
	rawEventData
	VendorId              uint32
	ReferenceManifestGuid efi.GUID
}

func (e *SP800_155_PlatformIdEventData) String() string {
	return fmt.Sprintf("Sp800_155_PlatformId_Event{ VendorId: %d, ReferenceManifestGuid: %s }", e.VendorId, e.ReferenceManifestGuid)
}

func (e *SP800_155_PlatformIdEventData) Write(w io.Writer) error {
	var signature [16]byte
	copy(signature[:], []byte("SP800-155 Event"))
	if _, err := w.Write(signature[:]); err != nil {
		return err
	}

	if err := binary.Write(w, binary.LittleEndian, e.VendorId); err != nil {
		return err
	}
	_, err := w.Write(e.ReferenceManifestGuid[:])
	return err
}

// https://trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientSpecPlat_TPM_2p0_1p04_pub.pdf
//  (section 9.4.5.2 "BIOS Integrity Measurement Reference Manifest Event")
// https://trustedcomputinggroup.org/wp-content/uploads/TCG_EFI_Platform_1_22_Final_-v15.pdf
//  (section 7.4 "EV_NO_ACTION Event Types")
func decodeBIMReferenceManifestEvent(data []byte, r io.Reader) (*SP800_155_PlatformIdEventData, error) {
	var d struct {
		VendorId uint32
		Guid     efi.GUID
	}
	if err := binary.Read(r, binary.LittleEndian, &d); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return &SP800_155_PlatformIdEventData{rawEventData: data, VendorId: d.VendorId, ReferenceManifestGuid: d.Guid}, nil
}

// EFIVariableData corresponds to the EFI_VARIABLE_DATA type and is the event data associated with the measurement of an
// EFI variable.
type EFIVariableData struct {
	rawEventData
	VariableName efi.GUID
	UnicodeName  string
	VariableData []byte
}

func (e *EFIVariableData) String() string {
	return fmt.Sprintf("UEFI_VARIABLE_DATA{ VariableName: %s, UnicodeName: \"%s\", VariableData:\n\t%s}",
		e.VariableName, e.UnicodeName, strings.Replace(hex.Dump(e.VariableData), "\n", "\n\t", -1))
}

func (e *EFIVariableData) Write(w io.Writer) error {
	if _, err := w.Write(e.VariableName[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(utf8.RuneCount([]byte(e.UnicodeName)))); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(len(e.VariableData))); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, convertStringToUtf16(e.UnicodeName)); err != nil {
		return err
	}
	_, err := w.Write(e.VariableData)
	return err
}

// ComputeEFIVariableDataDigest computes the EFI_VARIABLE_DATA digest associated with the supplied
// parameters
func ComputeEFIVariableDataDigest(alg crypto.Hash, name string, guid efi.GUID, data []byte) []byte {
	h := alg.New()
	varData := EFIVariableData{VariableName: guid, UnicodeName: name, VariableData: data}
	varData.Write(h)
	return h.Sum(nil)
}

// https://trustedcomputinggroup.org/wp-content/uploads/TCG_EFI_Platform_1_22_Final_-v15.pdf (section 7.8 "Measuring EFI Variables")
// https://trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientSpecPlat_TPM_2p0_1p04_pub.pdf (section 9.2.6 "Measuring UEFI Variables")
func decodeEventDataEFIVariable(data []byte) (*EFIVariableData, error) {
	r := bytes.NewReader(data)

	d := &EFIVariableData{rawEventData: data}

	variableName, err := efi.ReadGUID(r)
	if err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	d.VariableName = variableName

	var unicodeNameLength uint64
	if err := binary.Read(r, binary.LittleEndian, &unicodeNameLength); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	var variableDataLength uint64
	if err := binary.Read(r, binary.LittleEndian, &variableDataLength); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	utf16Name, err := extractUTF16Buffer(r, unicodeNameLength)
	if err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	d.UnicodeName = convertUtf16ToString(utf16Name)

	d.VariableData = make([]byte, variableDataLength)
	if _, err := io.ReadFull(r, d.VariableData); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return d, nil
}

type rawEFIImageLoadEventHdr struct {
	LocationInMemory   efi.PhysicalAddress
	LengthInMemory     uint64
	LinkTimeAddress    uint64
	LengthOfDevicePath uint64
}

type EFIImageLoadEvent struct {
	rawEventData
	LocationInMemory efi.PhysicalAddress
	LengthInMemory   uint64
	LinkTimeAddress  uint64
	DevicePath       efi.DevicePath
}

func (e *EFIImageLoadEvent) String() string {
	return fmt.Sprintf("UEFI_IMAGE_LOAD_EVENT{ ImageLocationInMemory: 0x%016x, ImageLengthInMemory: %d, "+
		"ImageLinkTimeAddress: 0x%016x, DevicePath: %s }", e.LocationInMemory, e.LengthInMemory, e.LinkTimeAddress, e.DevicePath)
}

func (e *EFIImageLoadEvent) Write(w io.Writer) error {
	dpw := new(bytes.Buffer)
	if err := e.DevicePath.Write(dpw); err != nil {
		return xerrors.Errorf("cannot write device path: %w", err)
	}

	ev := rawEFIImageLoadEventHdr{
		LocationInMemory:   e.LocationInMemory,
		LengthInMemory:     e.LengthInMemory,
		LinkTimeAddress:    e.LinkTimeAddress,
		LengthOfDevicePath: uint64(dpw.Len())}
	if err := binary.Write(w, binary.LittleEndian, &ev); err != nil {
		return err
	}

	_, err := dpw.WriteTo(w)
	return err
}

// https://trustedcomputinggroup.org/wp-content/uploads/TCG_EFI_Platform_1_22_Final_-v15.pdf (section 4 "Measuring PE/COFF Image Files")
// https://trustedcomputinggroup.org/wp-content/uploads/TCG_PCClientSpecPlat_TPM_2p0_1p04_pub.pdf (section 9.2.3 "UEFI_IMAGE_LOAD_EVENT Structure")
func decodeEventDataEFIImageLoad(data []byte) (*EFIImageLoadEvent, error) {
	r := bytes.NewReader(data)

	var e rawEFIImageLoadEventHdr
	if err := binary.Read(r, binary.LittleEndian, &e); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	lr := io.LimitReader(r, int64(e.LengthOfDevicePath))
	path, err := efi.ReadDevicePath(lr)
	if err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	return &EFIImageLoadEvent{
		rawEventData:     data,
		LocationInMemory: e.LocationInMemory,
		LengthInMemory:   e.LengthInMemory,
		LinkTimeAddress:  e.LinkTimeAddress,
		DevicePath:       path}, nil
}

// EFIGPTData corresponds to UEFI_GPT_DATA and is the event data for EV_EFI_GPT_EVENT events.
type EFIGPTData struct {
	rawEventData
	Hdr        efi.PartitionTableHeader
	Partitions []*efi.PartitionEntry
}

func (e *EFIGPTData) String() string {
	var builder bytes.Buffer
	fmt.Fprintf(&builder, "UEFI_GPT_DATA{\n\tHdr: %s,\n\tPartitions: [", &e.Hdr)
	for _, part := range e.Partitions {
		fmt.Fprintf(&builder, "\n\t\t%s", part)
	}
	fmt.Fprintf(&builder, "\n\t]\n}")
	return builder.String()
}

func (e *EFIGPTData) Write(w io.Writer) error {
	if err := e.Hdr.Write(w); err != nil {
		return err
	}

	if err := binary.Write(w, binary.LittleEndian, uint64(len(e.Partitions))); err != nil {
		return err
	}

	for _, entry := range e.Partitions {
		w2 := new(bytes.Buffer)
		if err := entry.Write(w2); err != nil {
			return err
		}
		if w2.Len() > int(e.Hdr.SizeOfPartitionEntry) {
			return errors.New("SizeOfPartitionEntry too small")
		}

		b := make([]byte, e.Hdr.SizeOfPartitionEntry)
		copy(b, w2.Bytes())

		if _, err := w.Write(b); err != nil {
			return err
		}
	}

	return nil
}

func decodeEventDataEFIGPT(data []byte) (*EFIGPTData, error) {
	r := bytes.NewReader(data)

	d := &EFIGPTData{rawEventData: data}

	// UEFI_GPT_DATA.UEFIPartitionHeader
	hdr, err := efi.ReadPartitionTableHeader(r, false)
	if err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	d.Hdr = *hdr

	// UEFI_GPT_DATA.NumberOfPartitions
	var numberOfParts uint64
	if err := binary.Read(r, binary.LittleEndian, &numberOfParts); err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}

	if numberOfParts > math.MaxUint32 {
		return nil, errors.New("invalid EFI_GPT_DATA.NumberOfPartitons")
	}

	// UEFI_GPT_DATA.Partitions
	partitions, err := efi.ReadPartitionEntries(r, uint32(numberOfParts), hdr.SizeOfPartitionEntry)
	if err != nil {
		return nil, ioerr.EOFIsUnexpected(err)
	}
	d.Partitions = partitions

	return d, nil
}

// ComputeEFIGPTDataDigest computes a UEFI_GPT_DATA digest from the supplied data.
func ComputeEFIGPTDataDigest(alg crypto.Hash, data *EFIGPTData) ([]byte, error) {
	h := alg.New()
	if err := data.Write(h); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
