// Copyright © 2019-2021 Ettore Di Giacinto <mudler@gentoo.org>
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

package backend

import (
	"io"
	"os/exec"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	bus "github.com/mudler/luet/pkg/api/core/bus"
	"github.com/mudler/luet/pkg/api/core/image"
	"github.com/mudler/luet/pkg/api/core/types"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/pkg/errors"
)

type SimpleDocker struct {
	ctx types.Context
}

func NewSimpleDockerBackend(ctx types.Context) *SimpleDocker {
	return &SimpleDocker{ctx: ctx}
}

// TODO: Missing still: labels, and build args expansion
func (s *SimpleDocker) BuildImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePreBuild, opts)

	buildarg := genBuildCommand(opts)
	s.ctx.Info(":whale2: Building image " + name)
	cmd := exec.Command("docker", buildarg...)
	cmd.Dir = opts.SourcePath
	err := runCommand(s.ctx, cmd)
	if err != nil {
		return err
	}

	s.ctx.Success(":whale: Building image " + name + " done")

	bus.Manager.Publish(bus.EventImagePostBuild, opts)

	return nil
}

func (s *SimpleDocker) CopyImage(src, dst string) error {
	s.ctx.Debug(":whale: Tagging image:", src, "->", dst)
	cmd := exec.Command("docker", "tag", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed tagging image: "+string(out))
	}
	s.ctx.Success(":whale: Tagged image:", src, "->", dst)
	return nil
}

func (s *SimpleDocker) LoadImage(path string) error {
	s.ctx.Debug(":whale: Loading image:", path)
	cmd := exec.Command("docker", "load", "-i", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed loading image: "+string(out))
	}
	s.ctx.Success(":whale: Loaded image:", path)
	return nil
}

func (s *SimpleDocker) DownloadImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePull, opts)

	buildarg := []string{"pull", name}
	s.ctx.Debug(":whale: Downloading image " + name)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	cmd := exec.Command("docker", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pulling image: "+string(out))
	}

	s.ctx.Success(":whale: Downloaded image:", name)
	bus.Manager.Publish(bus.EventImagePostPull, opts)

	return nil
}

func (s *SimpleDocker) ImageExists(imagename string) bool {
	buildarg := []string{"inspect", "--type=image", imagename}
	s.ctx.Debug(":whale: Checking existance of docker image: " + imagename)
	cmd := exec.Command("docker", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.ctx.Debug("Image not present")
		s.ctx.Debug(string(out))
		return false
	}
	return true
}

func (*SimpleDocker) ImageAvailable(imagename string) bool {
	return image.Available(imagename)
}

func (s *SimpleDocker) RemoveImage(opts Options) error {
	name := opts.ImageName
	buildarg := []string{"rmi", name}
	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed removing image: "+string(out))
	}
	s.ctx.Success(":whale: Removed image:", name)
	//Info(string(out))
	return nil
}

func (s *SimpleDocker) Push(opts Options) error {
	name := opts.ImageName
	pusharg := []string{"push", name}
	bus.Manager.Publish(bus.EventImagePrePush, opts)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("docker", pusharg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pushing image: "+string(out))
	}
	s.ctx.Success(":whale: Pushed image:", name)
	bus.Manager.Publish(bus.EventImagePostPush, opts)

	//Info(string(out))
	return nil
}

func (s *SimpleDocker) imagefromDaemon(a string) (v1.Image, error) {
	ref, err := name.ParseReference(a)
	if err != nil {
		return nil, err
	}
	img, err := daemon.Image(ref, daemon.WithUnbufferedOpener())
	if err != nil {
		return nil, err
	}
	return img, nil
}

// TODO: Make it possible optionally to use this?
// It might be unsafer, as it relies on the pipe.
// imageFromCLIPipe returns a new image from a tarball by providing a reader from the docker stdout pipe.
// See also daemon.Image implementation below for an example (which returns the tarball stream
// from the HTTP api endpoint instead ).
func (s *SimpleDocker) imageFromCLIPipe(a string) (v1.Image, error) {
	return tarball.Image(func() (io.ReadCloser, error) {
		buildarg := []string{"save", a}
		s.ctx.Spinner()
		defer s.ctx.SpinnerStop()
		c := exec.Command("docker", buildarg...)
		p, err := c.StdoutPipe()
		if err != nil {
			return nil, err
		}
		err = c.Start()
		if err != nil {
			return nil, err
		}

		go func() { c.Wait() }()
		return p, nil
	}, nil)
}

func (s *SimpleDocker) imageFromDisk(a string) (v1.Image, error) {
	f, err := s.ctx.TempFile("snapshot")
	if err != nil {
		return nil, err
	}
	buildarg := []string{"save", a, "-o", f.Name()}
	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "Failed saving image: "+string(out))
	}

	return crane.Load(f.Name())
}

func (s *SimpleDocker) ImageReference(a string, ondisk bool) (v1.Image, error) {
	if ondisk {
		return s.imageFromDisk(a)
	}

	return s.imagefromDaemon(a)
}

func (s *SimpleDocker) ImageDefinitionToTar(opts Options) error {
	if err := s.BuildImage(opts); err != nil {
		return errors.Wrap(err, "Failed building image")
	}
	if err := s.ExportImage(opts); err != nil {
		return errors.Wrap(err, "Failed exporting image")
	}
	if err := s.RemoveImage(opts); err != nil {
		return errors.Wrap(err, "Failed removing image")
	}
	return nil
}

func (s *SimpleDocker) ExportImage(opts Options) error {
	name := opts.ImageName
	path := opts.Destination

	buildarg := []string{"save", name, "-o", path}
	s.ctx.Debug(":whale: Saving image " + name)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("docker", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}

	s.ctx.Debug(":whale: Exported image:", name)
	return nil
}

type ManifestEntry struct {
	Layers []string `json:"Layers"`
}
