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

package compiler

import (
	"fmt"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

// ImageHashTree is holding the Database
// and the options to resolve PackageImageHashTrees
// for a given specfile
// It is responsible of returning a concrete result
// which identifies a Package in a HashTree
type ImageHashTree struct {
	Database      types.PackageDatabase
	SolverOptions types.LuetSolverOptions
}

// PackageImageHashTree represent the Package into a given image hash tree
// The hash tree is constructed by a set of images representing
// the package during its build stage. A Hash is assigned to each image
// from the package fingerprint, plus the SAT solver assertion result (which is hashed as well)
// and the specfile signatures. This guarantees that each image of the build stage
// is unique and can be identified later on.
type PackageImageHashTree struct {
	Target                       *types.PackageAssert
	Dependencies                 types.PackagesAssertions
	Solution                     types.PackagesAssertions
	dependencyBuilderImageHashes map[string]string
	SourceHash                   string
	BuilderImageHash             string
}

func NewHashTree(db types.PackageDatabase) *ImageHashTree {
	return &ImageHashTree{
		Database: db,
	}
}

func (ht *PackageImageHashTree) DependencyBuildImage(p *types.Package) (string, error) {
	found, ok := ht.dependencyBuilderImageHashes[p.GetFingerPrint()]
	if !ok {
		return "", errors.New("package hash not found")
	}
	return found, nil
}

func (ht *PackageImageHashTree) String() string {
	return fmt.Sprintf(
		"Target buildhash: %s\nTarget packagehash: %s\nBuilder Imagehash: %s\nSource Imagehash: %s\n",
		ht.Target.Hash.BuildHash,
		ht.Target.Hash.PackageHash,
		ht.BuilderImageHash,
		ht.SourceHash,
	)
}

// Query takes a compiler and a compilation spec and returns a PackageImageHashTree tied to it.
// PackageImageHashTree contains all the informations to resolve the spec build images in order to
// reproducibly re-build images from packages
func (ht *ImageHashTree) Query(cs *LuetCompiler, p *types.LuetCompilationSpec) (*PackageImageHashTree, error) {
	assertions, err := ht.resolve(cs, p)
	if err != nil {
		return nil, err
	}
	targetAssertion := assertions.Search(p.GetPackage().GetFingerPrint())

	dependencies := assertions.Drop(p.GetPackage())
	var sourceHash string
	imageHashes := map[string]string{}
	for _, assertion := range dependencies {
		var depbuildImageTag string
		compileSpec, err := cs.FromPackage(assertion.Package)
		if err != nil {
			return nil, errors.Wrap(err, "Error while generating compilespec for "+assertion.Package.GetName())
		}
		if compileSpec.GetImage() != "" {
			depbuildImageTag = assertion.Hash.BuildHash
		} else {
			depbuildImageTag = ht.genBuilderImageTag(compileSpec, targetAssertion.Hash.PackageHash)
		}
		imageHashes[assertion.Package.GetFingerPrint()] = depbuildImageTag
		sourceHash = assertion.Hash.PackageHash
	}

	return &PackageImageHashTree{
		Dependencies:                 dependencies,
		Target:                       targetAssertion,
		SourceHash:                   sourceHash,
		BuilderImageHash:             ht.genBuilderImageTag(p, targetAssertion.Hash.PackageHash),
		dependencyBuilderImageHashes: imageHashes,
		Solution:                     assertions,
	}, nil
}

func (ht *ImageHashTree) genBuilderImageTag(p *types.LuetCompilationSpec, packageImage string) string {
	// Use packageImage as salt into the fp being used
	// so the hash is unique also in cases where
	// some package deps does have completely different
	// depgraphs
	return fmt.Sprintf("builder-%s", p.GetPackage().HashFingerprint(packageImage))
}

// resolve computes the dependency tree of a compilation spec and returns solver assertions
// in order to be able to compile the spec.
func (ht *ImageHashTree) resolve(cs *LuetCompiler, p *types.LuetCompilationSpec) (types.PackagesAssertions, error) {
	dependencies, err := cs.ComputeDepTree(p, cs.Database)
	if err != nil {
		return nil, errors.Wrap(err, "While computing a solution for "+p.GetPackage().HumanReadableString())
	}

	// Get hash from buildpsecs
	salts := map[string]string{}
	for _, assertion := range dependencies { //highly dependent on the order
		if assertion.Value {
			spec, err := cs.FromPackage(assertion.Package)
			if err != nil {
				return nil, errors.Wrap(err, "while computing hash buildspecs")
			}
			hash, err := spec.Hash()
			if err != nil {
				return nil, errors.Wrap(err, "failed computing hash")
			}
			salts[assertion.Package.GetFingerPrint()] = hash
		}
	}

	assertions := types.PackagesAssertions{}
	for _, assertion := range dependencies { //highly dependent on the order
		if assertion.Value {
			nthsolution := dependencies.Cut(assertion.Package)
			assertion.Hash = types.PackageHash{
				BuildHash:   nthsolution.SaltedHashFrom(assertion.Package, salts),
				PackageHash: nthsolution.SaltedAssertionHash(salts),
			}
			assertion.Package.SetTreeDir(p.Package.GetTreeDir())
			assertions = append(assertions, assertion)
		}
	}

	return assertions, nil
}
