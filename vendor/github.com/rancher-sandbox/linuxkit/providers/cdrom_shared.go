package providers

import (
	"fmt"
	"github.com/diskfs/go-diskfs"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	cdromDevs = "/dev/sr[0-9]*"
	blockDevs = "/sys/class/block/*"
)

// ProviderCDROM is the type implementing the Provider interface for CDROMs (nocloud/config-drive)
type ProviderCDROM struct {
	providerType string
	device       string
	mountPoint   string
	err          error
	userdata     []byte
}

func (p *ProviderCDROM) String() string {
	return fmt.Sprintf("%s(%s)", p.providerType, p.device)
}

// Probe checks if the CD has the right file
func (p *ProviderCDROM) Probe() bool {
	if p.err != nil {
		log.Errorf("there were errors probing %s: %v", p.device, p.err)
	}
	return len(p.userdata) != 0
}

// Extract gets both the CDROM specific and generic userdata
func (p *ProviderCDROM) Extract() ([]byte, error) {
	return p.userdata, p.err
}

// mount mounts a CDROM/DVD device under mountPoint
func (p *ProviderCDROM) mount() error {
	var err error
	// We may need to poll a little for device ready
	errISO := syscall.Mount(p.device, p.mountPoint, "iso9660", syscall.MS_RDONLY, "")
	if errISO != nil {
		errFat := syscall.Mount(p.device, p.mountPoint, "vfat", syscall.MS_RDONLY, "")
		if errFat != nil {
			err = fmt.Errorf("failed mounting %s: %v %v", p.device, errISO, errFat)
			p.err = err
		}
	}
	return err
}

// unmount removes the mount
func (p *ProviderCDROM) unmount() {
	_ = syscall.Unmount(p.mountPoint, 0)
}

// uniqueString returns a unique subset of the string slice provided.
func uniqueString(input []string) []string {
	u := make([]string, 0, len(input))
	m := make(map[string]bool)

	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}

	return u
}

func NewProviderCDROM(device string, datafiles []string, providerType string) *ProviderCDROM {
	mountPoint, err := os.MkdirTemp("", "cd")
	p := ProviderCDROM{providerType, device, mountPoint, err, []byte{}}
	if err == nil {
		if p.err = p.mount(); p.err == nil {
			defer p.unmount()
			// read the userdata - we read the spec file and the fallback, but eventually
			// will remove the fallback
			for _, f := range datafiles {
				userdata, err := os.ReadFile(path.Join(p.mountPoint, f))
				// did we find a file?
				if err == nil && userdata != nil {
					p.userdata = userdata
					break
				}
			}
			if p.userdata == nil {
				log.Debugf("no userdata file found at any of %v", datafiles)
			}
		}
	}
	return &p
}

// FindCIs goes through all known devices and checks for the given label
// https://github.com/canonical/cloud-init/blob/main/doc/rtd/reference/datasources/configdrive.rst
// https://github.com/canonical/cloud-init/blob/master/doc/rtd/topics/datasources/nocloud.rst
func FindCIs(findLabel string) []string {
	devs, err := filepath.Glob(blockDevs)
	log.Debugf("block devices found: %v", devs)
	if err != nil {
		// Glob can only error on invalid pattern
		panic(fmt.Sprintf("Invalid glob pattern: %s", blockDevs))
	}
	foundDevices := []string{}
	for _, device := range devs {
		// get the base device name
		dev := filepath.Base(device)
		// ignore loop and ram devices
		if strings.HasPrefix(dev, "loop") || strings.HasPrefix(dev, "ram") {
			log.Debugf("ignoring loop or ram device: %s", dev)
			continue
		}
		dev = fmt.Sprintf("/dev/%s", dev)
		log.Debugf("checking device: %s", dev)
		// open readonly, ignore errors
		disk, err := diskfs.Open(dev, diskfs.WithOpenMode(diskfs.ReadOnly))
		if err != nil {
			log.Debugf("failed to open device read-only: %s: %v", dev, err)
			continue
		}
		disk.DefaultBlocks = true
		fs, err := disk.GetFilesystem(0)
		if err != nil {
			log.Debugf("failed to get filesystem on partition 0 for device: %s: %v", dev, err)
			_ = disk.File.Close()
			continue
		}
		// get the label
		label := strings.TrimSpace(fs.Label())
		log.Debugf("found trimmed filesystem label for device: %s: '%s'", dev, label)
		if label == strings.ToUpper(findLabel) || label == strings.ToLower(findLabel) {
			log.Debugf("adding device: %s", dev)
			foundDevices = append(foundDevices, dev)
		}
		err = disk.File.Close()
		if err != nil {
			log.Debugf("failed closing device %s", dev)
		}
	}
	return foundDevices
}
