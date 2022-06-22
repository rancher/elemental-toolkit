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
	"io"
	"os"

	containerdCompression "github.com/containerd/containerd/archive/compression"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
)

func imageFromTar(imagename, architecture, OS string, opener func() (io.ReadCloser, error)) (name.Reference, v1.Image, error) {
	newRef, err := name.ParseReference(imagename)
	if err != nil {
		return nil, nil, err
	}

	layer, err := tarball.LayerFromOpener(opener)
	if err != nil {
		return nil, nil, err
	}

	baseImage := empty.Image
	cfg, err := baseImage.ConfigFile()
	if err != nil {
		return nil, nil, err
	}

	cfg.Architecture = architecture
	cfg.OS = OS

	baseImage, err = mutate.ConfigFile(baseImage, cfg)
	if err != nil {
		return nil, nil, err
	}
	img, err := mutate.Append(baseImage, mutate.Addendum{
		Layer: layer,
		History: v1.History{
			CreatedBy: "luet",
			Comment:   "Custom image",
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return newRef, img, nil
}

// CreateTar a imagetarball from a standard tarball
func CreateTar(srctar, dstimageTar, imagename, architecture, OS string) error {

	dstFile, err := os.Create(dstimageTar)
	if err != nil {
		return errors.Wrap(err, "Cannot create "+dstimageTar)
	}
	defer dstFile.Close()

	newRef, img, err := imageFromTar(imagename, architecture, OS, func() (io.ReadCloser, error) {
		f, err := os.Open(srctar)
		if err != nil {
			return nil, errors.Wrap(err, "Cannot open "+srctar)
		}
		decompressed, err := containerdCompression.DecompressStream(f)
		if err != nil {
			return nil, errors.Wrap(err, "Cannot open "+srctar)
		}

		return decompressed, nil
	})
	if err != nil {
		return err
	}

	// NOTE: We might also stream that back to the daemon with daemon.Write(tag, img)
	return tarball.Write(newRef, img, dstFile)

}
