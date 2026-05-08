/*
Copyright © 2022 - 2026 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"io/fs"
	"os"
)

type FS interface {
	Chmod(name string, mode fs.FileMode) error
	Create(name string) (*os.File, error)
	Glob(pattern string) ([]string, error)
	Link(oldname, newname string) error
	Lstat(name string) (fs.FileInfo, error)
	Mkdir(name string, perm fs.FileMode) error
	Open(name string) (fs.File, error)
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
	RawPath(name string) (string, error)
	ReadDir(dirname string) ([]fs.DirEntry, error)
	ReadFile(filename string) ([]byte, error)
	Readlink(name string) (string, error)
	Remove(name string) error
	RemoveAll(name string) error
	Rename(oldpath, newpath string) error
	Stat(name string) (fs.FileInfo, error)
	Symlink(oldname, newname string) error
	WriteFile(filename string, data []byte, perm fs.FileMode) error
}
