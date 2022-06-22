// Copyright © 2019-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mudler/luet/pkg/helpers"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	version "github.com/mudler/luet/pkg/versioner"

	gentoo "github.com/Sabayon/pkgs-checker/pkg/gentoo"
	"github.com/crillab/gophersat/bf"
	"github.com/ghodss/yaml"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
)

type PackageAnnotation string

const (
	ConfigProtectAnnotation PackageAnnotation = "config_protect"
)

const (
	PackageMetaSuffix     = "metadata.yaml"
	PackageCollectionFile = "collection.yaml"
	PackageDefinitionFile = "definition.yaml"
)

type Packages []*Package

type PackageMap map[string]*Package

// Database is a merely simple in-memory db.
// FIXME: Use a proper structure or delegate to third-party
type PackageDatabase interface {
	PackageSet

	Get(s string) (string, error)
	Set(k, v string) error

	Create(string, []byte) (string, error)
	Retrieve(ID string) ([]byte, error)
}

type PackageFile struct {
	ID                 int `storm:"id,increment"` // primary key with auto increment
	PackageFingerprint string
	Files              []string
}

type PackageSet interface {
	Clone(PackageDatabase) error
	Copy() (PackageDatabase, error)

	GetRevdeps(p *Package) (Packages, error)
	GetPackages() []string //Ids
	CreatePackage(pkg *Package) (string, error)
	GetPackage(ID string) (*Package, error)
	Clean() error
	FindPackage(*Package) (*Package, error)
	FindPackages(p *Package) (Packages, error)
	UpdatePackage(p *Package) error
	GetAllPackages(packages chan *Package) error
	RemovePackage(*Package) error

	GetPackageFiles(*Package) ([]string, error)
	SetPackageFiles(*PackageFile) error
	RemovePackageFiles(*Package) error
	FindPackageVersions(p *Package) (Packages, error)
	World() Packages

	FindPackageCandidate(p *Package) (*Package, error)
	FindPackageLabel(labelKey string) (Packages, error)
	FindPackageLabelMatch(pattern string) (Packages, error)
	FindPackageMatch(pattern string) (Packages, error)
	FindPackageByFile(pattern string) (Packages, error)
}

func (pm PackageMap) String() string {
	rr := []string{}
	for _, r := range pm {

		rr = append(rr, r.HumanReadableString())

	}
	return fmt.Sprint(rr)
}

func (d Packages) Hash(salt string) string {

	overallFp := ""
	for _, c := range d {
		overallFp = overallFp + c.HashFingerprint("join")
	}
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s-%s", overallFp, salt))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// >> Unmarshallers
// PackageFromYaml decodes a package from yaml bytes
func PackageFromYaml(yml []byte) (Package, error) {

	var unescaped Package
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return Package{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return Package{}, err
	}
	err = json.Unmarshal(bytes, &unescaped)
	if err != nil {
		return Package{}, err
	}
	return unescaped, nil
}

type rawPackages []map[string]interface{}

func (r rawPackages) Find(wanted Package) map[string]interface{} {
	for _, v := range r {
		p := &Package{}
		dat, _ := json.Marshal(v)
		json.Unmarshal(dat, p)
		if wanted.Matches(p) {
			return v
		}
	}
	return map[string]interface{}{}
}

func GetRawPackages(yml []byte) (rawPackages, error) {
	var rawPackages struct {
		Packages []map[string]interface{} `yaml:"packages"`
	}
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return []map[string]interface{}{}, err
	}
	err = json.Unmarshal(bytes, &rawPackages)
	if err != nil {
		return []map[string]interface{}{}, err
	}
	return rawPackages.Packages, nil

}

type Collection struct {
	Packages []Package `json:"packages"`
}

