// Copyright © 2019-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/template"
	"github.com/mudler/luet/pkg/api/core/types"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"github.com/pkg/errors"
)

const (
	CompilerDefinitionFile = "build.yaml"
)

var DefaultCompilerParsers = []FileParser{
	BuildCollectionParser,
	BuildDefinitionParser,
}

func NewCompilerRecipe(d types.PackageDatabase, fp ...FileParser) Builder {
	if len(fp) == 0 {
		fp = DefaultCompilerParsers
	}
	return &CompilerRecipe{Recipe: Recipe{Database: d}, fileParsers: fp}
}

func ReadDefinitionFile(path string) (types.Package, error) {
	empty := types.Package{}
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return empty, errors.Wrap(err, "Error reading file "+path)
	}
	pack, err := types.PackageFromYaml(dat)
	if err != nil {
		return empty, errors.Wrap(err, "Error reading yaml "+path)
	}

	return pack, nil
}

// Recipe is the "general" reciper for Trees
type CompilerRecipe struct {
	fileParsers []FileParser
	Recipe
}

// CompilerRecipes copies tree 1:1 as they contain the specs
// and the build context required for reproducible builds
func (r *CompilerRecipe) Save(path string) error {
	for _, p := range r.SourcePath {
		if err := fileHelper.CopyDir(p, filepath.Join(path, filepath.Base(p))); err != nil {
			return errors.Wrap(err, "while copying source tree")
		}
	}
	return nil
}

func (r *CompilerRecipe) Load(path string) error {

	r.SourcePath = append(r.SourcePath, path)

	c, err := template.FilesInDir(template.FindPossibleTemplatesDir(path))
	if err != nil {
		return err
	}

	var ff = func(currentpath string, info os.FileInfo, err error) error {

		if err != nil {
			return errors.Wrap(err, "Error on walk path "+currentpath)
		}

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

func (r *CompilerRecipe) GetDatabase() types.PackageDatabase   { return r.Database }
func (r *CompilerRecipe) WithDatabase(d types.PackageDatabase) { r.Database = d }
func (r *CompilerRecipe) GetSourcePath() []string              { return r.SourcePath }
