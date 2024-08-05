// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	efi "github.com/canonical/go-efilib"
)

func init() {
	registerDevicePathNodeHandler("pci-root", handlePCIRootDevicePathNode)
	registerDevicePathNodeHandler("vmbus-root", handleVMBusRootDevicePathNode)
	registerDevicePathNodeHandler("virtual", handleVirtualDevicePathNode)
	registerDevicePathNodeHandler("acpi", handleACPIDevicePathNode)

	registerDevicePathNodeHandler("pci", handlePCIDevicePathNode, interfaceTypePCI)

	registerDevicePathNodeHandler("scsi", handleSCSIDevicePathNode, interfaceTypeSCSI)

	registerDevicePathNodeHandler("ide", handleIDEDevicePathNode, interfaceTypeIDE)

	registerDevicePathNodeHandler("sata", handleSATADevicePathNode, interfaceTypeSATA)

	registerDevicePathNodeHandler("nvme", handleNVMEDevicePathNode, interfaceTypeNVME)

	registerDevicePathNodeHandler("hv", handleHVDevicePathNode, interfaceTypeVMBus)

	registerDevicePathNodeHandler("virtio", handleVirtioDevicePathNode, interfaceTypeVirtio)

}

// FilePathToDevicePathMode specifies the mode for FilePathToDevicePath
type FilePathToDevicePathMode int

const (
	// FullPath indicates that only a full device path should be created.
	FullPath FilePathToDevicePathMode = iota

	// ShortFormPathHD indicates that a short-form device path beginning
	// with a HD() node should be created.
	ShortFormPathHD

	// ShortFormPathFile indicates that a short-form device path consisting
	// of only the file path relative to the device should be created.
	ShortFormPathFile
)

// ErrNoDevicePath is returned from FilePathToDevicePath if the device in
// which a file is stored cannot be mapped to a device path with the
// specified mode.
type ErrNoDevicePath string

func (e ErrNoDevicePath) Error() string {
	return "cannot map file path to a UEFI device path: " + string(e)
}

type interfaceType int

const (
	interfaceTypeUnknown interfaceType = iota
	interfaceTypePCI
	interfaceTypeUSB
	interfaceTypeSCSI
	interfaceTypeIDE
	interfaceTypeSATA
	interfaceTypeNVME
	interfaceTypeVMBus
	interfaceTypeVirtio
)

var (
	// errSkipDevicePathNodeHandler is returned from a handler when it
	// wants to defer handling to another handler.
	errSkipDevicePathNodeHandler = errors.New("")
)

// errUnsupportedDevice is returned from a handler when it cannot
// determine the interface.
type errUnsupportedDevice string

func (e errUnsupportedDevice) Error() string {
	return "unsupported device: " + string(e)
}

type devicePathNodeHandler func(*devicePathBuilderState) error

type registeredDpHandler struct {
	name string
	fn   devicePathNodeHandler
}

var devicePathNodeHandlers = make(map[interfaceType][]registeredDpHandler)

func registerDevicePathNodeHandler(name string, fn devicePathNodeHandler, interfaces ...interfaceType) {
	if len(interfaces) == 0 {
		interfaces = []interfaceType{interfaceTypeUnknown}
	}
	for _, i := range interfaces {
		devicePathNodeHandlers[i] = append(devicePathNodeHandlers[i], registeredDpHandler{name, fn})
	}
}

type devicePathBuilderState struct {
	Interface interfaceType
	Path      efi.DevicePath

	processed []string
	remaining []string
}

func (s *devicePathBuilderState) SysfsPath() string {
	return filepath.Join(append([]string{sysfsPath, "devices"}, s.processed...)...)
}

func (s *devicePathBuilderState) SysfsComponentsRemaining() int {
	return len(s.remaining)
}

func (s *devicePathBuilderState) PeekUnhandledSysfsComponents(n int) string {
	if n < 0 {
		n = len(s.remaining)
	}
	if n > len(s.remaining) {
		n = len(s.remaining)
	}
	return filepath.Join(s.remaining[:n]...)
}

func (s *devicePathBuilderState) AdvanceSysfsPath(n int) {
	if n < 0 {
		n = len(s.remaining)
	}
	if n > len(s.remaining) {
		n = len(s.remaining)
	}
	s.processed = append(s.processed, s.remaining[:n]...)
	s.remaining = s.remaining[n:]
}