func PackagesFromYAML(yml []byte) ([]Package, error) {

	var unescaped Collection
	source, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return []Package{}, err
	}

	rawIn := json.RawMessage(source)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return []Package{}, err
	}
	err = json.Unmarshal(bytes, &unescaped)
	if err != nil {
		return []Package{}, err
	}
	return unescaped.Packages, nil
}

// JSON returns the package in JSON form.
// Note this function sets a specific encoder as
// major and minor gets escaped when marshalling in JSON,
// making compiler fails recognizing selectors for expansion
func (t *Package) JSON() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

// GetMetadataFilePath returns the canonical name of an artifact metadata file
func (d *Package) GetMetadataFilePath() string {
	return fmt.Sprintf("%s.%s", d.GetFingerPrint(), PackageMetaSuffix)
}

// Package represent a standard package definition
type Package struct {
	ID               int        `storm:"id,increment" json:"id"` // primary key with auto increment
	Name             string     `json:"name"`                    // Affects YAML field names too.
	Version          string     `json:"version"`                 // Affects YAML field names too.
	Category         string     `json:"category"`                // Affects YAML field names too.
	UseFlags         []string   `json:"use_flags,omitempty"`     // Affects YAML field names too.
	State            State      `json:"state,omitempty"`
	PackageRequires  []*Package `json:"requires"`           // Affects YAML field names too.
	PackageConflicts []*Package `json:"conflicts"`          // Affects YAML field names too.
	Provides         []*Package `json:"provides,omitempty"` // Affects YAML field names too.
	Hidden           bool       `json:"hidden,omitempty"`   // Affects YAML field names too.

	// Annotations are used for core features/options
	Annotations map[PackageAnnotation]string `json:"annotations,omitempty"` // Affects YAML field names too

	// Path is set only internally when tree is loaded from disk
	Path string `json:"path,omitempty"`

	Description    string   `json:"description,omitempty"`
	Uri            []string `json:"uri,omitempty"`
	License        string   `json:"license,omitempty"`
	BuildTimestamp string   `json:"buildtimestamp,omitempty"`

	Labels map[string]string `json:"labels,omitempty"` // Affects YAML field names too.

	TreeDir string `json:"treedir,omitempty"`

	OriginDockerfile string `json:"dockerfile,omitempty"`
}

// State represent the package state
type State string

// NewPackage returns a new package
func NewPackage(name, version string, requires []*Package, conflicts []*Package) *Package {
	return &Package{
		Name:             name,
		Version:          version,
		PackageRequires:  requires,
		PackageConflicts: conflicts,
		Labels:           nil,
	}
}

func (p *Package) SetTreeDir(s string) {
	p.TreeDir = s
}
func (p *Package) GetTreeDir() string {
	return p.TreeDir
}

func (p *Package) SetOriginalDockerfile(s string) error {
	dat, err := ioutil.ReadFile(s)
	if err != nil {
		return errors.Wrap(err, "Error reading file "+s)
	}

	p.OriginDockerfile = string(dat)
	return nil
}

func (p *Package) String() string {
	b, err := p.JSON()
	if err != nil {
		return fmt.Sprintf("{ id: \"%d\", name: \"%s\", version: \"%s\", category: \"%s\"  }", p.ID, p.Name, p.Version, p.Category)
	}
	return fmt.Sprintf("%s", string(b))
}

// HasVersionDefined returns true when a specific version of a package is implied
func (p *Package) HasVersionDefined() bool {
	return p.Version != ">=0"
}

// GetFingerPrint returns a UUID of the package.
// FIXME: this needs to be unique, now just name is generalized
func (p *Package) GetFingerPrint() string {
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Category, p.Version)
}

func (p *Package) HashFingerprint(salt string) string {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s-%s", p.GetFingerPrint(), salt))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Package) HumanReadableString() string {
	switch {
	case p.Category != "" && p.Name != "" && p.Version == "":
		return fmt.Sprintf("%s/%s", p.Category, p.Name)
	case p.Category == "" && p.Name != "" && p.Version == "":
		return p.Name
	case p.Category == "" && p.Name != "" && p.Version != "":
		return fmt.Sprintf("%s@%s", p.Category, p.Name)
	default:
		return p.FullString()
	}
}

