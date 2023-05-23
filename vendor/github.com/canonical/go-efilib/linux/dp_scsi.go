// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/canonical/go-efilib"

	"golang.org/x/xerrors"
)

// scsiRE matches a SCSI path, capturing the channel, target and LUN.
var scsiRE = regexp.MustCompile(`^host[[:digit:]]+\/target[[:digit:]]+\:[[:digit:]]+\:[[:digit:]]+\/[[:digit:]]+\:([[:digit:]]+)\:([[:digit:]]+)\:([[:digit:]]+)\/block\/s[dr][[:alpha:]]$`)

type scsiParams struct {
	channel uint32
	target  uint32
	lun     uint64
}

func handleSCSIPath(path string) (*scsiParams, error) {
	components := strings.Split(path, string(os.PathSeparator))
	if len(components) < 5 {
		return nil, errors.New("invalid path: insufficient components")
	}

	path = filepath.Join(components[len(components)-5:]...)
	m := scsiRE.FindStringSubmatch(path)
	if len(m) == 0 {
		return nil, fmt.Errorf("invalid path components: %s", path)
	}

	channel, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return nil, xerrors.Errorf("invalid channel: %w", err)
	}
	target, err := strconv.ParseUint(m[2], 10, 32)
	if err != nil {
		return nil, xerrors.Errorf("invalid target: %w", err)
	}
	lun, err := strconv.ParseUint(m[3], 10, 64)
	if err != nil {
		return nil, xerrors.Errorf("invalid lun: %w", err)
	}

	return &scsiParams{
		channel: uint32(channel),
		target:  uint32(target),
		lun:     lun}, nil
}

func handleSCSIDevicePathNode(builder devicePathBuilder) error {
	if builder.numRemaining() < 5 {
		return errors.New("invalid path: insufficient components")
	}

	params, err := handleSCSIPath(builder.absPath(builder.next(5)))
	if err != nil {
		return err
	}

	builder.advance(5)

	if params.channel != 0 {
		return errors.New("invalid channel")
	}
	if params.target > math.MaxUint16 {
		return errors.New("invalid target")
	}
	if params.lun > math.MaxUint16 {
		return errors.New("invalid LUN")
	}

	builder.append(&efi.SCSIDevicePathNode{
		PUN: uint16(params.target),
		LUN: uint16(params.lun)})
	return nil
}

func init() {
	registerDevicePathNodeHandler("scsi", handleSCSIDevicePathNode, 0, interfaceTypeSCSI)
}
