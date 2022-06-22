// Copyright © 2019-2020 Ettore Di Giacinto <mudler@gentoo.org>
//                       Daniele Rondina <geaaru@sabayonlinux.org>
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

package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ConfigProtectConfFile struct {
	Filename string

	Name        string   `mapstructure:"name" yaml:"name" json:"name"`
	Directories []string `mapstructure:"dirs" yaml:"dirs" json:"dirs"`
}

func NewConfigProtectConfFile(filename string) *ConfigProtectConfFile {
	return &ConfigProtectConfFile{
		Filename:    filename,
		Name:        "",
		Directories: []string{},
	}
}

func (c *ConfigProtectConfFile) String() string {
	return fmt.Sprintf("[%s] filename: %s, dirs: %s", c.Name, c.Filename,
		c.Directories)
}

type ConfigProtect struct {
	AnnotationDir string
	MapProtected  map[string]bool
}

func NewConfigProtect(annotationDir string) *ConfigProtect {
	if len(annotationDir) > 0 && annotationDir[0:1] != "/" {
		annotationDir = "/" + annotationDir
	}
	return &ConfigProtect{
		AnnotationDir: annotationDir,
		MapProtected:  make(map[string]bool),
	}
}

func (c *ConfigProtect) Map(files []string, protected []ConfigProtectConfFile) {

	for _, file := range files {

		if file[0:1] != "/" {
			file = "/" + file
		}

		if len(protected) > 0 {
			for _, conf := range protected {
				for _, dir := range conf.Directories {
					// Note file is without / at begin (on unpack)
					if strings.HasPrefix(file, filepath.Clean(dir)) {
						// docker archive modifier works with path without / at begin.
						c.MapProtected[file] = true
						goto nextFile
					}
				}
			}
		}

		if c.AnnotationDir != "" && strings.HasPrefix(file, filepath.Clean(c.AnnotationDir)) {
			c.MapProtected[file] = true
		}
	nextFile:
	}

}

func (c *ConfigProtect) Protected(file string) bool {
	if file[0:1] != "/" {
		file = "/" + file
	}
	_, ans := c.MapProtected[file]
	return ans
}

func (c *ConfigProtect) GetProtectFiles(withSlash bool) []string {
	ans := []string{}

	for key := range c.MapProtected {
		if withSlash {
			ans = append(ans, key)
		} else {
			ans = append(ans, key[1:])
		}
	}
	return ans
}
