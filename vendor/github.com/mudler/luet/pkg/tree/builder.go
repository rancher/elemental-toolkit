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

package tree

import "github.com/mudler/luet/pkg/api/core/types"

// reads a luet tree and generates the package lists
type Builder interface {
	Save(string) error // A tree might be saved to a folder structure (human editable)
	Load(string) error // A tree might be loaded from a db (e.g. bolt) and written to folder
	GetDatabase() types.PackageDatabase
	WithDatabase(d types.PackageDatabase)

	GetSourcePath() []string
}
