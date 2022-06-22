// Copyright © 2019-2021 Ettore Di Giacinto <mudler@sabayon.org>
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

package compiler

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"regexp"
	"strings"
	"sync"
	"time"

	dockerfile "github.com/asottile/dockerfile"
	"github.com/imdario/mergo"
	bus "github.com/mudler/luet/pkg/api/core/bus"
	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/api/core/image"
	"github.com/mudler/luet/pkg/api/core/template"
	"github.com/mudler/luet/pkg/api/core/types"
	artifact "github.com/mudler/luet/pkg/api/core/types/artifact"
	"github.com/mudler/luet/pkg/compiler/backend"
	pkg "github.com/mudler/luet/pkg/database"
	"github.com/mudler/luet/pkg/helpers"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"github.com/mudler/luet/pkg/solver"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const BuildFile = "build.yaml"
const DefinitionFile = "definition.yaml"
const CollectionFile = "collection.yaml"

type ArtifactIndex []*artifact.PackageArtifact

func (i ArtifactIndex) CleanPath() ArtifactIndex {
	newIndex := ArtifactIndex{}
	for _, art := range i {
		copy := art.ShallowCopy()
		copy.Path = path.Base(art.Path)
		newIndex = append(newIndex, copy)
	}
	return newIndex
}

type LuetCompiler struct {
	//*tree.CompilerRecipe
	Backend  CompilerBackend
	Database types.PackageDatabase
	Options  types.CompilerOptions
}

func NewCompiler(p ...types.CompilerOption) *LuetCompiler {
	c := newDefaultCompiler()
	c.Apply(p...)

	return &LuetCompiler{Options: *c}
}

func NewLuetCompiler(backend CompilerBackend, db types.PackageDatabase, compilerOpts ...types.CompilerOption) *LuetCompiler {
	// The CompilerRecipe will gives us a tree with only build deps listed.

	c := NewCompiler(compilerOpts...)
	if c.Options.Context == nil {
		c.Options.Context = context.NewContext()
	}
	//	c.Options.BackendType
	c.Backend = backend
	c.Database = db

	return c
}

func (cs *LuetCompiler) compilerWorker(i int, wg *sync.WaitGroup, cspecs chan *types.LuetCompilationSpec, a *[]*artifact.PackageArtifact, m *sync.Mutex, concurrency int, keepPermissions bool, errors chan error) {
	defer wg.Done()

	for s := range cspecs {
		ar, err := cs.compile(concurrency, keepPermissions, nil, nil, s)
		if err != nil {
			errors <- err
		}

		m.Lock()
		*a = append(*a, ar)
		m.Unlock()
	}
}

// CompileWithReverseDeps compiles the supplied compilationspecs and their reverse dependencies
func (cs *LuetCompiler) CompileWithReverseDeps(keepPermissions bool, ps *types.LuetCompilationspecs) ([]*artifact.PackageArtifact, []error) {
	artifacts, err := cs.CompileParallel(keepPermissions, ps)
	if len(err) != 0 {
		return artifacts, err
	}

	cs.Options.Context.Info(":ant: Resolving reverse dependencies")
	toCompile := types.NewLuetCompilationspecs()
	for _, a := range artifacts {

		revdeps := a.CompileSpec.GetPackage().Revdeps(cs.Database)
		for _, r := range revdeps {
			spec, asserterr := cs.FromPackage(r)
			if err != nil {
				return nil, append(err, asserterr)
			}
			spec.SetOutputPath(ps.All()[0].GetOutputPath())

			toCompile.Add(spec)
		}
	}

	uniques := toCompile.Unique().Remove(ps)
	for _, u := range uniques.All() {
		cs.Options.Context.Info(" :arrow_right_hook:", u.GetPackage().GetName(), ":leaves:", u.GetPackage().GetVersion(), "(", u.GetPackage().GetCategory(), ")")
	}

	artifacts2, err := cs.CompileParallel(keepPermissions, uniques)
	return append(artifacts, artifacts2...), err
}

// CompileParallel compiles the supplied compilationspecs in parallel
// to note, no specific heuristic is implemented, and the specs are run in parallel as they are.
func (cs *LuetCompiler) CompileParallel(keepPermissions bool, ps *types.LuetCompilationspecs) ([]*artifact.PackageArtifact, []error) {
	all := make(chan *types.LuetCompilationSpec)
	artifacts := []*artifact.PackageArtifact{}
	mutex := &sync.Mutex{}
	errors := make(chan error, ps.Len())
	var wg = new(sync.WaitGroup)
	for i := 0; i < cs.Options.Concurrency; i++ {
		wg.Add(1)
		go cs.compilerWorker(i, wg, all, &artifacts, mutex, cs.Options.Concurrency, keepPermissions, errors)
	}

	for _, p := range ps.All() {
		all <- p
	}

	close(all)
	wg.Wait()
	close(errors)

	var allErrors []error

	for e := range errors {
		allErrors = append(allErrors, e)
	}

	return artifacts, allErrors
}

func (cs *LuetCompiler) stripFromRootfs(includes []string, rootfs string, include bool) error {
	var includeRegexp []*regexp.Regexp
	for _, i := range includes {
		r, e := regexp.Compile(i)
		if e != nil {
			return errors.Wrap(e, "Could not compile regex in the include of the package")
		}
		includeRegexp = append(includeRegexp, r)
	}

	toRemove := []string{}

	// the function that handles each file or dir
	var ff = func(currentpath string, info os.FileInfo, err error) error {

		// if info.Name() != DefinitionFile {
		// 	return nil // Skip with no errors
		// }
		if currentpath == rootfs {
			return nil
		}

		abspath := strings.ReplaceAll(currentpath, rootfs, "")

		match := false

		for _, i := range includeRegexp {
			if i.MatchString(abspath) {
				match = true
				break
			}
		}

		if include && !match || !include && match {
			toRemove = append(toRemove, currentpath)
			cs.Options.Context.Debug(":scissors: Removing file", currentpath)
		} else {
			cs.Options.Context.Debug(":sun: Matched file", currentpath)
		}

		return nil
	}

	err := filepath.Walk(rootfs, ff)
	if err != nil {
		return err
	}

	for _, s := range toRemove {
		e := os.RemoveAll(s)
		if e != nil {
			cs.Options.Context.Warning("Failed removing", s, e.Error())
			return e
		}
	}
	return nil
}

