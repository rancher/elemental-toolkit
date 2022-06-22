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

import (
	"github.com/mudler/luet/pkg/api/core/types"
)

// parses ebuilds (?) and generates data which is readable by the builder
type Parser interface {
	Generate(string) (types.PackageDatabase, error) // Generate scannable luet tree (by builder)
}

type FileParser func(string, string, string, []string, types.PackageDatabase) error