func (p *Package) FullString() string {
	return fmt.Sprintf("%s/%s-%s", p.Category, p.Name, p.Version)
}

func PackageFromString(s string) *Package {
	var unescaped Package

	err := json.Unmarshal([]byte(s), &unescaped)
	if err != nil {
		return &unescaped
	}
	return &unescaped
}

func (p *Package) GetPackageName() string {
	return fmt.Sprintf("%s-%s", p.Name, p.Category)
}

func (p *Package) ImageID() string {
	return helpers.SanitizeImageString(p.GetFingerPrint())
}

// GetBuildTimestamp returns the package build timestamp
func (p *Package) GetBuildTimestamp() string {
	return p.BuildTimestamp
}

// SetBuildTimestamp sets the package Build timestamp
func (p *Package) SetBuildTimestamp(s string) {
	p.BuildTimestamp = s
}

// GetPath returns the path where the definition file was found
func (p *Package) GetPath() string {
	return p.Path
}

func (p *Package) Rel(s string) string {
	return filepath.Join(p.GetPath(), s)
}

func (p *Package) SetPath(s string) {
	p.Path = s
}

func (p *Package) IsSelector() bool {
	return strings.ContainsAny(p.GetVersion(), "<>=")
}

func (p *Package) IsHidden() bool {
	return p.Hidden
}

func (p *Package) HasLabel(label string) (b bool) {
	for k := range p.Labels {
		if k == label {
			b = true
			return
		}
	}

	return
}

func (p *Package) MatchLabel(r *regexp.Regexp) (b bool) {
	for k, v := range p.Labels {
		if r.MatchString(k + "=" + v) {
			b = true
			return
		}
	}
	return
}

func (p Package) IsCollection() bool {
	return fileHelper.Exists(filepath.Join(p.Path, PackageCollectionFile))
}

func (p *Package) MatchAnnotation(r *regexp.Regexp) (b bool) {
	for k, v := range p.Annotations {
		if r.MatchString(string(k) + "=" + v) {
			b = true
			return
		}
	}
	return
}

// AddUse adds a use to a package
func (p *Package) AddUse(use string) {
	for _, v := range p.UseFlags {
		if v == use {
			return
		}
	}
	p.UseFlags = append(p.UseFlags, use)
}

// RemoveUse removes a use to a package
func (p *Package) RemoveUse(use string) {

	for i := len(p.UseFlags) - 1; i >= 0; i-- {
		if p.UseFlags[i] == use {
			p.UseFlags = append(p.UseFlags[:i], p.UseFlags[i+1:]...)
		}
	}

}

// Encode encodes the package to string.
// It returns an ID which can be used to retrieve the package later on.
func (p *Package) Encode(db PackageDatabase) (string, error) {
	return db.CreatePackage(p)
}

func (p *Package) Yaml() ([]byte, error) {
	j, err := p.JSON()
	if err != nil {
		return []byte{}, err
	}
	y, err := yaml.JSONToYAML(j)
	if err != nil {

		return []byte{}, err
	}
	return y, nil
}

func (p *Package) GetName() string {
	return p.Name
}

func (p *Package) GetVersion() string {
	return p.Version
}
func (p *Package) SetVersion(v string) {
	p.Version = v
}
func (p *Package) GetDescription() string {
	return p.Description
}
func (p *Package) SetDescription(s string) {
	p.Description = s
}
func (p *Package) GetLicense() string {
	return p.License
}
func (p *Package) SetLicense(s string) {
	p.License = s
}
func (p *Package) AddURI(s string) {
	p.Uri = append(p.Uri, s)
}
func (p *Package) GetURI() []string {
	return p.Uri
}
func (p *Package) GetCategory() string {
	return p.Category
}
func (p *Package) SetCategory(s string) {
	p.Category = s
}

