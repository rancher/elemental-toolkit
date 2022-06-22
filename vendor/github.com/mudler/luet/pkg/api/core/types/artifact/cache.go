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

package artifact

import (
	"crypto/sha512"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rancher-sandbox/gofilecache"
)

type ArtifactCache struct {
	gofilecache.Cache
}

func NewCache(dir string) *ArtifactCache {
	return &ArtifactCache{Cache: *gofilecache.InitCache(dir)}
}

func (c *ArtifactCache) cacheID(a *PackageArtifact) [64]byte {
	fingerprint := filepath.Base(a.Path)
	if a.CompileSpec != nil && a.CompileSpec.Package != nil {
		fingerprint = a.CompileSpec.Package.GetFingerPrint()
	}
	if len(a.Checksums) > 0 {
		for _, cs := range a.Checksums.List() {
			t := cs[0]
			result := cs[1]
			fingerprint += fmt.Sprintf("+%s:%s", t, result)
		}
	}
	return sha512.Sum512([]byte(fingerprint))
}

func (c *ArtifactCache) Get(a *PackageArtifact) (string, error) {
	fileName, _, err := c.Cache.GetFile(c.cacheID(a))
	return fileName, err
}

func (c *ArtifactCache) Put(a *PackageArtifact) (gofilecache.OutputID, int64, error) {
	file, err := os.Open(a.Path)
	if err != nil {
		return [64]byte{}, 0, errors.Wrapf(err, "failed opening %s", a.Path)
	}
	defer file.Close()
	return c.Cache.Put(c.cacheID(a), file)
}
