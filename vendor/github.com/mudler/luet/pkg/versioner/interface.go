// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>,
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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

package version

// Versioner is responsible of sanitizing versions,
// validating them and ordering by precedence
type Versioner interface {
	Sanitize(string) string
	Validate(string) error
	Sort([]string) []string

	ValidateSelector(version string, selector string) bool
}