func (p *Package) SetName(s string) {
	p.Name = s
}

func (p *Package) GetUses() []string {
	return p.UseFlags
}
func (p *Package) AddLabel(k, v string) {
	if p.Labels == nil {
		p.Labels = make(map[string]string, 0)
	}
	p.Labels[k] = v
}
func (p *Package) AddAnnotation(k, v string) {
	if p.Annotations == nil {
		p.Annotations = make(map[PackageAnnotation]string, 0)
	}
	p.Annotations[PackageAnnotation(k)] = v
}
func (p *Package) GetLabels() map[string]string {
	return p.Labels
}

func (p *Package) GetProvides() []*Package {
	return p.Provides
}
func (p *Package) SetProvides(req []*Package) *Package {
	p.Provides = req
	return p
}
func (p *Package) GetRequires() []*Package {
	return p.PackageRequires
}
func (p *Package) GetConflicts() []*Package {
	return p.PackageConflicts
}
func (p *Package) Requires(req []*Package) *Package {
	p.PackageRequires = req
	return p
}
func (p *Package) Conflicts(req []*Package) *Package {
	p.PackageConflicts = req
	return p
}
func (p *Package) Clone() *Package {
	new := &Package{}
	copier.Copy(&new, &p)
	return new
}
func (p *Package) Matches(m *Package) bool {
	if p.GetFingerPrint() == m.GetFingerPrint() {
		return true
	}
	return false
}

func (p *Package) AtomMatches(m *Package) bool {
	if p.GetName() == m.GetName() && p.GetCategory() == m.GetCategory() {
		return true
	}
	return false
}

func (p *Package) Mark() *Package {
	marked := p.Clone()
	marked.SetName("@@" + marked.GetName())
	return marked
}

func (p *Package) Expand(definitiondb PackageDatabase) (Packages, error) {
	var versionsInWorld Packages

	all, err := definitiondb.FindPackages(p)
	if err != nil {
		return nil, err
	}
	for _, w := range all {
		match, err := p.SelectorMatchVersion(w.GetVersion(), nil)
		if err != nil {
			return nil, err
		}
		if match {
			versionsInWorld = append(versionsInWorld, w)
		}
	}

	return versionsInWorld, nil
}

func (p *Package) Revdeps(definitiondb PackageDatabase) Packages {
	var versionsInWorld Packages
	for _, w := range definitiondb.World() {
		if w.Matches(p) {
			continue
		}
		for _, re := range w.GetRequires() {
			if re.Matches(p) {
				versionsInWorld = append(versionsInWorld, w)
				versionsInWorld = append(versionsInWorld, w.Revdeps(definitiondb)...)
			}
		}
	}

	return versionsInWorld
}

func walkPackage(p *Package, definitiondb PackageDatabase, visited map[string]interface{}) Packages {
	var versionsInWorld Packages
	if _, ok := visited[p.FullString()]; ok {
		return versionsInWorld
	}
	visited[p.FullString()] = true

	revdeps, _ := definitiondb.GetRevdeps(p)
	for _, r := range revdeps {
		versionsInWorld = append(versionsInWorld, r)
	}

	if !p.IsSelector() {
		versionsInWorld = append(versionsInWorld, p)
	}

	for _, re := range p.GetRequires() {
		versions, _ := re.Expand(definitiondb)
		for _, r := range versions {

			versionsInWorld = append(versionsInWorld, r)
			versionsInWorld = append(versionsInWorld, walkPackage(r, definitiondb, visited)...)
		}

	}
	for _, re := range p.GetConflicts() {
		versions, _ := re.Expand(definitiondb)
		for _, r := range versions {

			versionsInWorld = append(versionsInWorld, r)
			versionsInWorld = append(versionsInWorld, walkPackage(r, definitiondb, visited)...)

		}
	}
	return versionsInWorld.Unique()
}

