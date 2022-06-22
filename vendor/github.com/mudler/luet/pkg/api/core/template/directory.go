// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package template

import (
	"os"
	"path/filepath"
)

const Directory = "templates"

// FindPossibleTemplatesDir returns templates dir located at root
func FindPossibleTemplatesDir(root string) (res []string) {
	var ff = func(currentpath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == Directory {
			res = append(res, currentpath)
		}
		return nil
	}

	filepath.Walk(root, ff)
	return
}
