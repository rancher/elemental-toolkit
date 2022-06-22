// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package types

import "os"

// GarbageCollector is a general garbage collector for temporary files
// It is responsible of assigning temporary files and cleaning up afterwards
type GarbageCollector interface {
	Clean() error
	TempDir(pattern string) (string, error)
	TempFile(s string) (*os.File, error)
	String() string
}