func (p *Package) Related(definitiondb PackageDatabase) Packages {
	return walkPackage(p, definitiondb, map[string]interface{}{})
}

func (p *Package) LabelDeps(definitiondb PackageDatabase, labelKey string) Packages {
	var pkgsWithLabelInWorld Packages
	// TODO: check if integrate some index to improve
	// research instead of iterate all list.
	for _, w := range definitiondb.World() {
		if w.HasLabel(labelKey) {
			pkgsWithLabelInWorld = append(pkgsWithLabelInWorld, w)
		}
	}

	return pkgsWithLabelInWorld
}

func DecodePackage(ID string, db PackageDatabase) (*Package, error) {
	return db.GetPackage(ID)
}

func (pack *Package) scanRequires(definitiondb PackageDatabase, s *Package, visited map[string]interface{}) (bool, error) {
	if _, ok := visited[pack.FullString()]; ok {
		return false, nil
	}
	visited[pack.FullString()] = true
	p, err := definitiondb.FindPackage(pack)
	if err != nil {
		p = pack //relax things
		//return false, errors.Wrap(err, "Package not found in definition db")
	}

	for _, re := range p.GetRequires() {
		if re.Matches(s) {
			return true, nil
		}

		packages, _ := re.Expand(definitiondb)
		for _, pa := range packages {
			if pa.Matches(s) {
				return true, nil
			}
		}
		if contains, err := re.scanRequires(definitiondb, s, visited); err == nil && contains {
			return true, nil
		}
	}

	return false, nil
}

// RequiresContains recursively scans into the database packages dependencies to find a match with the given package
// It is used by the solver during uninstall.
func (pack *Package) RequiresContains(definitiondb PackageDatabase, s *Package) (bool, error) {
	return pack.scanRequires(definitiondb, s, make(map[string]interface{}))
}

// Best returns the best version of the package (the most bigger) from a list
// Accepts a versioner interface to change the ordering policy. If null is supplied
// It defaults to version.WrappedVersioner which supports both semver and debian versioning
func (set Packages) Best(v version.Versioner) *Package {
	if v == nil {
		v = &version.WrappedVersioner{}
	}
	var versionsMap map[string]*Package = make(map[string]*Package)
	if len(set) == 0 {
		panic("Best needs a list with elements")
	}

	versionsRaw := []string{}
	for _, p := range set {
		versionsRaw = append(versionsRaw, p.GetVersion())
		versionsMap[p.GetVersion()] = p
	}
	sorted := v.Sort(versionsRaw)

	return versionsMap[sorted[len(sorted)-1]]
}

func (set Packages) Find(packageName string) (*Package, error) {
	for _, p := range set {
		if p.GetPackageName() == packageName {
			return p, nil
		}
	}

	return nil, errors.New("package not found")
}

func (set Packages) Unique() Packages {
	var result Packages
	uniq := make(map[string]*Package)
	for _, p := range set {
		uniq[p.GetFingerPrint()] = p
	}
	for _, p := range uniq {
		result = append(result, p)
	}
	return result
}

func (p *Package) GetRuntimePackage() (*Package, error) {
	var r *Package
	if p.IsCollection() {
		collectionFile := filepath.Join(p.Path, PackageCollectionFile)
		dat, err := ioutil.ReadFile(collectionFile)
		if err != nil {
			return r, errors.Wrapf(err, "failed while reading '%s'", collectionFile)
		}
		coll, err := PackagesFromYAML(dat)
		if err != nil {
			return r, errors.Wrapf(err, "failed while parsing YAML '%s'", collectionFile)
		}
		for _, c := range coll {
			if c.Matches(p) {
				r = &c
				break
			}
		}
	} else if p.OriginDockerfile != "" {
		// XXX: There are no runtime metadata at the moment available except package name in this case
		// This needs to be adapted and aligned up with the tree parser
		return &Package{Name: p.Name}, nil
	} else {
		definitionFile := filepath.Join(p.Path, PackageDefinitionFile)
		dat, err := ioutil.ReadFile(definitionFile)
		if err != nil {
			return r, errors.Wrapf(err, "failed while reading '%s'", definitionFile)
		}
		d, err := PackageFromYaml(dat)
		if err != nil {
			return r, errors.Wrapf(err, "failed while parsing YAML '%s'", definitionFile)
		}
		r = &d
	}
	return r, nil
}

