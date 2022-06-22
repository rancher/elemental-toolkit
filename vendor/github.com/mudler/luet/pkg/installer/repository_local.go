// Copyright © 2019-2021 Ettore Di Giacinto <mudler@sabayon.org>
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

package installer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mudler/luet/pkg/api/core/types"
	artifact "github.com/mudler/luet/pkg/api/core/types/artifact"

	"github.com/mudler/luet/pkg/api/core/bus"
	"github.com/pkg/errors"
)

type localRepositoryGenerator struct {
	context    types.Context
	snapshotID string
}

func (l *localRepositoryGenerator) Initialize(path string, db types.PackageDatabase) ([]*artifact.PackageArtifact, error) {
	return buildPackageIndex(l.context, path, db)
}

func buildPackageIndex(ctx types.Context, path string, db types.PackageDatabase) ([]*artifact.PackageArtifact, error) {

	var art []*artifact.PackageArtifact
	var ff = func(currentpath string, info os.FileInfo, err error) error {
		if err != nil {
			ctx.Debug("Failed walking", err.Error())
			return err
		}

		if !strings.HasSuffix(info.Name(), ".metadata.yaml") {
			return nil // Skip with no errors
		}

		dat, err := ioutil.ReadFile(currentpath)
		if err != nil {
			return errors.Wrap(err, "Error reading file "+currentpath)
		}

		a, err := artifact.NewPackageArtifactFromYaml(dat)
		if err != nil {
			return errors.Wrap(err, "Error reading yaml "+currentpath)
		}

		// We want to include packages that are ONLY referenced in the tree.
		// the ones which aren't should be deleted. (TODO: by another cli command?)
		if _, notfound := db.FindPackage(a.CompileSpec.GetPackage()); notfound != nil {
			ctx.Debug(fmt.Sprintf("Package %s not found in tree. Ignoring it.",
				a.CompileSpec.GetPackage().HumanReadableString()))
			return nil
		}

		art = append(art, a)

		return nil
	}

	err := filepath.Walk(path, ff)
	if err != nil {
		return nil, err

	}
	return art, nil
}

// Generate creates a Local luet repository
func (g *localRepositoryGenerator) Generate(r *LuetSystemRepository, dst string, resetRevision bool) error {
	err := os.MkdirAll(dst, os.ModePerm)
	if err != nil {
		return err
	}
	r.LastUpdate = strconv.FormatInt(time.Now().Unix(), 10)

	repospec := filepath.Join(dst, REPOSITORY_SPECFILE)
	// Increment the internal revision version by reading the one which is already available (if any)
	if err := r.BumpRevision(repospec, resetRevision); err != nil {
		return err
	}

	g.context.Info(fmt.Sprintf(
		"Repository %s: creating revision %d and last update %s...",
		r.Name, r.Revision, r.LastUpdate,
	))

	bus.Manager.Publish(bus.EventRepositoryPreBuild, struct {
		Repo LuetSystemRepository
		Path string
	}{
		Repo: *r,
		Path: dst,
	})

	if _, err := r.AddTree(g.context, r.GetTree(), dst, REPOFILE_TREE_KEY, NewDefaultTreeRepositoryFile()); err != nil {
		return errors.Wrap(err, "error met while adding runtime tree to repository")
	}

	if _, err := r.AddTree(g.context, r.BuildTree, dst, REPOFILE_COMPILER_TREE_KEY, NewDefaultCompilerTreeRepositoryFile()); err != nil {
		return errors.Wrap(err, "error met while adding compiler tree to repository")
	}

	if _, err := r.AddMetadata(g.context, repospec, dst); err != nil {
		return errors.Wrap(err, "failed adding Metadata file to repository")
	}

	// Create named snapshot.
	// It edits the metadata pointing at the repository files associated with the snapshot
	// And copies the new files
	if _, _, err := r.Snapshot(g.snapshotID, dst); err != nil {
		return errors.Wrap(err, "while creating snapshot")
	}

	bus.Manager.Publish(bus.EventRepositoryPostBuild, struct {
		Repo LuetSystemRepository
		Path string
	}{
		Repo: *r,
		Path: dst,
	})
	return nil
}
