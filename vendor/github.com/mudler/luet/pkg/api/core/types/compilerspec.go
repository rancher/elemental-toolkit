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

package types

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/mitchellh/hashstructure/v2"

	"github.com/ghodss/yaml"
	"github.com/otiai10/copy"
	dirhash "golang.org/x/mod/sumdb/dirhash"
)

type LuetCompilationspecs []LuetCompilationSpec

func NewLuetCompilationspecs(s ...*LuetCompilationSpec) *LuetCompilationspecs {
	all := LuetCompilationspecs{}

	for _, spec := range s {
		all.Add(spec)
	}
	return &all
}

func (specs LuetCompilationspecs) Len() int {
	return len(specs)
}

func (specs *LuetCompilationspecs) Remove(s *LuetCompilationspecs) *LuetCompilationspecs {
	newSpecs := LuetCompilationspecs{}
SPECS:
	for _, spec := range specs.All() {
		for _, target := range s.All() {
			if target.GetPackage().Matches(spec.GetPackage()) {
				continue SPECS
			}
		}
		newSpecs.Add(spec)
	}
	return &newSpecs
}

func (specs *LuetCompilationspecs) Add(s *LuetCompilationSpec) {
	*specs = append(*specs, *s)
}

func (specs *LuetCompilationspecs) All() []*LuetCompilationSpec {
	var cspecs []*LuetCompilationSpec
	for i := range *specs {
		f := (*specs)[i]
		cspecs = append(cspecs, &f)
	}

	return cspecs
}

func (specs *LuetCompilationspecs) Unique() *LuetCompilationspecs {
	newSpecs := LuetCompilationspecs{}
	seen := map[string]bool{}

	for i := range *specs {
		j := (*specs)[i]
		_, ok := seen[j.GetPackage().GetFingerPrint()]
		if !ok {
			seen[j.GetPackage().GetFingerPrint()] = true
			newSpecs = append(newSpecs, j)
		}
	}
	return &newSpecs
}

type CopyField struct {
	Package     *Package `json:"package"`
	Image       string   `json:"image"`
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
}

type CompressionImplementation string

const (
	None      CompressionImplementation = "none" // e.g. tar for standard packages
	GZip      CompressionImplementation = "gzip"
	Zstandard CompressionImplementation = "zstd"
)

type SubPackage struct {
	*Package
	Includes []string `json:"includes,omitempty" yaml:"includes,omitempty"`
	Excludes []string `json:"excludes,omitempty" yaml:"excludes,omitempty"`
}

type LuetCompilationSpec struct {
	Steps           []string           `json:"steps" yaml:"steps,omitempty"` // Are run inside a container and the result layer diff is saved
	Env             []string           `json:"env"`
	Prelude         []string           `json:"prelude"` // Are run inside the image which will be our builder
	Image           string             `json:"image"`
	Seed            string             `json:"seed"`
	Package         *Package           `json:"package"`
	SourceAssertion PackagesAssertions `json:"-"`
	PackageDir      string             `json:"package_dir" yaml:"package_dir"`

	SubPackages []*SubPackage `json:"subpackages,omitempty" yaml:"subpackages,omitempty"`

	Retrieve []string `json:"retrieve"`

	OutputPath string   `json:"-"` // Where the build processfiles go
	Unpack     bool     `json:"unpack"`
	Includes   []string `json:"includes"`
	Excludes   []string `json:"excludes"`

	BuildOptions *CompilerOptions `json:"build_options"`

	Copy []CopyField `json:"copy"`

	RequiresFinalImages bool `json:"requires_final_images" yaml:"requires_final_images"`
}

// Signature is a portion of the spec that yields a signature for the hash
type Signature struct {
	Image               string
	Steps               []string
	PackageDir          string
	Prelude             []string
	Seed                string
	Env                 []string
	Retrieve            []string
	Unpack              bool
	Includes            []string
	Excludes            []string
	Copy                []CopyField
	Requires            Packages
	RequiresFinalImages bool
	Dockerfile          string
}

