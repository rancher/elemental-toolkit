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

package installer

import (
	"github.com/mudler/luet/pkg/api/core/types"
	artifact "github.com/mudler/luet/pkg/api/core/types/artifact"
	"github.com/mudler/luet/pkg/tree"
	//"github.com/mudler/luet/pkg/solver"
)

type Client interface {
	DownloadArtifact(*artifact.PackageArtifact) (*artifact.PackageArtifact, error)
	DownloadFile(string) (string, error)
	CacheGet(*artifact.PackageArtifact) (*artifact.PackageArtifact, error)
}

type Repositories []*LuetSystemRepository

type Repository interface {
	GetTree() tree.Builder
	Client(types.Context) Client
	GetName() string
}
