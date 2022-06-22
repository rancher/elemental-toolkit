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
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	containerdarchive "github.com/containerd/containerd/archive"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

// ExtractDeltaAdditionsFromImages is a filter that takes two images
// an includes and an excludes list. It computes the delta between the images
// considering the added files only, and applies a filter on them based on the regexes
// in the lists.
func ExtractDeltaAdditionsFiles(
	ctx types.Context,
	srcimg v1.Image,
	includes []string, excludes []string,
) (func(h *tar.Header) (bool, error), error) {

	includeRegexp := compileRegexes(includes)
	excludeRegexp := compileRegexes(excludes)

	srcfilesd, err := ctx.TempDir("srcfiles")
	if err != nil {
		return nil, err
	}
	filesSrc := NewCache(srcfilesd, 50*1024*1024, 10000)

	srcReader := mutate.Extract(srcimg)
	defer srcReader.Close()

	srcTar := tar.NewReader(srcReader)

	for {
		var hdr *tar.Header
		hdr, err := srcTar.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return nil, err
		}

		filesSrc.Set(hdr.Name, "")
	}

	return func(h *tar.Header) (bool, error) {

		fileName := filepath.Join(string(os.PathSeparator), h.Name)
		_, exists := filesSrc.Get(h.Name)
		if exists {
			return false, nil
		}

		switch {
		case len(includes) == 0 && len(excludes) != 0:
			for _, i := range excludeRegexp {
				if i.MatchString(filepath.Join(string(os.PathSeparator), h.Name)) &&
					fileName == filepath.Join(string(os.PathSeparator), h.Name) {
					return false, nil
				}
			}
			ctx.Debug("Adding name", fileName)

			return true, nil
		case len(includes) > 0 && len(excludes) == 0:
			for _, i := range includeRegexp {
				if i.MatchString(filepath.Join(string(os.PathSeparator), h.Name)) && fileName == filepath.Join(string(os.PathSeparator), h.Name) {
					ctx.Debug("Adding name", fileName)

					return true, nil
				}
			}
			return false, nil
		case len(includes) != 0 && len(excludes) != 0:
			for _, i := range includeRegexp {
				if i.MatchString(filepath.Join(string(os.PathSeparator), h.Name)) && fileName == filepath.Join(string(os.PathSeparator), h.Name) {
					for _, e := range excludeRegexp {
						if e.MatchString(fileName) {
							return false, nil
						}
					}
					ctx.Debug("Adding name", fileName)

					return true, nil
				}
			}

			return false, nil
		default:
			ctx.Debug("Adding name", fileName)
			return true, nil
		}

	}, nil
}

// ExtractFiles returns a filter that extracts files from the given path (if not empty)
// It then filters files by an include and exclude list.
// The list can be regexes
func ExtractFiles(
	ctx types.Context,
	prefixPath string,
	includes []string, excludes []string,
) func(h *tar.Header) (bool, error) {
	includeRegexp := compileRegexes(includes)
	excludeRegexp := compileRegexes(excludes)

	return func(h *tar.Header) (bool, error) {

		fileName := filepath.Join(string(os.PathSeparator), h.Name)
		switch {
		case len(includes) == 0 && len(excludes) != 0:
			for _, i := range excludeRegexp {
				if i.MatchString(filepath.Join(prefixPath, fileName)) {
					return false, nil
				}
			}
			if prefixPath != "" {
				return strings.HasPrefix(fileName, prefixPath), nil
			}
			ctx.Debug("Adding name", fileName)
			return true, nil

		case len(includes) > 0 && len(excludes) == 0:
			for _, i := range includeRegexp {
				if i.MatchString(filepath.Join(prefixPath, fileName)) {
					if prefixPath != "" {
						return strings.HasPrefix(fileName, prefixPath), nil
					}
					ctx.Debug("Adding name", fileName)

					return true, nil
				}
			}
			return false, nil
		case len(includes) != 0 && len(excludes) != 0:
			for _, i := range includeRegexp {
				if i.MatchString(filepath.Join(prefixPath, fileName)) {
					for _, e := range excludeRegexp {
						if e.MatchString(filepath.Join(prefixPath, fileName)) {
							return false, nil
						}
					}
					if prefixPath != "" {
						return strings.HasPrefix(fileName, prefixPath), nil
					}
					ctx.Debug("Adding name", fileName)

					return true, nil
				}
			}
			return false, nil
		default:
			if prefixPath != "" {
				return strings.HasPrefix(fileName, prefixPath), nil
			}

			return true, nil
		}
	}
}

// ExtractReader perform the extracting action over the io.ReadCloser
// it extracts the files over output. Accepts a filter as an option
// and additional containerd Options
func ExtractReader(ctx types.Context, reader io.ReadCloser, output string, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	defer reader.Close()

	// If no filter is specified, grab all.
	if filter == nil {
		filter = func(h *tar.Header) (bool, error) { return true, nil }
	}

	opts = append(opts, containerdarchive.WithFilter(filter))

	// Handle the extraction
	c, err := containerdarchive.Apply(context.Background(), output, reader, opts...)
	if err != nil {
		return 0, "", err
	}

	return c, output, nil
}

// Extract is just syntax sugar around ExtractReader. It extracts an image into a dir
func Extract(ctx types.Context, img v1.Image, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	tmpdiffs, err := ctx.TempDir("extraction")
	if err != nil {
		return 0, "", errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	return ExtractReader(ctx, mutate.Extract(img), tmpdiffs, filter, opts...)
}

// ExtractTo is just syntax sugar around ExtractReader
func ExtractTo(ctx types.Context, img v1.Image, output string, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	return ExtractReader(ctx, mutate.Extract(img), output, filter, opts...)
}