func (pack *Package) buildFormula(definitiondb PackageDatabase, db PackageDatabase, visited map[string]interface{}) ([]bf.Formula, error) {
	if _, ok := visited[pack.FullString()]; ok {
		return nil, nil
	}
	visited[pack.FullString()] = true
	p, err := definitiondb.FindPackage(pack)
	if err != nil {
		p = pack // Relax failures and trust the def
	}
	encodedA, err := p.Encode(db)
	if err != nil {
		return nil, err
	}

	A := bf.Var(encodedA)

	var formulas []bf.Formula

	// Do conflict with other packages versions (if A is selected, then conflict with other versions of A)
	packages, _ := definitiondb.FindPackageVersions(p)
	if len(packages) > 0 {
		for _, cp := range packages {
			encodedB, err := cp.Encode(db)
			if err != nil {
				return nil, err
			}
			B := bf.Var(encodedB)
			if !p.Matches(cp) {
				formulas = append(formulas, bf.Or(bf.Not(A), bf.Or(bf.Not(A), bf.Not(B))))
			}
		}
	}

	for _, requiredDef := range p.GetRequires() {
		required, err := definitiondb.FindPackage(requiredDef)
		if err != nil || requiredDef.IsSelector() {
			if err == nil {
				required = requiredDef
			}
			packages, err := definitiondb.FindPackages(requiredDef)
			if err != nil || len(packages) == 0 {
				required = requiredDef
			} else {

				var ALO []bf.Formula
				// AMO/ALO - At most/least one
				for _, o := range packages {
					encodedB, err := o.Encode(db)
					if err != nil {
						return nil, err
					}
					B := bf.Var(encodedB)
					ALO = append(ALO, B)
					for _, i := range packages {
						encodedI, err := i.Encode(db)
						if err != nil {
							return nil, err
						}
						I := bf.Var(encodedI)
						if !o.Matches(i) {
							formulas = append(formulas, bf.Or(bf.Not(A), bf.Or(bf.Not(I), bf.Not(B))))
						}
					}
				}
				formulas = append(formulas, bf.Or(bf.Not(A), bf.Or(ALO...))) // ALO - At least one
				continue
			}

		}

		encodedB, err := required.Encode(db)
		if err != nil {
			return nil, err
		}
		B := bf.Var(encodedB)
		formulas = append(formulas, bf.Or(bf.Not(A), B))
		f, err := required.buildFormula(definitiondb, db, visited)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, f...)

	}

	for _, requiredDef := range p.GetConflicts() {
		required, err := definitiondb.FindPackage(requiredDef)
		if err != nil || requiredDef.IsSelector() {
			if err == nil {
				requiredDef = required
			}
			packages, err := definitiondb.FindPackages(requiredDef)
			if err != nil || len(packages) == 0 {
				required = requiredDef
			} else {
				if len(packages) == 1 {
					required = packages[0]
				} else {
					for _, p := range packages {
						encodedB, err := p.Encode(db)
						if err != nil {
							return nil, err
						}
						B := bf.Var(encodedB)
						formulas = append(formulas, bf.Or(bf.Not(A),
							bf.Not(B)))
						f, err := p.buildFormula(definitiondb, db, visited)
						if err != nil {
							return nil, err
						}
						formulas = append(formulas, f...)
					}
					continue
				}
			}
		}

		encodedB, err := required.Encode(db)
		if err != nil {
			return nil, err
		}
		B := bf.Var(encodedB)
		formulas = append(formulas, bf.Or(bf.Not(A),
			bf.Not(B)))

		f, err := required.buildFormula(definitiondb, db, visited)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, f...)

	}

	return formulas, nil
}

