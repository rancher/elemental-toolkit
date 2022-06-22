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

package gc

import (
	"io/ioutil"
	"os"
)

type GarbageCollector string

func (c GarbageCollector) String() string {
	return string(c)
}

func (c GarbageCollector) init() error {
	if _, err := os.Stat(string(c)); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(string(c), os.ModePerm)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c GarbageCollector) Clean() error {
	return os.RemoveAll(string(c))
}

func (c GarbageCollector) TempDir(pattern string) (string, error) {
	err := c.init()
	if err != nil {
		return "", err
	}
	return ioutil.TempDir(string(c), pattern)
}

func (c GarbageCollector) TempFile(s string) (*os.File, error) {
	err := c.init()
	if err != nil {
		return nil, err
	}
	return ioutil.TempFile(string(c), s)
}
