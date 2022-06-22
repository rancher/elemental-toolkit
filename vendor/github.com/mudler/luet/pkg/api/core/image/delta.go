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

package image

import (
	"archive/tar"
	"io"
	"regexp"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

func compileRegexes(regexes []string) []*regexp.Regexp {
	var result []*regexp.Regexp
	for _, i := range regexes {
		r, e := regexp.Compile(i)
		if e != nil {
			continue
		}
		result = append(result, r)
	}
	return result
}

type ImageDiffNode struct {
	Name string `json:"Name"`
	Size int    `json:"Size"`
}
type ImageDiff struct {
	Additions []ImageDiffNode `json:"Adds"`
	Deletions []ImageDiffNode `json:"Dels"`
	Changes   []ImageDiffNode `json:"Mods"`
}

func Delta(srcimg, dstimg v1.Image) (res ImageDiff, err error) {
	srcReader := mutate.Extract(srcimg)
	defer srcReader.Close()

	dstReader := mutate.Extract(dstimg)
	defer dstReader.Close()

	filesSrc, filesDst := map[string]int64{}, map[string]int64{}

	srcTar := tar.NewReader(srcReader)
	dstTar := tar.NewReader(dstReader)

	for {
		var hdr *tar.Header
		hdr, err = srcTar.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return
		}
		filesSrc[hdr.Name] = hdr.Size
	}

	for {
		var hdr *tar.Header
		hdr, err = dstTar.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return
		}
		filesDst[hdr.Name] = hdr.Size
	}
	err = nil

	for f, size := range filesDst {
		if size2, exist := filesSrc[f]; exist && size2 != size {
			res.Changes = append(res.Changes, ImageDiffNode{
				Name: f,
				Size: int(size),
			})
		} else if !exist {
			res.Additions = append(res.Additions, ImageDiffNode{
				Name: f,
				Size: int(size),
			})
		}
	}
	for f, size := range filesSrc {
		if _, exist := filesDst[f]; !exist {
			res.Deletions = append(res.Deletions, ImageDiffNode{
				Name: f,
				Size: int(size),
			})
		}
	}

	return
}
