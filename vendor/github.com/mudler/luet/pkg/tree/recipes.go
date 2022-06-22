// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
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

// Recipe is a builder imeplementation.

// It reads a Tree and spit it in human readable form (YAML), called recipe,
// It also loads a tree (recipe) from a YAML (to a db, e.g. BoltDB), allowing to query it
// with the solver, using the package object.
package tree

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	spectooling "github.com/mudler/luet/pkg/spectooling"

	"github.com/pkg/errors"
)

func NewGeneralRecipe(db types.PackageDatabase, fp ...FileParser) Builder {
	if len(fp) == 0 {
		fp = DefaultInstallerParsers
	}
	return &Recipe{Database: db, fileParsers: fp}
}

// Recipe is the "general" reciper for Trees
type Recipe struct {
	fileParsers []FileParser

	SourcePath []string
	Database   types.PackageDatabase
}

func WriteDefinitionFile(p *types.Package, definitionFilePath string) error {
	data, err := spectooling.NewDefaultPackageSanitized(p).Yaml()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(definitionFilePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (r *Recipe) Save(path string) error {
	for _, p := range r.Database.World() {
		dir := filepath.Join(path, p.GetCategory(), p.GetName(), p.GetVersion())
		os.MkdirAll(dir, os.ModePerm)

		err := WriteDefinitionFile(p, filepath.Join(dir, types.PackageDefinitionFile))
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Recipe) Load(path string) error {

	// tmpfile, err := ioutil.TempFile("", "luet")
	// if err != nil {
	// 	return err
	// }
	if !fileHelper.Exists(path) {
		return errors.New(fmt.Sprintf(
			"Path %s doesn't exit.", path,
		))
	}

	r.SourcePath = append(r.SourcePath, path)

	if r.Database == nil {
		r.Database = pkg.NewInMemoryDatabase(false)
	}

	var ff = func(currentpath string, info os.FileInfo, err error) error {
		for _, p := range r.fileParsers {
			if err := p(path, currentpath, info.Name(), []string{}, r.Database); err != nil {
				return err
			}
		}
		return nil
	}
	err := filepath.Walk(path, ff)
	if err != nil {
		return err
	}
	return nil
}

func (r *Recipe) GetDatabase() types.PackageDatabase   { return r.Database }
func (r *Recipe) WithDatabase(d types.PackageDatabase) { r.Database = d }
func (r *Recipe) GetSourcePath() []string              { return r.SourcePath }