func (cs *LuetCompiler) unpackFs(concurrency int, keepPermissions bool, p *types.LuetCompilationSpec, runnerOpts backend.Options) (*artifact.PackageArtifact, error) {

	if !cs.Backend.ImageExists(runnerOpts.ImageName) {
		if err := cs.Backend.DownloadImage(runnerOpts); err != nil {
			return nil, errors.Wrap(err, "failed pulling image "+runnerOpts.ImageName+" during extraction")
		}
	}

	img, err := cs.Backend.ImageReference(runnerOpts.ImageName, true)
	if err != nil {
		return nil, err
	}

	ctx := cs.Options.Context.WithLoggingContext(fmt.Sprintf("extract %s", runnerOpts.ImageName))

	_, rootfs, err := image.Extract(
		ctx,
		img,
		image.ExtractFiles(
			cs.Options.Context,
			p.GetPackageDir(),
			p.GetIncludes(),
			p.GetExcludes(),
		),
	)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(rootfs) // clean up

	toUnpack := rootfs

	if p.PackageDir != "" {
		toUnpack = filepath.Join(toUnpack, p.PackageDir)
	}

	a := artifact.NewPackageArtifact(p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar"))
	a.CompressionType = cs.Options.CompressionType

	if err := a.Compress(toUnpack, concurrency); err != nil {
		return nil, errors.Wrap(err, "Error met while creating package archive")
	}

	a.CompileSpec = p
	return a, nil
}

func (cs *LuetCompiler) unpackDelta(concurrency int, keepPermissions bool, p *types.LuetCompilationSpec, builderOpts, runnerOpts backend.Options) (*artifact.PackageArtifact, error) {

	rootfs, err := cs.Options.Context.TempDir("rootfs")
	if err != nil {
		return nil, errors.Wrap(err, "Could not create tempdir")
	}
	defer os.RemoveAll(rootfs)

	pkgTag := ":package: " + p.GetPackage().HumanReadableString()
	if cs.Options.PullFirst {
		if !cs.Backend.ImageExists(builderOpts.ImageName) {
			err := cs.Backend.DownloadImage(builderOpts)
			if err != nil {
				return nil, errors.Wrap(err, "Could not pull image")
			}
		}
		if !cs.Backend.ImageExists(runnerOpts.ImageName) {
			err := cs.Backend.DownloadImage(runnerOpts)
			if err != nil {
				return nil, errors.Wrap(err, "Could not pull image")
			}
		}
	}

	cs.Options.Context.Info(pkgTag, ":hammer: Generating delta")

	cs.Options.Context.Debug(pkgTag, ":hammer: Retrieving reference for", builderOpts.ImageName)

	ref, err := cs.Backend.ImageReference(builderOpts.ImageName, true)
	if err != nil {
		return nil, err
	}

	cs.Options.Context.Debug(pkgTag, ":hammer: Retrieving reference for", runnerOpts.ImageName)

	ref2, err := cs.Backend.ImageReference(runnerOpts.ImageName, true)
	if err != nil {
		return nil, err
	}

	cs.Options.Context.Debug(pkgTag, ":hammer: Generating filters for extraction")

	filter, err := image.ExtractDeltaAdditionsFiles(cs.Options.Context, ref, p.GetIncludes(), p.GetExcludes())
	if err != nil {
		return nil, errors.Wrap(err, "failed generating filter for extraction")
	}

	cs.Options.Context.Info(pkgTag, ":hammer: Extracting artifact from image", runnerOpts.ImageName)
	a, err := artifact.ImageToArtifact(
		cs.Options.Context,
		ref2,
		cs.Options.CompressionType,
		p.Rel(fmt.Sprintf("%s%s", p.GetPackage().GetFingerPrint(), ".package.tar")),
		filter,
	)
	if err != nil {
		return nil, err
	}

	a.CompileSpec = p
	return a, nil
}

func (cs *LuetCompiler) buildPackageImage(image, buildertaggedImage, packageImage string,
	concurrency int, keepPermissions bool,
	p *types.LuetCompilationSpec) (backend.Options, backend.Options, error) {

	var runnerOpts, builderOpts backend.Options

	pkgTag := ":package: " + p.GetPackage().HumanReadableString()

	// TODO:  Cleanup, not actually hit
	if packageImage == "" {
		return runnerOpts, builderOpts, errors.New("no package image given")
	}

	p.SetSeedImage(image) // In this case, we ignore the build deps as we suppose that the image has them - otherwise we recompose the tree with a solver,
	// and we build all the images first.

	buildDir, err := cs.Options.Context.TempDir("build")
	if err != nil {
		return builderOpts, runnerOpts, err
	}
	defer os.RemoveAll(buildDir)

	// First we copy the source definitions into the output - we create a copy which the builds will need (we need to cache this phase somehow)
	err = fileHelper.CopyDir(p.GetPackage().GetPath(), buildDir)
	if err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Could not copy package sources")
	}

	// Copy file into the build context, the compilespec might have requested to do so.
	if len(p.GetRetrieve()) > 0 {
		err := p.CopyRetrieves(buildDir)
		if err != nil {
			cs.Options.Context.Warning("Failed copying retrieves", err.Error())
		}
	}

	// First we create the builder image
	if err := p.WriteBuildImageDefinition(filepath.Join(buildDir, p.GetPackage().ImageID()+"-builder.dockerfile")); err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Could not generate image definition")
	}

	// Even if we don't have prelude steps, we want to push
	// An intermediate image to tag images which are outside of the tree.
	// Those don't have an hash otherwise, and thus makes build unreproducible
	// see SKIPBUILD for the other logic
	// if len(p.GetPreBuildSteps()) == 0 {
	// 	buildertaggedImage = image
	// }
	// We might want to skip this phase but replacing with a tag that we push. But in case
	// steps in prelude are == 0 those are equivalent.

	// Then we write the step image, which uses the builder one
	if err := p.WriteStepImageDefinition(buildertaggedImage, filepath.Join(buildDir, p.GetPackage().ImageID()+".dockerfile")); err != nil {
		return builderOpts, runnerOpts, errors.Wrap(err, "Could not generate image definition")
	}

	builderOpts = backend.Options{
		ImageName:      buildertaggedImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().ImageID() + "-builder.dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + "-builder.image.tar"),
		BackendArgs:    cs.Options.BackendArgs,
	}
	runnerOpts = backend.Options{
		ImageName:      packageImage,
		SourcePath:     buildDir,
		DockerFileName: p.GetPackage().ImageID() + ".dockerfile",
		Destination:    p.Rel(p.GetPackage().GetFingerPrint() + ".image.tar"),
		BackendArgs:    cs.Options.BackendArgs,
	}

	buildAndPush := func(opts backend.Options) error {
		buildImage := true
		if cs.Options.PullFirst {
			err := cs.Backend.DownloadImage(opts)
			if err == nil {
				buildImage = false
			} else {
				cs.Options.Context.Warning("Failed to download '" + opts.ImageName + "'. Will keep going and build the image unless you use --fatal")
				cs.Options.Context.Warning(err.Error())
			}
		}
		if buildImage {
			if err := cs.Backend.BuildImage(opts); err != nil {
				return errors.Wrapf(err, "Could not build image: %s %s", image, opts.DockerFileName)
			}
			if cs.Options.Push {
				if err = cs.Backend.Push(opts); err != nil {
					return errors.Wrapf(err, "Could not push image: %s %s", image, opts.DockerFileName)
				}
			}
		}
		return nil
	}
	// SKIPBUILD
	//	if len(p.GetPreBuildSteps()) != 0 {
	cs.Options.Context.Info(pkgTag, ":whale: Generating 'builder' image from", image, "as", buildertaggedImage, "with prelude steps")
	if err := buildAndPush(builderOpts); err != nil {
		return builderOpts, runnerOpts, errors.Wrapf(err, "Could not push image: %s %s", image, builderOpts.DockerFileName)
	}
	//}

	// Even if we might not have any steps to build, we do that so we can tag the image used in this moment and use that to cache it in a registry, or in the system.
	// acting as a docker tag.
	cs.Options.Context.Info(pkgTag, ":whale: Generating 'package' image from", buildertaggedImage, "as", packageImage, "with build steps")
	if err := buildAndPush(runnerOpts); err != nil {
		return builderOpts, runnerOpts, errors.Wrapf(err, "Could not push image: %s %s", image, runnerOpts.DockerFileName)
	}

	return builderOpts, runnerOpts, nil
}