type CompilerOptions struct {
	PushImageRepository      string
	PullImageRepository      []string
	PullFirst, KeepImg, Push bool
	Concurrency              int
	CompressionType          CompressionImplementation

	Wait            bool
	OnlyDeps        bool
	NoDeps          bool
	SolverOptions   LuetSolverOptions
	BuildValuesFile []string
	BuildValues     []map[string]interface{}

	PackageTargetOnly bool
	Rebuild           bool

	BackendArgs []string

	BackendType string

	// TemplatesFolder. should default to tree/templates
	TemplatesFolder []string

	// Tells wether to push final container images after building
	PushFinalImages      bool
	PushFinalImagesForce bool

	GenerateFinalImages bool

	// Image repository to push to
	PushFinalImagesRepository string
	RuntimeDatabase           PackageDatabase

	Context Context
}

type CompilerOption func(cfg *CompilerOptions) error

func (cfg *CompilerOptions) Apply(opts ...CompilerOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return err
		}
	}
	return nil
}

func (cs *LuetCompilationSpec) signature() Signature {
	return Signature{
		Image:               cs.Image,
		Steps:               cs.Steps,
		PackageDir:          cs.PackageDir,
		Prelude:             cs.Prelude,
		Seed:                cs.Seed,
		Env:                 cs.Env,
		Retrieve:            cs.Retrieve,
		Unpack:              cs.Unpack,
		Includes:            cs.Includes,
		Excludes:            cs.Excludes,
		Copy:                cs.Copy,
		Requires:            cs.Package.GetRequires(),
		Dockerfile:          cs.Package.OriginDockerfile,
		RequiresFinalImages: cs.RequiresFinalImages,
	}
}

func NewLuetCompilationSpec(b []byte, p *Package) (*LuetCompilationSpec, error) {
	var spec LuetCompilationSpec
	var packageDefinition Package
	err := yaml.Unmarshal(b, &spec)
	if err != nil {
		return &spec, err
	}
	err = yaml.Unmarshal(b, &packageDefinition)
	if err != nil {
		return &spec, err
	}

	// Update requires/conflict/provides
	// When we have been passed a bytes slice, parse it as a package
	// and updates requires/conflicts/provides.
	// This is required in order to allow manipulation of such fields with templating
	copy := *p
	spec.Package = &copy
	if len(packageDefinition.GetRequires()) != 0 {
		spec.Package.Requires(packageDefinition.GetRequires())
	}
	if len(packageDefinition.GetConflicts()) != 0 {
		spec.Package.Conflicts(packageDefinition.GetConflicts())
	}
	if len(packageDefinition.GetProvides()) != 0 {
		spec.Package.SetProvides(packageDefinition.GetProvides())
	}
	return &spec, nil
}
func (cs *LuetCompilationSpec) GetSourceAssertion() PackagesAssertions {
	return cs.SourceAssertion
}

func (cs *LuetCompilationSpec) SetBuildOptions(b CompilerOptions) {
	cs.BuildOptions = &b
}

func (cs *LuetCompilationSpec) SetSourceAssertion(as PackagesAssertions) {
	cs.SourceAssertion = as
}
func (cs *LuetCompilationSpec) GetPackage() *Package {
	return cs.Package
}

func (cs *LuetCompilationSpec) GetPackageDir() string {
	return cs.PackageDir
}

func (cs *LuetCompilationSpec) SetPackageDir(s string) {
	cs.PackageDir = s
}

func (cs *LuetCompilationSpec) BuildSteps() []string {
	return cs.Steps
}

func (cs *LuetCompilationSpec) ImageUnpack() bool {
	return cs.Unpack
}

func (cs *LuetCompilationSpec) GetPreBuildSteps() []string {
	return cs.Prelude
}

func (cs *LuetCompilationSpec) GetIncludes() []string {
	return cs.Includes
}

func (cs *LuetCompilationSpec) GetExcludes() []string {
	return cs.Excludes
}

func (cs *LuetCompilationSpec) GetRetrieve() []string {
	return cs.Retrieve
}

// IsVirtual returns true if the spec is virtual.
// A spec is virtual if the package is empty, and it has no image source to unpack from.
func (cs *LuetCompilationSpec) IsVirtual() bool {
	return cs.EmptyPackage() && !cs.HasImageSource()
}

func (cs *LuetCompilationSpec) GetSeedImage() string {
	return cs.Seed
}

func (cs *LuetCompilationSpec) GetImage() string {
	return cs.Image
}

func (cs *LuetCompilationSpec) GetOutputPath() string {
	return cs.OutputPath
}

func (p *LuetCompilationSpec) Rel(s string) string {
	return filepath.Join(p.GetOutputPath(), s)
}

func (cs *LuetCompilationSpec) SetImage(s string) {
	cs.Image = s
}

