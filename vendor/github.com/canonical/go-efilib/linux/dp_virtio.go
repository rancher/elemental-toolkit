// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	virtioBlockDevice uint32 = 2
	virtioSCSIHost    uint32 = 8
)

var (
	blkRE    = regexp.MustCompile(`^block\/vd[[:alpha:]]$`)
	virtioRE = regexp.MustCompile(`^virtio[[:digit:]]`)
)

func handleVirtioDevicePathNode(state *devicePathBuilderState) error {
	if !virtioRE.MatchString(state.PeekUnhandledSysfsComponents(1)) {
		return errors.New("invalid path")
	}

	state.AdvanceSysfsPath(1)

	data, err := os.ReadFile(filepath.Join(state.SysfsPath(), "modalias"))
	if err != nil {
		return err
	}

	var device uint32
	var vendor uint32
	n, err := fmt.Sscanf(strings.TrimSpace(string(data)), "virtio:d%08xv%08x", &device, &vendor)
	if err != nil {
		return fmt.Errorf("cannot scan modalias: %w", err)
	}
	if n != 2 {
		return errors.New("invalid modalias format")
	}

	switch device {
	case virtioBlockDevice:
		if !blkRE.MatchString(state.PeekUnhandledSysfsComponents(2)) {
			return errors.New("invalid path for block device")
		}
		state.AdvanceSysfsPath(2)
	case virtioSCSIHost:
		state.Interface = interfaceTypeSCSI
	default:
		return fmt.Errorf("unrecognized virtio device type %#08x", device)
	}

	return nil
}