func (cs *LuetCompiler) genArtifact(p *types.LuetCompilationSpec, builderOpts, runnerOpts backend.Options, concurrency int, keepPermissions bool) (*artifact.PackageArtifact, error) {

	// generate *artifact.PackageArtifact
	var a *artifact.PackageArtifact
	var rootfs string
	var err error
	pkgTag := ":package: " + p.GetPackage().HumanReadableString()
	cs.Options.Context.Debug(pkgTag, "Generating artifact")
	// We can't generate delta in this case. It implies the package is a virtual, and nothing has to be done really
	if p.EmptyPackage() {
		fakePackage := p.Rel(p.GetPackage().GetFingerPrint() + ".package.tar")

		rootfs, err = cs.Options.Context.TempDir("rootfs")
		if err != nil {
			return nil, errors.Wrap(err, "Could not create tempdir")
		}
		defer os.RemoveAll(rootfs)

		a := artifact.NewPackageArtifact(fakePackage)
		a.CompressionType = cs.Options.CompressionType

		if err := a.Compress(rootfs, concurrency); err != nil {
			return nil, errors.Wrap(err, "Error met while creating package archive")
		}

		a.CompileSpec = p
		a.CompileSpec.GetPackage().SetBuildTimestamp(time.Now().String())
		err = a.WriteYAML(p.GetOutputPath())
		if err != nil {
			return a, errors.Wrap(err, "Failed while writing metadata file")
		}
		cs.Options.Context.Success(pkgTag, "   :white_check_mark: done (empty virtual package)")
		if err := cs.finalizeImages(a, p, keepPermissions); err != nil {
			return nil, err
		}

		return a, nil
	}

	if p.UnpackedPackage() {
		// Take content of container as a base for our package files
		a, err = cs.unpackFs(concurrency, keepPermissions, p, runnerOpts)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while extracting image")
		}
	} else {
		// Generate delta between the two images
		a, err = cs.unpackDelta(concurrency, keepPermissions, p, builderOpts, runnerOpts)
		if err != nil {
			return nil, errors.Wrap(err, "Error met while generating delta")
		}
	}

	if !p.Package.Hidden {
		filelist, err := a.FileList()
		if err != nil {
			return a, errors.Wrapf(err, "Failed getting package list for '%s' '%s'", a.Path, a.CompileSpec.Package.HumanReadableString())
		}
		a.Files = filelist
	}

	a.CompileSpec.GetPackage().SetBuildTimestamp(time.Now().String())

	err = a.WriteYAML(p.GetOutputPath())
	if err != nil {
		return a, errors.Wrap(err, "Failed while writing metadata file")
	}
	cs.Options.Context.Success(pkgTag, "   :white_check_mark: Done building")

	if err := cs.finalizeImages(a, p, keepPermissions); err != nil {
		return nil, err
	}

	// Write sub packages
	if len(a.CompileSpec.SubPackages) > 0 {
		cs.Options.Context.Success(pkgTag, "   :gear: Creating sub packages")
		for _, sub := range a.CompileSpec.SubPackages {
			if err := cs.buildSubPackage(a, sub, p, keepPermissions, concurrency); err != nil {
				return nil, err
			}
		}
	}

	return a, nil
}

func (cs *LuetCompiler) buildSubPackage(a *artifact.PackageArtifact, sub *types.SubPackage, spec *types.LuetCompilationSpec, keepPermissions bool, concurrency int) error {
	sub.SetPath(spec.Package.Path)

	cs.Options.Context.Info(":arrow_right: Creating sub package", sub.HumanReadableString())
	subArtifactDir, err := cs.Options.Context.TempDir("subpackage")
	if err != nil {
		return errors.Wrap(err, "could not create tempdir for final artifact")
	}
	defer os.RemoveAll(subArtifactDir)

	err = a.Unpack(cs.Options.Context, subArtifactDir, keepPermissions, image.ExtractFiles(cs.Options.Context, "", sub.Includes, sub.Excludes))
	if err != nil {
		return errors.Wrap(err, "while unpack sub package")
	}

	subP := spec.Rel(sub.GetFingerPrint() + ".package.tar")

	subArtifact := artifact.NewPackageArtifact(subP)
	subArtifact.CompressionType = cs.Options.CompressionType

	if err := subArtifact.Compress(subArtifactDir, concurrency); err != nil {
		return errors.Wrap(err, "Error met while creating package archive")
	}

	subArtifact.CompileSpec = spec
	subArtifact.CompileSpec.Package = sub.Package
	subArtifact.Runtime = sub.Package
	subArtifact.CompileSpec.GetPackage().SetBuildTimestamp(time.Now().String())

	err = subArtifact.WriteYAML(spec.GetOutputPath(), artifact.WithRuntimePackage(sub.Package))
	if err != nil {
		return errors.Wrap(err, "Failed while writing metadata file")
	}
	cs.Options.Context.Success("   :white_check_mark: done (subpackage)", sub.HumanReadableString())

	if err := cs.finalizeImages(subArtifact, spec, keepPermissions); err != nil {
		return errors.Wrap(err, "Failed while writing finalizing images")
	}

	return nil
}

// finalizeImages finalizes images and generates final artifacts (push them as well if necessary).
func (cs *LuetCompiler) finalizeImages(a *artifact.PackageArtifact, p *types.LuetCompilationSpec, keepPermissions bool) error {

	// TODO: This is a small readaptation of repository_docker.go pushImageFromArtifact().
	//       Maybe can be moved to a common place.

	// We either check if finalization is needed
	// and push or generate final images here, anything else we just return successfully
	if !cs.Options.PushFinalImages && !cs.Options.GenerateFinalImages {
		return nil
	}

	imageID := fmt.Sprintf("%s:%s", cs.Options.PushFinalImagesRepository, a.CompileSpec.Package.ImageID())
	metadataImageID := fmt.Sprintf("%s:%s", cs.Options.PushFinalImagesRepository, helpers.SanitizeImageString(a.CompileSpec.GetPackage().GetMetadataFilePath()))

	// Do generate image only, might be required for local iteration without pushing to remote repository
	if cs.Options.GenerateFinalImages && !cs.Options.PushFinalImages {
		cs.Options.Context.Info("Generating final image for", a.CompileSpec.Package.HumanReadableString())

		if err := a.GenerateFinalImage(cs.Options.Context, imageID, cs.GetBackend(), true); err != nil {
			return errors.Wrap(err, "while creating final image")
		}

		a := artifact.NewPackageArtifact(filepath.Join(p.GetOutputPath(), a.CompileSpec.GetPackage().GetMetadataFilePath()))
		metadataArchive, err := artifact.CreateArtifactForFile(cs.Options.Context, a.Path)
		if err != nil {
			return errors.Wrap(err, "failed generating checksums for tree")
		}
		if err := metadataArchive.GenerateFinalImage(cs.Options.Context, metadataImageID, cs.Backend, keepPermissions); err != nil {
			return errors.Wrap(err, "Failed generating metadata tree "+metadataImageID)
		}

		return nil
	}

	cs.Options.Context.Info("Pushing final image for", a.CompileSpec.Package.HumanReadableString())

	// First push the package image
	if !cs.Backend.ImageAvailable(imageID) || cs.Options.PushFinalImagesForce {
		cs.Options.Context.Info("Generating and pushing final image for", a.CompileSpec.Package.HumanReadableString(), "as", imageID)

		if err := a.GenerateFinalImage(cs.Options.Context, imageID, cs.GetBackend(), true); err != nil {
			return errors.Wrap(err, "while creating final image")
		}
		if err := cs.Backend.Push(backend.Options{ImageName: imageID}); err != nil {
			return errors.Wrapf(err, "Could not push image: %s", imageID)
		}
	}

	// Then the image ID
	if !cs.Backend.ImageAvailable(metadataImageID) || cs.Options.PushFinalImagesForce {
		cs.Options.Context.Info("Generating metadata image for", a.CompileSpec.Package.HumanReadableString(), metadataImageID)

		a := artifact.NewPackageArtifact(filepath.Join(p.GetOutputPath(), a.CompileSpec.GetPackage().GetMetadataFilePath()))
		metadataArchive, err := artifact.CreateArtifactForFile(cs.Options.Context, a.Path)
		if err != nil {
			return errors.Wrap(err, "failed generating checksums for tree")
		}
		if err := metadataArchive.GenerateFinalImage(cs.Options.Context, metadataImageID, cs.Backend, keepPermissions); err != nil {
			return errors.Wrap(err, "Failed generating metadata tree "+metadataImageID)
		}
		if err = cs.Backend.Push(backend.Options{ImageName: metadataImageID}); err != nil {
			return errors.Wrapf(err, "Could not push image: %s", metadataImageID)
		}
	}
	return nil
}