func (pack *Package) BuildFormula(definitiondb PackageDatabase, db PackageDatabase) ([]bf.Formula, error) {
	return pack.buildFormula(definitiondb, db, make(map[string]interface{}))
}

func (p *Package) Explain() {

	fmt.Println("====================")
	fmt.Println("Name: ", p.GetName())
	fmt.Println("Category: ", p.GetCategory())
	fmt.Println("Version: ", p.GetVersion())

	for _, req := range p.GetRequires() {
		fmt.Println("\t-> ", req)
	}

	for _, con := range p.GetConflicts() {
		fmt.Println("\t!! ", con)
	}

	fmt.Println("====================")

}

func (p *Package) BumpBuildVersion() error {
	cat := p.Category
	if cat == "" {
		// Use fake category for parse package
		cat = "app"
	}
	gp, err := gentoo.ParsePackageStr(
		fmt.Sprintf("%s/%s-%s", cat,
			p.Name, p.GetVersion()))
	if err != nil {
		return errors.Wrap(err, "Error on parser version")
	}

	buildPrefix := ""
	buildId := 0

	if gp.VersionBuild != "" {
		// Check if version build is a number
		buildId, err = strconv.Atoi(gp.VersionBuild)
		if err == nil {
			goto end
		}
		// POST: is not only a number

		// TODO: check if there is a better way to handle all use cases.

		r1 := regexp.MustCompile(`^r[0-9]*$`)
		if r1 == nil {
			return errors.New("Error on create regex for -r[0-9]")
		}
		if r1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "r", ""))
			if err == nil {
				buildPrefix = "r"
				goto end
			}
		}

		p1 := regexp.MustCompile(`^p[0-9]*$`)
		if p1 == nil {
			return errors.New("Error on create regex for -p[0-9]")
		}
		if p1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "p", ""))
			if err == nil {
				buildPrefix = "p"
				goto end
			}
		}

		rc1 := regexp.MustCompile(`^rc[0-9]*$`)
		if rc1 == nil {
			return errors.New("Error on create regex for -rc[0-9]")
		}
		if rc1.MatchString(gp.VersionBuild) {
			buildId, err = strconv.Atoi(strings.ReplaceAll(gp.VersionBuild, "rc", ""))
			if err == nil {
				buildPrefix = "rc"
				goto end
			}
		}

		// Check if version build contains a dot
		dotIdx := strings.LastIndex(gp.VersionBuild, ".")
		if dotIdx > 0 {
			buildPrefix = gp.VersionBuild[0 : dotIdx+1]
			bVersion := gp.VersionBuild[dotIdx+1:]
			buildId, err = strconv.Atoi(bVersion)
			if err == nil {
				goto end
			}
		}

		buildPrefix = gp.VersionBuild + "."
		buildId = 0
	}

end:

	buildId++
	p.Version = fmt.Sprintf("%s%s+%s%d",
		gp.Version, gp.VersionSuffix, buildPrefix, buildId)

	return nil
}

func (p *Package) SelectorMatchVersion(ver string, v version.Versioner) (bool, error) {
	if !p.IsSelector() {
		return false, errors.New("Package is not a selector")
	}
	if v == nil {
		v = &version.WrappedVersioner{}
	}

	return v.ValidateSelector(ver, p.GetVersion()), nil
}

func (p *Package) VersionMatchSelector(selector string, v version.Versioner) (bool, error) {
	if v == nil {
		v = &version.WrappedVersioner{}
	}

	return v.ValidateSelector(p.GetVersion(), selector), nil
}
