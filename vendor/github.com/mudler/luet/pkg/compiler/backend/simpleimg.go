// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
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
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	bus "github.com/mudler/luet/pkg/api/core/bus"
	"github.com/mudler/luet/pkg/api/core/image"
	"github.com/mudler/luet/pkg/api/core/types"

	"github.com/pkg/errors"
)

type SimpleImg struct {
	ctx types.Context
}

func NewSimpleImgBackend(ctx types.Context) *SimpleImg {
	return &SimpleImg{ctx: ctx}
}

func (s *SimpleImg) LoadImage(string) error {
	return errors.New("Not supported")
}

// TODO: Missing still: labels, and build args expansion
func (s *SimpleImg) BuildImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePreBuild, opts)

	buildarg := genBuildCommand(opts)

	s.ctx.Info(":tea: Building image " + name)

	cmd := exec.Command("img", buildarg...)
	cmd.Dir = opts.SourcePath
	err := runCommand(s.ctx, cmd)
	if err != nil {
		return err
	}
	bus.Manager.Publish(bus.EventImagePostBuild, opts)

	s.ctx.Info(":tea: Building image " + name + " done")

	return nil
}

func (s *SimpleImg) RemoveImage(opts Options) error {
	name := opts.ImageName
	buildarg := []string{"rm", name}
	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()
	out, err := exec.Command("img", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed removing image: "+string(out))
	}

	s.ctx.Info(":tea: Image " + name + " removed")
	return nil
}

func (s *SimpleImg) ImageReference(a string, ondisk bool) (v1.Image, error) {

	f, err := s.ctx.TempFile("snapshot")
	if err != nil {
		return nil, err
	}
	buildarg := []string{"save", a, "-o", f.Name()}
	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("img", buildarg...).CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "Failed saving image: "+string(out))
	}

	img, err := crane.Load(f.Name())
	if err != nil {
		return nil, err
	}

	return img, nil
}

func (s *SimpleImg) DownloadImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePull, opts)

	buildarg := []string{"pull", name}
	s.ctx.Debug(":tea: Downloading image " + name)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	cmd := exec.Command("img", buildarg...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed downloading image: "+string(out))
	}

	s.ctx.Info(":tea: Image " + name + " downloaded")
	bus.Manager.Publish(bus.EventImagePostPull, opts)

	return nil
}
func (s *SimpleImg) CopyImage(src, dst string) error {
	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	s.ctx.Debug(":tea: Tagging image", src, dst)
	cmd := exec.Command("img", "tag", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed tagging image: "+string(out))
	}
	s.ctx.Info(":tea: Image " + dst + " tagged")

	return nil
}

func (s *SimpleImg) ImageAvailable(imagename string) bool {
	return image.Available(imagename)
}

// ImageExists check if the given image is available locally
func (*SimpleImg) ImageExists(imagename string) bool {
	cmd := exec.Command("img", "ls")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	if strings.Contains(string(out), imagename) {
		return true
	}
	return false
}

func (s *SimpleImg) ImageDefinitionToTar(opts Options) error {
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

func (s *SimpleImg) ExportImage(opts Options) error {
	name := opts.ImageName
	path := opts.Destination
	buildarg := []string{"save", "-o", path, name}
	s.ctx.Debug(":tea: Saving image " + name)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("img", buildarg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}
	s.ctx.Info(":tea: Image " + name + " saved")
	return nil
}

func (s *SimpleImg) Push(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePush, opts)

	pusharg := []string{"push", name}
	out, err := exec.Command("img", pusharg...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pushing image: "+string(out))
	}
	s.ctx.Info(":tea: Pushed image:", name)
	bus.Manager.Publish(bus.EventImagePostPush, opts)

	//s.ctx.Info(string(out))
	return nil
}