func (cs *LuetCompiler) waitForImages(images []string) {
	if cs.Options.PullFirst && cs.Options.Wait {
		available, _ := oneOfImagesAvailable(images, cs.Backend)
		if !available {
			cs.Options.Context.Info(fmt.Sprintf("Waiting for image %s to be available... :zzz:", images))
			cs.Options.Context.Spinner()
			defer cs.Options.Context.SpinnerStop()
			for !available {
				available, _ = oneOfImagesAvailable(images, cs.Backend)
				cs.Options.Context.Info(fmt.Sprintf("Image %s not available yet, sleeping", images))
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func oneOfImagesExists(images []string, b CompilerBackend) (bool, string) {
	for _, i := range images {
		if exists := b.ImageExists(i); exists {
			return true, i
		}
	}
	return false, ""
}
func oneOfImagesAvailable(images []string, b CompilerBackend) (bool, string) {
	for _, i := range images {
		if exists := b.ImageAvailable(i); exists {
			return true, i
		}
	}
	return false, ""
}

func (cs *LuetCompiler) findImageHash(imageHash string, p *types.LuetCompilationSpec) string {
	var resolvedImage string
	cs.Options.Context.Debug("Resolving image hash for", p.Package.HumanReadableString(), "hash", imageHash, "Pull repositories", p.BuildOptions.PullImageRepository)
	toChecklist := append([]string{fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, imageHash)},
		genImageList(p.BuildOptions.PullImageRepository, imageHash)...)

	if cs.Options.PushFinalImagesRepository != "" {
		toChecklist = append(toChecklist, fmt.Sprintf("%s:%s", cs.Options.PushFinalImagesRepository, imageHash))
	}
	if exists, which := oneOfImagesExists(toChecklist, cs.Backend); exists {
		resolvedImage = which
	}
	if cs.Options.PullFirst {
		if exists, which := oneOfImagesAvailable(toChecklist, cs.Backend); exists {
			resolvedImage = which
		}
	}
	return resolvedImage
}

func (cs *LuetCompiler) resolveExistingImageHash(imageHash string, p *types.LuetCompilationSpec) string {
	resolvedImage := cs.findImageHash(imageHash, p)

	if resolvedImage == "" {
		resolvedImage = fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, imageHash)
	}
	return resolvedImage
}

func LoadArtifactFromYaml(spec *types.LuetCompilationSpec) (*artifact.PackageArtifact, error) {
	metaFile := spec.GetPackage().GetMetadataFilePath()
	dat, err := ioutil.ReadFile(spec.Rel(metaFile))
	if err != nil {
		return nil, errors.Wrap(err, "Error reading file "+metaFile)
	}
	art, err := artifact.NewPackageArtifactFromYaml(dat)
	if err != nil {
		return nil, errors.Wrap(err, "Error writing file "+metaFile)
	}
	// It is relative, set it back to abs
	art.Path = spec.Rel(art.Path)
	return art, nil
}

func (cs *LuetCompiler) getImageArtifact(hash string, p *types.LuetCompilationSpec) (*artifact.PackageArtifact, error) {
	// we check if there is an available image with the given hash and
	// we return a full artifact if can be loaded locally.
	cs.Options.Context.Debug("Get image artifact for", p.Package.HumanReadableString(), "hash", hash, "Pull repositories", p.BuildOptions.PullImageRepository)

	toChecklist := append([]string{fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, hash)},
		genImageList(p.BuildOptions.PullImageRepository, hash)...)

	exists, _ := oneOfImagesExists(toChecklist, cs.Backend)
	if art, err := LoadArtifactFromYaml(p); err == nil && exists { // If YAML is correctly loaded, and both images exists, no reason to rebuild.
		cs.Options.Context.Debug("Package reloaded from YAML. Skipping build")
		return art, nil
	}
	cs.waitForImages(toChecklist)
	available, _ := oneOfImagesAvailable(toChecklist, cs.Backend)
	if exists || (cs.Options.PullFirst && available) {
		cs.Options.Context.Debug("Image available, returning empty artifact")
		return &artifact.PackageArtifact{}, nil
	}

	return nil, errors.New("artifact not found")
}

// compileWithImage compiles a PackageTagHash image using the image source, and tagging an indermediate
// image buildertaggedImage.
// Images that can be resolved from repositories are prefered over the local ones if PullFirst is set to true
// avoiding to rebuild images as much as possible
func (cs *LuetCompiler) compileWithImage(image, builderHash string, packageTagHash string,
	concurrency int,
	keepPermissions, keepImg bool,
	p *types.LuetCompilationSpec, generateArtifact bool) (*artifact.PackageArtifact, error) {

	// If it is a virtual, check if we have to generate an empty artifact or not.
	if generateArtifact && p.IsVirtual() {
		return cs.genArtifact(p, backend.Options{}, backend.Options{}, concurrency, keepPermissions)
	} else if p.IsVirtual() {
		return &artifact.PackageArtifact{}, nil
	}

	if !generateArtifact {
		if art, err := cs.getImageArtifact(packageTagHash, p); err == nil {
			// try to avoid regenerating the image if possible by checking the hash in the
			// given repositories
			// It is best effort. If we fail resolving, we will generate the images and keep going
			return art, nil
		}
	}

	packageImage := fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, packageTagHash)
	remoteBuildertaggedImage := fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, builderHash)
	builderResolved := cs.resolveExistingImageHash(builderHash, p)
	//generated := false
	// if buildertaggedImage == "" {
	// 	buildertaggedImage = fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, buildertaggedImage)
	// 	generated = true
	// 	//	cs.Options.Context.Debug(pkgTag, "Creating intermediary image", buildertaggedImage, "from", image)
	// }

	if cs.Options.PullFirst && !cs.Options.Rebuild {
		cs.Options.Context.Debug("Checking if an image is already available")
		// FIXUP here. If packageimage hash exists and pull is true, generate package
		resolved := cs.resolveExistingImageHash(packageTagHash, p)
		cs.Options.Context.Debug("Resolved: " + resolved)
		cs.Options.Context.Debug("Expected remote: " + resolved)
		cs.Options.Context.Debug("Package image: " + packageImage)
		cs.Options.Context.Debug("Resolved builder image: " + builderResolved)

		// a remote image is there already
		remoteImageAvailable := resolved != packageImage && remoteBuildertaggedImage != builderResolved
		// or a local one is available
		localImageAvailable := cs.Backend.ImageExists(remoteBuildertaggedImage) && cs.Backend.ImageExists(packageImage)

		switch {
		case remoteImageAvailable:
			cs.Options.Context.Debug("Images available remotely for", p.Package.HumanReadableString(), "generating artifact from remote images:", resolved)
			return cs.genArtifact(p, backend.Options{ImageName: builderResolved}, backend.Options{ImageName: resolved}, concurrency, keepPermissions)
		case localImageAvailable:
			cs.Options.Context.Debug("Images locally available for", p.Package.HumanReadableString(), "generating artifact from image:", resolved)
			return cs.genArtifact(p, backend.Options{ImageName: remoteBuildertaggedImage}, backend.Options{ImageName: packageImage}, concurrency, keepPermissions)
		default:
			cs.Options.Context.Debug("Images not available for", p.Package.HumanReadableString())
		}
	}

	// always going to point at the destination from the repo defined
	builderOpts, runnerOpts, err := cs.buildPackageImage(image, builderResolved, packageImage, concurrency, keepPermissions, p)
	if err != nil {
		return nil, errors.Wrap(err, "failed building package image")
	}

	if !keepImg {
		defer func() {
			// We keep them around, so to not reload them from the tar (which should be the "correct way") and we automatically share the same layers
			if err := cs.Backend.RemoveImage(builderOpts); err != nil {
				cs.Options.Context.Warning("Could not remove image ", builderOpts.ImageName)
			}
			if err := cs.Backend.RemoveImage(runnerOpts); err != nil {
				cs.Options.Context.Warning("Could not remove image ", runnerOpts.ImageName)
			}
		}()
	}

	if !generateArtifact {
		return &artifact.PackageArtifact{}, nil
	}

	return cs.genArtifact(p, builderOpts, runnerOpts, concurrency, keepPermissions)
}

