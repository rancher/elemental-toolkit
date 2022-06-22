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

// InstallerRecipe is a builder imeplementation.

// It reads a Tree and spit it in human readable form (YAML), called recipe,
// It also loads a tree (recipe) from a YAML (to a db, e.g. BoltDB), allowing to query it
// with the solver, using the package object.
package tree

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/template"
	"github.com/mudler/luet/pkg/api/core/types"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	"github.com/pkg/errors"
)

const (
	FinalizerFile = "finalize.yaml"
)

var DefaultInstallerParsers = []FileParser{
	RuntimeCollectionParser,
	RuntimeDefinitionParser,
}

func NewInstallerRecipe(db types.PackageDatabase, fp ...FileParser) Builder {
	if len(fp) == 0 {
		fp = DefaultInstallerParsers
	}
	return &InstallerRecipe{Database: db, fileParsers: fp}
}

// InstallerRecipe is the "general" reciper for Trees
type InstallerRecipe struct {
	SourcePath  []string
	Database    types.PackageDatabase
	fileParsers []FileParser
}

func (r *InstallerRecipe) Save(path string) error {

	for _, p := range r.Database.World() {

		dir := filepath.Join(path, p.GetCategory(), p.GetName(), p.GetVersion())
		os.MkdirAll(dir, os.ModePerm)
		data, err := p.Yaml()
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(dir, types.PackageDefinitionFile), data, 0644)
		if err != nil {
			return err
		}
		// Instead of rdeps, have a different tree for build deps.
		finalizerPath := p.Rel(FinalizerFile)
		if fileHelper.Exists(finalizerPath) { // copy finalizer file from the source tree
			fileHelper.CopyFile(finalizerPath, filepath.Join(dir, FinalizerFile))
		}

	}
	return nil
}

func (r *InstallerRecipe) Load(path string) error {

	if !fileHelper.Exists(path) {
		return errors.New(fmt.Sprintf(
			"Path %s doesn't exit.", path,
		))
	}

	r.SourcePath = append(r.SourcePath, path)

	c, err := template.FilesInDir(template.FindPossibleTemplatesDir(path))
	if err != nil {
		return err
	}
	//r.Tree().SetPackageSet(pkg.NewBoltDatabase(tmpfile.Name()))
	// TODO: Handle cleaning after? Cleanup implemented in GetPackageSet().Clean()

	// the function that handles each file or dir
	var ff = func(currentpath string, info os.FileInfo, err error) error {
		for _, p := range r.fileParsers {
			if err := p(path, currentpath, info.Name(), c, r.Database); err != nil {
				return err
			}
		}
		return nil
	}

	err = filepath.Walk(path, ff)
	if err != nil {
		return err
	}
	return nil
}

func (r *InstallerRecipe) GetDatabase() types.PackageDatabase   { return r.Database }
func (r *InstallerRecipe) WithDatabase(d types.PackageDatabase) { r.Database = d }
func (r *InstallerRecipe) GetSourcePath() []string              { return r.SourcePath }