type devicePathBuilder struct {
	devicePathBuilderState
}

func (s *devicePathBuilder) done() bool {
	return len(s.remaining) == 0
}

func (b *devicePathBuilder) ProcessNextComponent() error {
	nProcessed := len(b.processed)
	remaining := b.remaining
	iface := b.Interface

	handlers := devicePathNodeHandlers[b.Interface]
	if len(handlers) == 0 {
		// There should always be at least one handler registered for an interface.
		panic(fmt.Sprintf("no handlers registered for interface type %v", b.Interface))
	}

	for _, handler := range handlers {
		err := handler.fn(&b.devicePathBuilderState)
		if err != nil {
			// Roll back changes
			b.processed = b.processed[:nProcessed]
			b.remaining = remaining
			b.Interface = iface
		}
		if err == errSkipDevicePathNodeHandler {
			// Try the next handler.
			continue
		}
		if err != nil {
			return fmt.Errorf("[handler %s]: %w", handler.name, err)
		}

		if iface != interfaceTypeUnknown && b.Interface == interfaceTypeUnknown {
			// The handler set the interface type back to unknown. Turn this
			// in to a errUnsupportedDevice error.
			return errUnsupportedDevice("[handler " + handler.name + "]: unrecognized interface")
		}
		return nil
	}

	// If we get here, then all handlers returned errSkipDevicePathNodeHandler.

	if b.Interface != interfaceTypeUnknown {
		// If the interface has already been determined, require at least one
		// handler to handle this node or return an error.
		panic(fmt.Sprintf("all handlers skipped handling interface type %v", b.Interface))
	}

	return errUnsupportedDevice("unhandled root node")
}

func newDevicePathBuilder(dev *dev) (*devicePathBuilder, error) {
	path, err := filepath.Rel(filepath.Join(sysfsPath, "devices"), dev.sysfsPath)
	if err != nil {
		return nil, err
	}

	return &devicePathBuilder{
		devicePathBuilderState: devicePathBuilderState{
			remaining: strings.Split(path, string(os.PathSeparator))}}, nil
}

type mountPoint struct {
	dev         uint64
	root        string
	mountDir    string
	mountSource string
}

func scanBlockDeviceMounts() (mounts []*mountPoint, err error) {
	f, err := os.Open(mountsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 || len(fields) > 11 {
			return nil, errors.New("invalid mount info: incorrect number of fields")
		}

		devStr := strings.Split(fields[2], ":")
		if len(devStr) != 2 {
			return nil, errors.New("invalid mount info: invalid device number")
		}
		devMajor, err := strconv.ParseUint(devStr[0], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid mount info: invalid device number: %w", err)
		}
		devMinor, err := strconv.ParseUint(devStr[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid mount info: invalid device number: %w", err)
		}

		var mountSource string
		if len(fields) == 10 {
			mountSource = fields[8]
		} else {
			mountSource = fields[9]
		}
		if !filepath.IsAbs(mountSource) {
			continue
		}

		mounts = append(mounts, &mountPoint{
			dev:         unix.Mkdev(uint32(devMajor), uint32(devMinor)),
			root:        fields[3],
			mountDir:    fields[4],
			mountSource: mountSource})
	}
	if scanner.Err() != nil {
		return nil, fmt.Errorf("cannot parse mount info: %w", err)
	}

	return mounts, nil
}

func getFileMountPoint(path string) (*mountPoint, error) {
	var st unix.Stat_t
	if err := unixStat(path, &st); err != nil {
		return nil, fmt.Errorf("cannot stat %s: %w", path, err)
	}

	mounts, err := scanBlockDeviceMounts()
	if err != nil {
		return nil, fmt.Errorf("cannot obtain list of block device mounts: %w", err)
	}

	var candidate *mountPoint

	for _, mount := range mounts {
		if mount.dev != st.Dev {
			continue
		}

		rel, err := filepath.Rel(mount.mountDir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if candidate == nil {
			candidate = mount
		}
		if len(mount.mountDir) > len(candidate.mountDir) {
			candidate = mount
		}
	}

	if candidate == nil {
		return nil, errors.New("not found")
	}

	return candidate, nil
}

type dev struct {
	sysfsPath string
	devPath   string
	part      int
}

type filePath struct {
	dev
	path string
}

func newFilePath(path string) (*filePath, error) {
	path, err := filepathEvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate symbolic links: %w", err)
	}

	mount, err := getFileMountPoint(path)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain mount information for path: %w", err)
	}

	rel, err := filepath.Rel(mount.mountDir, path)
	if err != nil {
		return nil, err
	}
	out := &filePath{path: filepath.Join(mount.root, rel)}

	childDev, err := filepath.EvalSymlinks(filepath.Join(sysfsPath, "dev/block", fmt.Sprintf("%d:%d", unix.Major(mount.dev), unix.Minor(mount.dev))))
	if err != nil {
		return nil, err
	}

	parentDev := filepath.Dir(childDev)
	parentSubsystem, err := filepath.EvalSymlinks(filepath.Join(parentDev, "subsystem"))
	switch {
	case os.IsNotExist(err):
		// No subsystem link, could be the block/ directory
	case err != nil:
		return nil, err
	}

	if parentSubsystem != filepath.Join(sysfsPath, "class", "block") {
		// Parent device is not a block device
		out.dev.sysfsPath = childDev
		out.dev.devPath = filepath.Join("/dev", filepath.Base(childDev))
	} else {
		// Parent device is a block device, so this is a partitioned
		// device.
		out.dev.sysfsPath = parentDev
		out.dev.devPath = filepath.Join("/dev", filepath.Base(parentDev))
		b, err := os.ReadFile(filepath.Join(childDev, "partition"))
		if err != nil {
			return nil, fmt.Errorf("cannot obtain partition number for %d: %w", mount.dev, err)
		}
		part, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return nil, fmt.Errorf("cannot determine partition number for %d: %w", mount.dev, err)
		}
		out.dev.part = part
	}

	return out, nil
}