// FromDatabase returns all the available compilation specs from a database. If the minimum flag is returned
// it will be computed a minimal subset that will guarantees that all packages are built ( if not targeting a single package explictly )
func (cs *LuetCompiler) FromDatabase(db types.PackageDatabase, minimum bool, dst string) ([]*types.LuetCompilationSpec, error) {
	compilerSpecs := types.NewLuetCompilationspecs()

	w := db.World()

	for _, p := range w {
		spec, err := cs.FromPackage(p)
		if err != nil {
			return nil, err
		}
		if dst != "" {
			spec.SetOutputPath(dst)
		}
		compilerSpecs.Add(spec)
	}

	switch minimum {
	case true:
		return cs.ComputeMinimumCompilableSet(compilerSpecs.Unique().All()...)
	default:
		return compilerSpecs.Unique().All(), nil
	}
}

func (cs *LuetCompiler) ComputeDepTree(p *types.LuetCompilationSpec, db types.PackageDatabase) (types.PackagesAssertions, error) {
	s := solver.NewResolver(cs.Options.SolverOptions.SolverOptions, pkg.NewInMemoryDatabase(false), db, pkg.NewInMemoryDatabase(false), solver.NewSolverFromOptions(cs.Options.SolverOptions))

	solution, err := s.Install(types.Packages{p.GetPackage()})
	if err != nil {
		return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().HumanReadableString())
	}

	dependencies, err := solution.Order(db, p.GetPackage().GetFingerPrint())
	if err != nil {
		return nil, errors.Wrap(err, "While order a solution for "+p.GetPackage().HumanReadableString())
	}
	return dependencies, nil
}

// BuildTree returns a BuildTree which represent the order in which specs should be compiled.
// It places specs into levels, and each level can be built in parallel. The root nodes starting from the top.
// A BuildTree can be marshaled into JSON or walked like:
// for _, l := range bt.AllLevels() {
//	fmt.Println(strings.Join(bt.AllInLevel(l), " "))
// }
func (cs *LuetCompiler) BuildTree(compilerSpecs types.LuetCompilationspecs) (*BuildTree, error) {
	compilationTree := map[string]map[string]interface{}{}
	bt := &BuildTree{}

	for _, sp := range compilerSpecs.All() {
		ass, err := cs.ComputeDepTree(sp, cs.Database)
		if err != nil {
			return nil, err
		}
		bt.Reset(fmt.Sprintf("%s/%s", sp.GetPackage().GetCategory(), sp.GetPackage().GetName()))
		for _, p := range ass {
			bt.Reset(fmt.Sprintf("%s/%s", p.Package.GetCategory(), p.Package.GetName()))

			spec, err := cs.FromPackage(p.Package)
			if err != nil {
				return nil, err
			}
			ass, err := cs.ComputeDepTree(spec, cs.Database)
			if err != nil {
				return nil, err
			}
			for _, r := range ass {
				if compilationTree[fmt.Sprintf("%s/%s", p.Package.GetCategory(), p.Package.GetName())] == nil {
					compilationTree[fmt.Sprintf("%s/%s", p.Package.GetCategory(), p.Package.GetName())] = make(map[string]interface{})
				}
				compilationTree[fmt.Sprintf("%s/%s", p.Package.GetCategory(), p.Package.GetName())][fmt.Sprintf("%s/%s", r.Package.GetCategory(), r.Package.GetName())] = nil
			}
			if compilationTree[fmt.Sprintf("%s/%s", sp.GetPackage().GetCategory(), sp.GetPackage().GetName())] == nil {
				compilationTree[fmt.Sprintf("%s/%s", sp.GetPackage().GetCategory(), sp.GetPackage().GetName())] = make(map[string]interface{})
			}
			compilationTree[fmt.Sprintf("%s/%s", sp.GetPackage().GetCategory(), sp.GetPackage().GetName())][fmt.Sprintf("%s/%s", p.Package.GetCategory(), p.Package.GetName())] = nil
		}
	}

	bt.Order(compilationTree)

	return bt, nil
}

// ComputeMinimumCompilableSet strips specs that are eventually compiled by leafs
func (cs *LuetCompiler) ComputeMinimumCompilableSet(p ...*types.LuetCompilationSpec) ([]*types.LuetCompilationSpec, error) {
	// Generate a set with all the deps of the provided specs
	// we will use that set to remove the deps from the list of provided compilation specs
	allDependencies := types.PackagesAssertions{} // Get all packages that will be in deps
	result := []*types.LuetCompilationSpec{}
	for _, spec := range p {
		sol, err := cs.ComputeDepTree(spec, cs.Database)
		if err != nil {
			return nil, errors.Wrap(err, "failed querying hashtree")
		}
		allDependencies = append(allDependencies, sol.Drop(spec.GetPackage())...)
	}

	for _, spec := range p {
		if found := allDependencies.Search(spec.GetPackage().GetFingerPrint()); found == nil {
			result = append(result, spec)
		}
	}
	return result, nil
}

// Compile is a non-parallel version of CompileParallel. It builds the compilation specs and generates
// an artifact
func (cs *LuetCompiler) Compile(keepPermissions bool, p *types.LuetCompilationSpec) (*artifact.PackageArtifact, error) {
	return cs.compile(cs.Options.Concurrency, keepPermissions, nil, nil, p)
}

func genImageList(refs []string, hash string) []string {
	var res []string
	for _, r := range refs {
		res = append(res, fmt.Sprintf("%s:%s", r, hash))
	}
	return res
}

func (cs *LuetCompiler) inheritSpecBuildOptions(p *types.LuetCompilationSpec) {
	cs.Options.Context.Debug(p.GetPackage().HumanReadableString(), "Build options before inherit", p.BuildOptions)

	// Append push repositories from buildpsec buildoptions as pull if found.
	// This allows to resolve the hash automatically if we pulled the metadata from
	// repositories that are advertizing their cache.
	if len(p.BuildOptions.PushImageRepository) != 0 {
		p.BuildOptions.PullImageRepository = append(p.BuildOptions.PullImageRepository, p.BuildOptions.PushImageRepository)
		cs.Options.Context.Debug("Inheriting pull repository from PushImageRepository buildoptions", p.BuildOptions.PullImageRepository)
	}

	if len(cs.Options.PullImageRepository) != 0 {
		p.BuildOptions.PullImageRepository = append(p.BuildOptions.PullImageRepository, cs.Options.PullImageRepository...)
		cs.Options.Context.Debug("Inheriting pull repository from PullImageRepository buildoptions", p.BuildOptions.PullImageRepository)
	}

	cs.Options.Context.Debug(p.GetPackage().HumanReadableString(), "Build options after inherit", p.BuildOptions)
}

