// Copyright © 2019-2022 Ettore Di Giacinto <mudler@luet.io>
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
	"io/ioutil"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/mudler/luet/pkg/api/core/template"
	"github.com/mudler/luet/pkg/api/core/types"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"github.com/pkg/errors"
)

func BuildCollectionParser(srcDir, currentpath, name string, templates []string, db types.PackageDatabase) error {
	if name != types.PackageCollectionFile {
		return nil
	}

	dat, err := ioutil.ReadFile(currentpath)
	if err != nil {
		return errors.Wrap(err, "Error reading file "+currentpath)
	}

	packs, err := types.PackagesFromYAML(dat)
	if err != nil {
		return errors.Wrap(err, "Error reading yaml "+currentpath)
	}

	packsRaw, err := types.GetRawPackages(dat)
	if err != nil {
		return errors.Wrap(err, "Error reading raw packages from "+currentpath)
	}

	for _, pack := range packs {
		pack.SetPath(filepath.Dir(currentpath))
		pack.SetTreeDir(srcDir)

		// Instead of rdeps, have a different tree for build deps.
		compileDefPath := pack.Rel(CompilerDefinitionFile)
		if fileHelper.Exists(compileDefPath) {

			raw := packsRaw.Find(pack)
			buildyaml, err := ioutil.ReadFile(compileDefPath)
			if err != nil {
				return errors.Wrap(err, "Error reading file "+currentpath)
			}
			dat, err := template.Render(append(template.ReadFiles(templates...), string(buildyaml)), raw, map[string]interface{}{})
			if err != nil {
				return errors.Wrap(err,
					"Error templating file "+CompilerDefinitionFile+" from "+
						filepath.Dir(currentpath))
			}

			packbuild, err := types.PackageFromYaml([]byte(dat))
			if err != nil {
				return errors.Wrap(err,
					"Error reading yaml "+CompilerDefinitionFile+" from "+
						filepath.Dir(currentpath))
			}
			pack.Requires(packbuild.GetRequires())

			pack.Conflicts(packbuild.GetConflicts())
		}

		_, err = db.CreatePackage(&pack)
		if err != nil {
			return errors.Wrap(err, "Error creating package "+pack.GetName())
		}
	}
	return nil
}

func RuntimeCollectionParser(srcDir, currentpath, name string, templates []string, db types.PackageDatabase) error {
	if name != types.PackageCollectionFile {
		return nil
	}

	dat, err := ioutil.ReadFile(currentpath)
	if err != nil {
		return errors.Wrap(err, "Error reading file "+currentpath)
	}
	packs, err := types.PackagesFromYAML(dat)
	if err != nil {
		return errors.Wrap(err, "Error reading yaml "+currentpath)
	}

	packsRaw, err := types.GetRawPackages(dat)
	if err != nil {
		return errors.Wrap(err, "Error reading raw packages from "+currentpath)
	}
	for _, p := range packs {
		// Path is set only internally when tree is loaded from disk
		p.SetPath(filepath.Dir(currentpath))
		_, err = db.CreatePackage(&p)
		if err != nil {
			return errors.Wrap(err, "Error creating package "+p.GetName())
		}

		compileDefPath := p.Rel(CompilerDefinitionFile)
		if fileHelper.Exists(compileDefPath) {
			raw := packsRaw.Find(p)
			buildyaml, err := ioutil.ReadFile(compileDefPath)
			if err != nil {
				return errors.Wrap(err, "Error reading file "+currentpath)
			}
			dat, err := template.Render(append(template.ReadFiles(templates...), string(buildyaml)), raw, map[string]interface{}{})
			if err != nil {
				return errors.Wrap(err,
					"Error templating file "+CompilerDefinitionFile+" from "+
						filepath.Dir(currentpath))
			}

			spec := &types.LuetCompilationSpec{}
			if err := yaml.Unmarshal([]byte(dat), spec); err != nil {
				return err
			}

			for _, s := range spec.SubPackages {

				_, err = db.CreatePackage(s.Package)
				if err != nil {
					return errors.Wrap(err, "Error creating package "+p.GetName())
				}
			}
		}
	}
	return nil
}
