// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
)

// ataRE matches an ATA path component, capturing the ATA print ID.
var ataRE = regexp.MustCompile(`^ata([[:digit:]]+)$`)

type ataParams struct {
	printId uint32
	port    uint32
	*scsiParams
}

func handleATAPath(path string) (*ataParams, error) {
	components := strings.Split(path, string(os.PathSeparator))
	if len(components) < 6 {
		return nil, errors.New("invalid path: insufficient components")
	}

	ata := components[len(components)-6]
	m := ataRE.FindStringSubmatch(ata)
	if len(m) == 0 {
		return nil, fmt.Errorf("invalid path component: %s", ata)
	}

	scsiParams, err := handleSCSIPath(path)
	if err != nil {
		return nil, err
	}

	printId, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return nil, xerrors.Errorf("invalid print ID: %w", err)
	}

	// Obtain the ATA port number local to this ATA controller. The kernel
	// creates one ata%d device per port (see drivers/ata/libata-core.c:ata_host_register).
	portBytes, err := ioutil.ReadFile(filepath.Join(path, "../../../../..", "ata_port", ata, "port_no"))
	if err != nil {
		return nil, xerrors.Errorf("cannot obtain port ID: %w", err)
	}
	port, err := strconv.ParseUint(strings.TrimSpace(string(portBytes)), 10, 16)
	if err != nil {
		return nil, xerrors.Errorf("invalid port ID: %w", err)
	}

	return &ataParams{
		printId:    uint32(printId),
		port:       uint32(port),
		scsiParams: scsiParams}, nil
}