func (cs *LuetCompiler) getSpecHash(pkgs types.Packages, salt string) (string, error) {
	ht := NewHashTree(cs.Database)
	overallFp := ""
	for _, p := range pkgs {
		compileSpec, err := cs.FromPackage(p)
		if err != nil {
			return "", errors.Wrap(err, "Error while generating compilespec for "+p.GetName())
		}
		packageHashTree, err := ht.Query(cs, compileSpec)
		if err != nil {
			return "nil", errors.Wrap(err, "failed querying hashtree")
		}
		overallFp = overallFp + packageHashTree.Target.Hash.PackageHash + p.GetFingerPrint()
	}

	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s-%s", overallFp, salt))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (cs *LuetCompiler) resolveFinalImages(concurrency int, keepPermissions bool, p *types.LuetCompilationSpec) error {
	if !p.RequiresFinalImages {
		return nil
	}

	joinTag := ">:loop: final images<"

	var fromPackages types.Packages

	cs.Options.Context.Info(joinTag, "Generating a parent image from final packages")

	if cs.Options.RuntimeDatabase != nil {
		// Create a fake db from runtime which has the target entry as the compiler view
		db := pkg.NewInMemoryDatabase(false)
		cs.Options.RuntimeDatabase.Clone(db)
		defer db.Clean()

		if err := db.UpdatePackage(p.Package); err != nil {
			return err
		}

		// resolve deptree of runtime of p and use it in fromPackages
		t, err := cs.ComputeDepTree(p, db)
		if err != nil {
			return errors.Wrap(err, "failed querying hashtree")
		}

		for _, a := range t {
			if !a.Value || a.Package.Matches(p.Package) {
				continue
			}

			fromPackages = append(fromPackages, a.Package)
			cs.Options.Context.Infof("Adding dependency '%s'.", a.Package.HumanReadableString())
		}
	} else {
		cs.Options.Context.Info(joinTag, "No runtime db present, first level join only")
		fromPackages = p.Package.GetRequires() // first level only
	}

	// First compute a hash and check if image is available. if it is, then directly consume that
	overallFp, err := cs.getSpecHash(fromPackages, "join")
	if err != nil {
		return errors.Wrap(err, "could not generate image hash")
	}

	cs.Options.Context.Info(joinTag, "Searching existing image with hash", overallFp)

	if img := cs.findImageHash(overallFp, p); img != "" {
		cs.Options.Context.Info("Image already found", img)
		p.SetImage(img)
		return nil
	}
	cs.Options.Context.Info(joinTag, "Image not found. Generating image join with hash ", overallFp)

	// Make sure there is an output path
	if err := os.MkdirAll(p.GetOutputPath(), os.ModePerm); err != nil {
		return errors.Wrap(err, "while creating output path")
	}

	// otherwise, generate it and push it aside
	joinDir, err := cs.Options.Context.TempDir("join")
	if err != nil {
		return errors.Wrap(err, "could not create tempdir for joining images")
	}
	defer os.RemoveAll(joinDir)

	for _, p := range fromPackages {
		cs.Options.Context.Info(joinTag, ":arrow_right_hook:", p.HumanReadableString(), ":leaves:")
	}

	current := 0
	for _, c := range fromPackages {
		current++
		if c != nil && c.Name != "" && c.Version != "" {
			joinTag2 := fmt.Sprintf("%s %d/%d ⤑ :hammer: build %s", joinTag, current, len(fromPackages), c.HumanReadableString())

			// Search if we have already a final-image that was already pushed
			// for this to work on the same repo, it is required to push final images during build
			if img := cs.findImageHash(c.ImageID(), p); cs.Options.PullFirst && img != "" {
				cs.Options.Context.Info("Final image already found", img)
				if !cs.Backend.ImageExists(img) {
					if err := cs.Backend.DownloadImage(backend.Options{ImageName: img}); err != nil {
						return errors.Wrap(err, "failed pulling image "+img+" during extraction")
					}
				}

				imgRef, err := cs.Backend.ImageReference(img, true)
				if err != nil {
					return err
				}

				ctx := cs.Options.Context.WithLoggingContext(fmt.Sprintf("final image extract %s", img))
				_, _, err = image.ExtractTo(
					ctx,
					imgRef,
					joinDir,
					nil,
				)
				if err != nil {
					return err
				}
			} else {
				cs.Options.Context.Info("Final image not found for", c.HumanReadableString())

				// If no image was found, we have to build it from scratch
				cs.Options.Context.Info(joinTag2, "compilation starts")
				spec, err := cs.FromPackage(c)
				if err != nil {
					return errors.Wrap(err, "while generating images to join from")
				}
				wantsArtifact := true
				genDepsArtifact := !cs.Options.PackageTargetOnly

				spec.SetOutputPath(p.GetOutputPath())

				artifact, err := cs.compile(concurrency, keepPermissions, &wantsArtifact, &genDepsArtifact, spec)
				if err != nil {
					return errors.Wrap(err, "failed building join image")
				}

				err = artifact.Unpack(cs.Options.Context, joinDir, keepPermissions)
				if err != nil {
					return errors.Wrap(err, "failed building join image")
				}
				cs.Options.Context.Info(joinTag2, ":white_check_mark: Done")
			}
		}
	}

	artifactDir, err := cs.Options.Context.TempDir("join")
	if err != nil {
		return errors.Wrap(err, "could not create tempdir for final artifact")
	}
	defer os.RemoveAll(artifactDir)

	cs.Options.Context.Info(joinTag, ":droplet: generating artifact for source image of", p.GetPackage().HumanReadableString())

	// After unpack, create a new artifact and a new final image from it.
	// no need to compress, as we are going to toss it away.
	a := artifact.NewPackageArtifact(filepath.Join(artifactDir, p.GetPackage().GetFingerPrint()+".join.tar"))
	if err := a.Compress(joinDir, concurrency); err != nil {
		return errors.Wrap(err, "error met while creating package archive")
	}

	joinImageName := fmt.Sprintf("%s:%s", cs.Options.PushImageRepository, overallFp)
	cs.Options.Context.Info(joinTag, ":droplet: generating image from artifact", joinImageName)
	err = a.GenerateFinalImage(cs.Options.Context, joinImageName, cs.Backend, keepPermissions)
	if err != nil {
		return errors.Wrap(err, "could not create final image")
	}
	if cs.Options.Push {
		cs.Options.Context.Info(joinTag, ":droplet: pushing image from artifact", joinImageName)
		if err = cs.Backend.Push(backend.Options{ImageName: joinImageName}); err != nil {
			return errors.Wrapf(err, "Could not push image: %s", joinImageName)
		}
	}
	cs.Options.Context.Info(joinTag, ":droplet: Consuming image", joinImageName)
	p.SetImage(joinImageName)
	return nil
}

func (cs *LuetCompiler) resolveMultiStageImages(concurrency int, keepPermissions bool, p *types.LuetCompilationSpec) error {
	resolvedCopyFields := []types.CopyField{}
	copyTag := ">:droplet: copy<"

	if len(p.Copy) != 0 {
		cs.Options.Context.Info(copyTag, "Package has multi-stage copy, generating required images")
	}

	current := 0
	// TODO: we should run this only if we are going to build the image
	for _, c := range p.Copy {
		current++
		if c.Package != nil && c.Package.Name != "" && c.Package.Version != "" {
			copyTag2 := fmt.Sprintf("%s %d/%d ⤑ :hammer: build %s", copyTag, current, len(p.Copy), c.Package.HumanReadableString())

			cs.Options.Context.Info(copyTag2, "generating multi-stage images for", c.Package.HumanReadableString())
			spec, err := cs.FromPackage(c.Package)
			if err != nil {
				return errors.Wrap(err, "while generating images to copy from")
			}

			// If we specify --only-target package, we don't want any artifact, otherwise we do
			genArtifact := !cs.Options.PackageTargetOnly
			spec.SetOutputPath(p.GetOutputPath())
			artifact, err := cs.compile(concurrency, keepPermissions, &genArtifact, &genArtifact, spec)
			if err != nil {
				return errors.Wrap(err, "failed building multi-stage image")
			}

			resolvedCopyFields = append(resolvedCopyFields, types.CopyField{
				Image:       cs.resolveExistingImageHash(artifact.PackageCacheImage, spec),
				Source:      c.Source,
				Destination: c.Destination,
			})
			cs.Options.Context.Success(copyTag2, ":white_check_mark: Done")
		} else {
			resolvedCopyFields = append(resolvedCopyFields, c)
		}
	}
	p.Copy = resolvedCopyFields
	return nil
}

