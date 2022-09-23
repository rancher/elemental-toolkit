// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2021 Canonical Ltd
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

package secboot

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"

	"github.com/snapcore/snapd/osutil"
	"github.com/snapcore/snapd/osutil/sys"

	"golang.org/x/xerrors"
)

// FileKeyDataReader provides a mechanism to read a KeyData from a file.
type FileKeyDataReader struct {
	readableName string
	*bytes.Reader
}

func (r *FileKeyDataReader) ReadableName() string {
	return r.readableName
}

// NewFileKeyDataReader is used to read a file containing key data at the specified path.
func NewFileKeyDataReader(path string) (*FileKeyDataReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, xerrors.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	d, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, xerrors.Errorf("cannot read file: %w", err)
	}

	return &FileKeyDataReader{path, bytes.NewReader(d)}, nil
}

// FileKeyDataWriter provides a mechanism to write a KeyData to a file.
type FileKeyDataWriter struct {
	path string
	*bytes.Buffer
}

func (w *FileKeyDataWriter) Commit() error {
	f, err := osutil.NewAtomicFile(w.path, 0600, 0, sys.UserID(osutil.NoChown), sys.GroupID(osutil.NoChown))
	if err != nil {
		return xerrors.Errorf("cannot create new atomic file: %w", err)
	}
	defer f.Cancel()

	if _, err := io.Copy(f, w); err != nil {
		return xerrors.Errorf("cannot write file key data: %w", err)
	}

	if err := f.Commit(); err != nil {
		return xerrors.Errorf("cannot commit update: %w", err)
	}

	return nil
}

// NewFileKeyDataWriter creates a new FileKeyDataWriter for atomically writing a
// KeyData to a file.
func NewFileKeyDataWriter(path string) *FileKeyDataWriter {
	return &FileKeyDataWriter{path, new(bytes.Buffer)}
}
