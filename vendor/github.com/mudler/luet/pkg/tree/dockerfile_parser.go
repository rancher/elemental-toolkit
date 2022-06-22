// Copyright © 2022 Ettore Di Giacinto <mudler@luet.io>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package tree

import (
	"path/filepath"
	"strings"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

func RuntimeDockerfileParser(srcDir, currentpath, name string, templates []string, db types.PackageDatabase) error {
	if !strings.Contains(name, "Dockerfile") {
		return nil
	}

	// Path is set only internally when tree is loaded from disk
	_, err := db.CreatePackage(&types.Package{Name: filepath.Base(filepath.Dir(currentpath)), Path: filepath.Dir(currentpath), TreeDir: srcDir})
	if err != nil {
		return errors.Wrap(err, "Error creating package "+currentpath)
	}
	return nil
}

func BuildDockerfileParser(srcDir, currentpath, name string, templates []string, db types.PackageDatabase) error {
	if !strings.Contains(name, "Dockerfile") {
		return nil
	}

	// Simply imply the name package from the directory name
	// TODO: Read specific labels from dockerfile as we do read the image already
	p := &types.Package{
		Name:    filepath.Base(filepath.Dir(currentpath)),
		Path:    filepath.Dir(currentpath),
		TreeDir: srcDir}

	err := p.SetOriginalDockerfile(currentpath)
	if err != nil {
		return errors.Wrap(err, "Error reading file "+currentpath)
	}

	// Path is set only internally when tree is loaded from disk
	_, err = db.CreatePackage(p)
	if err != nil {
		return errors.Wrap(err, "Error creating package "+currentpath)
	}
	return nil
}