func CompilerFinalImages(cs *LuetCompiler) (*LuetCompiler, error) {
	// When computing the hash tree, we need to take into consideration
	// that packages that require final images have to be seen as packages without deps
	// This is because we don't really want to calculate the deptree of them as
	// as it is handled already when we are creating the images in resolveFinalImages().
	c := *cs
	copy := &c
	memDB := pkg.NewInMemoryDatabase(false)
	// Create a copy to avoid races
	dbCopy := pkg.NewInMemoryDatabase(false)
	err := cs.Database.Clone(dbCopy)
	if err != nil {
		return nil, errors.Wrap(err, "failed cloning db")
	}
	for _, p := range dbCopy.World() {
		copy := p.Clone()
		spec, err := cs.FromPackage(p)
		if err != nil {
			return nil, errors.Wrap(err, "failed getting compile spec for package "+p.HumanReadableString())
		}
		if spec.RequiresFinalImages {
			copy.Requires([]*types.Package{})
		}

		memDB.CreatePackage(copy)
	}
	copy.Database = memDB
	return copy, nil
}

func (cs *LuetCompiler) compile(concurrency int, keepPermissions bool, generateFinalArtifact *bool, generateDependenciesFinalArtifact *bool, p *types.LuetCompilationSpec) (*artifact.PackageArtifact, error) {
	cs.Options.Context.Info(":package: Compiling", p.GetPackage().HumanReadableString(), ".... :coffee:")

	//Before multistage : join - same as multistage, but keep artifacts, join them, create a new one and generate a final image.
	// When the image is there, use it as a source here, in place of GetImage().
	if err := cs.resolveFinalImages(concurrency, keepPermissions, p); err != nil {
		return nil, errors.Wrap(err, "while resolving join images")
	}

	if err := cs.resolveMultiStageImages(concurrency, keepPermissions, p); err != nil {
		return nil, errors.Wrap(err, "while resolving multi-stage images")
	}

	cs.Options.Context.Debug(fmt.Sprintf("%s: has images %t, empty package: %t", p.GetPackage().HumanReadableString(), p.HasImageSource(), p.EmptyPackage()))
	if !p.HasImageSource() && !p.EmptyPackage() {
		return nil,
			fmt.Errorf(
				"%s is invalid: package has no dependencies and no seed image supplied while it has steps defined",
				p.GetPackage().GetFingerPrint(),
			)
	}

	ht := NewHashTree(cs.Database)
	copy, err := CompilerFinalImages(cs)
	if err != nil {
		return nil, err
	}
	packageHashTree, err := ht.Query(copy, p)
	if err != nil {
		return nil, errors.Wrap(err, "failed querying hashtree")
	}

	// This is in order to have the metadata in the yaml
	p.SetSourceAssertion(packageHashTree.Solution)
	targetAssertion := packageHashTree.Target

	bus.Manager.Publish(bus.EventPackagePreBuild, struct {
		CompileSpec     *types.LuetCompilationSpec
		Assert          types.PackageAssert
		PackageHashTree *PackageImageHashTree
	}{
		CompileSpec:     p,
		Assert:          *targetAssertion,
		PackageHashTree: packageHashTree,
	})

	// Update compilespec build options - it will be then serialized into the compilation metadata file
	p.BuildOptions.PushImageRepository = cs.Options.PushImageRepository

	// - If image is set we just generate a plain dockerfile
	// Treat last case (easier) first. The image is provided and we just compute a plain dockerfile with the images listed as above
	if p.GetImage() != "" {
		localGenerateArtifact := true
		if generateFinalArtifact != nil {
			localGenerateArtifact = *generateFinalArtifact
		}

		a, err := cs.compileWithImage(p.GetImage(), packageHashTree.BuilderImageHash, targetAssertion.Hash.PackageHash, concurrency, keepPermissions, cs.Options.KeepImg, p, localGenerateArtifact)
		if err != nil {
			return nil, errors.Wrap(err, "building direct image")
		}
		a.SourceAssertion = p.GetSourceAssertion()

		a.PackageCacheImage = targetAssertion.Hash.PackageHash
		return a, nil
	}

	// - If image is not set, we read a base_image. Then we will build one image from it to kick-off our build based
	// on how we compute the resolvable tree.
	// This means to recursively build all the build-images needed to reach that tree part.
	// - We later on compute an hash used to identify the image, so each similar deptree keeps the same build image.
	dependencies := packageHashTree.Dependencies  // at this point we should have a flattened list of deps to build, including all of them (with all constraints propagated already)
	departifacts := []*artifact.PackageArtifact{} // TODO: Return this somehow
	depsN := 0
	currentN := 0

	packageDeps := !cs.Options.PackageTargetOnly
	if generateDependenciesFinalArtifact != nil {
		packageDeps = *generateDependenciesFinalArtifact
	}

	buildDeps := !cs.Options.NoDeps
	buildTarget := !cs.Options.OnlyDeps

	if buildDeps {

		cs.Options.Context.Info(":deciduous_tree: Build dependencies for " + p.GetPackage().HumanReadableString())
		for _, assertion := range dependencies { //highly dependent on the order
			depsN++
			cs.Options.Context.Info(" :arrow_right_hook:", assertion.Package.HumanReadableString(), ":leaves:")
		}

		for _, assertion := range dependencies { //highly dependent on the order
			currentN++
			pkgTag := fmt.Sprintf(":package: %d/%d %s ⤑ :hammer: build %s", currentN, depsN, p.GetPackage().HumanReadableString(), assertion.Package.HumanReadableString())
			cs.Options.Context.Info(pkgTag, " starts")
			compileSpec, err := cs.FromPackage(assertion.Package)
			if err != nil {
				return nil, errors.Wrap(err, "Error while generating compilespec for "+assertion.Package.GetName())
			}
			compileSpec.BuildOptions.PullImageRepository = append(compileSpec.BuildOptions.PullImageRepository, p.BuildOptions.PullImageRepository...)

			cs.Options.Context.Debug("PullImage repos:", compileSpec.BuildOptions.PullImageRepository)

			compileSpec.SetOutputPath(p.GetOutputPath())

			bus.Manager.Publish(bus.EventPackagePreBuild, struct {
				CompileSpec *types.LuetCompilationSpec
				Assert      types.PackageAssert
			}{
				CompileSpec: compileSpec,
				Assert:      assertion,
			})

			if err := cs.resolveFinalImages(concurrency, keepPermissions, compileSpec); err != nil {
				return nil, errors.Wrap(err, "while resolving join images")
			}

			if err := cs.resolveMultiStageImages(concurrency, keepPermissions, compileSpec); err != nil {
				return nil, errors.Wrap(err, "while resolving multi-stage images")
			}

			buildHash, err := packageHashTree.DependencyBuildImage(assertion.Package)
			if err != nil {
				return nil, errors.Wrap(err, "failed looking for dependency in hashtree")
			}

			cs.Options.Context.Debug(pkgTag, "    :arrow_right_hook: :whale: Builder image from hash", assertion.Hash.BuildHash)
			cs.Options.Context.Debug(pkgTag, "    :arrow_right_hook: :whale: Package image from hash", assertion.Hash.PackageHash)

			var sourceImage string

			if compileSpec.GetImage() != "" {
				cs.Options.Context.Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().HumanReadableString()+" from image")
				sourceImage = compileSpec.GetImage()
			} else {
				// for the source instead, pick an image and a buildertaggedImage from hashes if they exists.
				// otherways fallback to the pushed repo
				// Resolve images from the hashtree
				sourceImage = cs.resolveExistingImageHash(assertion.Hash.BuildHash, compileSpec)
				cs.Options.Context.Debug(pkgTag, " :wrench: Compiling "+compileSpec.GetPackage().HumanReadableString()+" from tree")
			}

			a, err := cs.compileWithImage(
				sourceImage,
				buildHash,
				assertion.Hash.PackageHash,
				concurrency,
				keepPermissions,
				cs.Options.KeepImg,
				compileSpec,
				packageDeps,
			)
			if err != nil {
				return nil, errors.Wrap(err, "Failed compiling "+compileSpec.GetPackage().HumanReadableString())
			}

			a.PackageCacheImage = assertion.Hash.PackageHash

			cs.Options.Context.Success(pkgTag, ":white_check_mark: Done")

			bus.Manager.Publish(bus.EventPackagePostBuild, struct {
				CompileSpec *types.LuetCompilationSpec
				Artifact    *artifact.PackageArtifact
			}{
				CompileSpec: compileSpec,
				Artifact:    a,
			})

			departifacts = append(departifacts, a)
		}
	}

	if buildTarget {
		localGenerateArtifact := true
		if generateFinalArtifact != nil {
			localGenerateArtifact = *generateFinalArtifact
		}
		resolvedSourceImage := cs.resolveExistingImageHash(packageHashTree.SourceHash, p)
		cs.Options.Context.Info(":rocket: All dependencies are satisfied, building package requested by the user", p.GetPackage().HumanReadableString())
		cs.Options.Context.Info(":package:", p.GetPackage().HumanReadableString(), " Using image: ", resolvedSourceImage)
		a, err := cs.compileWithImage(resolvedSourceImage, packageHashTree.BuilderImageHash, targetAssertion.Hash.PackageHash, concurrency, keepPermissions, cs.Options.KeepImg, p, localGenerateArtifact)
		if err != nil {
			return a, err
		}
		a.Dependencies = departifacts
		a.SourceAssertion = p.GetSourceAssertion()
		a.PackageCacheImage = targetAssertion.Hash.PackageHash
		bus.Manager.Publish(bus.EventPackagePostBuild, struct {
			CompileSpec *types.LuetCompilationSpec
			Artifact    *artifact.PackageArtifact
		}{
			CompileSpec: p,
			Artifact:    a,
		})

		return a, err
	} else {
		return departifacts[len(departifacts)-1], nil
	}
}