// FilePathToDevicePath creates an EFI device path from the supplied filepath.
//
// If mode is FullPath, this will attempt to create a full device path which
// requires the use of sysfs. If the device in which the file is stored cannot be
// mapped to a device path, a ErrNoDevicePath error is returned. This could be
// because the device is not recognized by this package, or because the device
// genuinely cannot be mapped to a device path (eg, it is a device-mapper or loop
// device). In this case, one of the ShortForm modes can be used.
//
// If mode is ShortFormPathHD, this will attempt to create a short-form device
// path beginning with a HD() component. If the file is stored inside an
// unpartitioned device, a ErrNoDevicePath error will be returned. In this case,
// ShortFormPathFile can be used.
//
// When mode is ShortFormPathHD or FullPath and the file is stored inside a
// partitoned device, read access is required on the underlying block device
// in order to decode the partition table.
//
// If mode is ShortFormPathFile, this will attempt to create a short-form device
// path consisting only of the file path relative to the device.
//
// In all modes, read access to the file's directory is required.
func FilePathToDevicePath(path string, mode FilePathToDevicePathMode) (out efi.DevicePath, err error) {
	fp, err := newFilePath(path)
	if err != nil {
		return nil, err
	}

	if mode == ShortFormPathHD && fp.part == 0 {
		return nil, ErrNoDevicePath("file is not inside partitioned media - use linux.ShortFormPathFile")
	}

	builder, err := newDevicePathBuilder(&fp.dev)
	if err != nil {
		return nil, err
	}

	if mode == FullPath {
		for !builder.done() {
			var e errUnsupportedDevice

			err := builder.ProcessNextComponent()
			switch {
			case errors.As(err, &e):
				return nil, ErrNoDevicePath("encountered an error when handling components " +
					builder.PeekUnhandledSysfsComponents(-1) + " from device path " +
					builder.SysfsPath() + ": " + err.Error())
			case err != nil:
				return nil, fmt.Errorf("cannot process components %s from device path %s: %w",
					builder.PeekUnhandledSysfsComponents(-1), builder.SysfsPath(), err)
			}
		}
	}

	out = builder.Path

	if mode != ShortFormPathFile && fp.part > 0 {
		node, err := NewHardDriveDevicePathNodeFromDevice(fp.devPath, fp.part)
		if err != nil {
			return nil, fmt.Errorf("cannot construct hard drive device path node: %w", err)
		}
		out = append(out, node)
	}

	out = append(out, efi.NewFilePathDevicePathNode(fp.path))
	return out, err
}