func (cs *LuetCompilationSpec) SetOutputPath(s string) {
	cs.OutputPath = s
}

func (cs *LuetCompilationSpec) SetSeedImage(s string) {
	cs.Seed = s
}

func (cs *LuetCompilationSpec) EmptyPackage() bool {
	return len(cs.BuildSteps()) == 0 && !cs.UnpackedPackage() && (cs.Package != nil && cs.Package.OriginDockerfile == "" || cs.Package == nil)
}

func (cs *LuetCompilationSpec) UnpackedPackage() bool {
	// If package_dir was specified in the spec, we want to treat the content of the directory
	// as the root of our archive.  ImageUnpack is implied to be true. override it
	unpack := cs.ImageUnpack()
	if cs.GetPackageDir() != "" {
		unpack = true
	}
	return unpack
}

// HasImageSource returns true when the compilation spec has an image source.
// a compilation spec has an image source when it depends on other packages or have a source image
// explictly supplied
func (cs *LuetCompilationSpec) HasImageSource() bool {
	return (cs.Package != nil && len(cs.GetPackage().GetRequires()) != 0) || cs.GetImage() != "" || (cs.RequiresFinalImages && len(cs.Package.GetRequires()) != 0)
}

func (cs *LuetCompilationSpec) Hash() (string, error) {
	// build a signature, we want to be part of the hash only the fields that are relevant for build purposes
	signature := cs.signature()
	h, err := hashstructure.Hash(signature, hashstructure.FormatV2, nil)
	if err != nil {
		return "", err
	}
	sum, err := dirhash.HashDir(cs.Package.Path, "", dirhash.DefaultHash)
	if err != nil {
		return fmt.Sprint(h), err
	}
	return fmt.Sprint(h, sum), err
}

func (cs *LuetCompilationSpec) CopyRetrieves(dest string) error {
	var err error
	if len(cs.Retrieve) > 0 {
		for _, s := range cs.Retrieve {
			matches, err := filepath.Glob(cs.Rel(s))

			if err != nil {
				continue
			}

			for _, m := range matches {
				err = copy.Copy(m, filepath.Join(dest, filepath.Base(m)))
			}
		}
	}
	return err
}

func (cs *LuetCompilationSpec) genDockerfile(image string, steps []string) string {
	spec := `
FROM ` + image + `
COPY . /luetbuild
WORKDIR /luetbuild
ENV PACKAGE_NAME=` + cs.Package.GetName() + `
ENV PACKAGE_VERSION=` + cs.Package.GetVersion() + `
ENV PACKAGE_CATEGORY=` + cs.Package.GetCategory()

	if len(cs.Retrieve) > 0 {
		for _, s := range cs.Retrieve {
			//var file string
			// if helpers.IsValidUrl(s) {
			// 	file = s
			// } else {
			// 	file = cs.Rel(s)
			// }
			spec = spec + `
ADD ` + s + ` /luetbuild/`
		}
	}

	for _, c := range cs.Copy {
		if c.Image != "" {
			copyLine := fmt.Sprintf("\nCOPY --from=%s %s %s\n", c.Image, c.Source, c.Destination)
			spec = spec + copyLine
		}
	}

	for _, s := range cs.Env {
		spec = spec + `
ENV ` + s
	}

	for _, s := range steps {
		spec = spec + `
RUN ` + s
	}
	return spec
}

// RenderBuildImage renders the dockerfile of the image used as a pre-build step
func (cs *LuetCompilationSpec) RenderBuildImage() (string, error) {
	return cs.genDockerfile(cs.GetSeedImage(), cs.GetPreBuildSteps()), nil

}

// RenderStepImage renders the dockerfile used for the image used for building the package
func (cs *LuetCompilationSpec) RenderStepImage(image string) (string, error) {
	return cs.genDockerfile(image, cs.BuildSteps()), nil
}

func (cs *LuetCompilationSpec) WriteBuildImageDefinition(path string) error {

	data, err := cs.RenderBuildImage()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, []byte(data), 0644)
}

func (cs *LuetCompilationSpec) WriteStepImageDefinition(fromimage, path string) error {
	var data string
	var err error
	if cs.Package.OriginDockerfile != "" {
		// pre-rendered
		data = cs.Package.OriginDockerfile
	} else {
		data, err = cs.RenderStepImage(fromimage)
		if err != nil {
			return err
		}
	}

	return ioutil.WriteFile(path, []byte(data), 0644)
}