type templatedata map[string]interface{}

func (cs *LuetCompiler) templatePackage(vals []map[string]interface{}, pack *types.Package, dst templatedata) ([]byte, error) {
	// Grab shared templates first
	var chartFiles []string
	if len(cs.Options.TemplatesFolder) != 0 {
		c, err := template.FilesInDir(cs.Options.TemplatesFolder)
		if err == nil {
			chartFiles = c
		}
	}

	var dataresult []byte
	val := pack.Rel(DefinitionFile)

	if _, err := os.Stat(pack.Rel(CollectionFile)); err == nil {
		val = pack.Rel(CollectionFile)

		data, err := ioutil.ReadFile(val)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+val)
		}

		dataBuild, err := ioutil.ReadFile(pack.Rel(BuildFile))
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+val)
		}

		packsRaw, err := types.GetRawPackages(data)
		if err != nil {
			return nil, errors.Wrap(err, "getting raw packages")
		}

		raw := packsRaw.Find(*pack)
		td := templatedata{}
		if len(vals) > 0 {
			for _, bv := range vals {
				current := templatedata(bv)
				if err := mergo.Merge(&td, current); err != nil {
					return nil, errors.Wrap(err, "merging values maps")
				}
			}
		}

		if err := mergo.Merge(&td, templatedata(raw)); err != nil {
			return nil, errors.Wrap(err, "merging values maps")
		}

		dat, err := template.Render(append(template.ReadFiles(chartFiles...), string(dataBuild)), td, dst)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+pack.Rel(BuildFile))
		}
		dataresult = []byte(dat)
	} else {
		bv := cs.Options.BuildValuesFile
		if len(vals) > 0 {
			valuesdir, err := cs.Options.Context.TempDir("genvalues")
			if err != nil {
				return nil, errors.Wrap(err, "Could not create tempdir")
			}
			defer os.RemoveAll(valuesdir)

			for _, b := range vals {
				out, err := yaml.Marshal(b)
				if err != nil {
					return nil, errors.Wrap(err, "while marshalling values file")
				}
				f := filepath.Join(valuesdir, fileHelper.RandStringRunes(20))
				if err := ioutil.WriteFile(f, out, os.ModePerm); err != nil {
					return nil, errors.Wrap(err, "while writing temporary values file")
				}
				bv = append([]string{f}, bv...)
			}
		}

		out, err := template.RenderWithValues(append(chartFiles, pack.Rel(BuildFile)), val, bv...)
		if err != nil {
			return nil, errors.Wrap(err, "rendering file "+pack.Rel(BuildFile))
		}
		dataresult = []byte(out)
	}

	return dataresult, nil
}

// FromPackage returns a compilation spec from a package definition
func (cs *LuetCompiler) FromPackage(p *types.Package) (*types.LuetCompilationSpec, error) {
	// This would be nice to move it out from the compiler, but it is strictly tight to it given the build options
	pack, err := cs.Database.FindPackageCandidate(p)
	if err != nil {
		return nil, err
	}

	opts := types.CompilerOptions{}

	artifactMetadataFile := filepath.Join(pack.GetTreeDir(), "..", pack.GetMetadataFilePath())
	cs.Options.Context.Debug("Checking if metadata file is present", artifactMetadataFile)
	if _, err := os.Stat(artifactMetadataFile); err == nil {
		f, err := os.Open(artifactMetadataFile)
		if err != nil {
			return nil, errors.Wrapf(err, "could not open %s", artifactMetadataFile)
		}
		dat, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
		art, err := artifact.NewPackageArtifactFromYaml(dat)
		if err != nil {
			return nil, errors.Wrap(err, "could not decode package from yaml")
		}

		cs.Options.Context.Debug("Read build options:", art.CompileSpec.BuildOptions, "from", artifactMetadataFile)
		if art.CompileSpec.BuildOptions != nil {
			opts = *art.CompileSpec.BuildOptions
		}
	} else if !os.IsNotExist(err) {
		cs.Options.Context.Debug("error reading artifact metadata file: ", err.Error())
	} else if os.IsNotExist(err) {
		cs.Options.Context.Debug("metadata file not present, skipping", artifactMetadataFile)
	}

	// If the input is a dockerfile, just consume it and parse any image source from it
	if pack.OriginDockerfile != "" {
		img := ""
		// TODO: Carry this info and parse Dockerfile from somewhere else?
		cmds, err := dockerfile.ParseReader(bytes.NewBufferString(pack.OriginDockerfile))
		if err != nil {
			return nil, errors.Wrap(err, "could not decode Dockerfile")
		}
		for _, c := range cmds {
			if c.Cmd == "FROM" &&
				len(c.Value) > 0 && !strings.Contains(strings.ToLower(fmt.Sprint(c.Value)), "as") {
				img = c.Value[0]
			}
		}

		compilationSpec := &types.LuetCompilationSpec{
			Image:        img,
			Package:      pack,
			BuildOptions: &types.CompilerOptions{},
		}
		cs.inheritSpecBuildOptions(compilationSpec)

		return compilationSpec, nil
	}

	// Update processed build values
	dst, err := template.UnMarshalValues(cs.Options.BuildValuesFile)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling values")
	}
	opts.BuildValues = append(opts.BuildValues, (map[string]interface{})(dst))

	bytes, err := cs.templatePackage(opts.BuildValues, pack, templatedata(dst))
	if err != nil {
		return nil, errors.Wrap(err, "while rendering package template")
	}

	newSpec, err := types.NewLuetCompilationSpec(bytes, pack)
	if err != nil {
		return nil, err
	}
	newSpec.BuildOptions = &opts

	cs.inheritSpecBuildOptions(newSpec)

	// Update the package in the compiler database to catch updates from NewLuetCompilationSpec
	if err := cs.Database.UpdatePackage(newSpec.Package); err != nil {
		return nil, errors.Wrap(err, "failed updating new package entry in compiler database")
	}

	return newSpec, err
}

// GetBackend returns the current compilation backend
func (cs *LuetCompiler) GetBackend() CompilerBackend {
	return cs.Backend
}

// SetBackend sets the compilation backend
func (cs *LuetCompiler) SetBackend(b CompilerBackend) {
	cs.Backend = b
}
