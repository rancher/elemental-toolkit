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

package database

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

var DBInMemoryInstance = &InMemoryDatabase{
	Mutex:            &sync.Mutex{},
	FileDatabase:     map[string][]string{},
	Database:         map[string]string{},
	CacheNoVersion:   map[string]map[string]interface{}{},
	ProvidesDatabase: map[string]map[string]*types.Package{},
	RevDepsDatabase:  map[string]map[string]*types.Package{},
	cached:           map[string]interface{}{},
}

type InMemoryDatabase struct {
	*sync.Mutex
	Database         map[string]string
	FileDatabase     map[string][]string
	CacheNoVersion   map[string]map[string]interface{}
	ProvidesDatabase map[string]map[string]*types.Package
	RevDepsDatabase  map[string]map[string]*types.Package
	cached           map[string]interface{}
}

func NewInMemoryDatabase(singleton bool) types.PackageDatabase {
	// In memoryDB is a singleton
	if !singleton {
		return &InMemoryDatabase{
			Mutex:            &sync.Mutex{},
			FileDatabase:     map[string][]string{},
			Database:         map[string]string{},
			CacheNoVersion:   map[string]map[string]interface{}{},
			ProvidesDatabase: map[string]map[string]*types.Package{},
			RevDepsDatabase:  map[string]map[string]*types.Package{},
			cached:           map[string]interface{}{},
		}
	}
	return DBInMemoryInstance
}

func (db *InMemoryDatabase) Get(s string) (string, error) {
	db.Lock()
	defer db.Unlock()
	pa, ok := db.Database[s]
	if !ok {
		return "", errors.New(fmt.Sprintf("No key found for: %s", s))
	}
	return pa, nil
}

func (db *InMemoryDatabase) Set(k, v string) error {
	db.Lock()
	defer db.Unlock()
	db.Database[k] = v

	return nil
}

func (db *InMemoryDatabase) Create(id string, v []byte) (string, error) {
	enc := base64.StdEncoding.EncodeToString(v)

	return id, db.Set(id, enc)
}

func (db *InMemoryDatabase) Retrieve(ID string) ([]byte, error) {
	pa, err := db.Get(ID)
	if err != nil {
		return nil, err
	}

	enc, err := base64.StdEncoding.DecodeString(pa)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

func (db *InMemoryDatabase) GetPackage(ID string) (*types.Package, error) {

	enc, err := db.Retrieve(ID)
	if err != nil {
		return nil, err
	}

	p := &types.Package{}

	rawIn := json.RawMessage(enc)
	bytes, err := rawIn.MarshalJSON()
	if err != nil {
		return p, err
	}

	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, err
	}
	return p, nil
}

func (db *InMemoryDatabase) GetAllPackages(packages chan *types.Package) error {
	packs := db.GetPackages()
	for _, p := range packs {
		pack, err := db.GetPackage(p)
		if err != nil {
			return err
		}
		packages <- pack
	}
	return nil
}

func (db *InMemoryDatabase) getRevdeps(p *types.Package, visited map[string]interface{}) (types.Packages, error) {
	var versionsInWorld types.Packages
	if _, ok := visited[p.FullString()]; ok {
		return versionsInWorld, nil
	}
	visited[p.FullString()] = true

	var res types.Packages
	packs, err := db.FindPackages(p)
	if err != nil {
		return res, err
	}

	for _, pp := range packs {
		//	db.Lock()
		list := db.RevDepsDatabase[pp.GetFingerPrint()]
		//	db.Unlock()
		for _, revdep := range list {
			dep, err := db.FindPackage(revdep)
			if err != nil {
				return res, err
			}
			res = append(res, dep)

			packs, err := db.getRevdeps(dep, visited)
			if err != nil {
				return res, err
			}
			res = append(res, packs...)

		}
	}
	return res.Unique(), nil
}

// GetRevdeps returns the package reverse dependencies,
// matching also selectors in versions (>, <, >=, <=)
// TODO: Code should use db explictly
func (db *InMemoryDatabase) GetRevdeps(p *types.Package) (types.Packages, error) {
	return db.getRevdeps(p, make(map[string]interface{}))
}

// Encode encodes the package to string.
// It returns an ID which can be used to retrieve the package later on.
func (db *InMemoryDatabase) CreatePackage(p *types.Package) (string, error) {

	res, err := p.JSON()
	if err != nil {
		return "", err
	}

	ID, err := db.Create(p.GetFingerPrint(), res)
	if err != nil {
		return "", err
	}

	db.populateCaches(p)

	return ID, nil
}

func (db *InMemoryDatabase) updateRevDep(k, v string, b *types.Package) {
	_, ok := db.RevDepsDatabase[k]
	if !ok {
		db.RevDepsDatabase[k] = make(map[string]*types.Package)
	}
	db.RevDepsDatabase[k][v] = b.Clone()
}

func (db *InMemoryDatabase) populateCaches(pd *types.Package) {

	// Create extra cache between package -> []versions
	db.Lock()
	if db.cached == nil {
		db.cached = map[string]interface{}{}
	}

	if _, ok := db.cached[pd.GetFingerPrint()]; ok {
		db.Unlock()
		return
	}
	db.cached[pd.GetFingerPrint()] = nil

	// Provides: Store package provides, we will reuse this when walking deps
	for _, provide := range pd.Provides {
		if _, ok := db.ProvidesDatabase[provide.GetPackageName()]; !ok {
			db.ProvidesDatabase[provide.GetPackageName()] = make(map[string]*types.Package)

		}

		db.ProvidesDatabase[provide.GetPackageName()][provide.GetVersion()] = pd
	}

	_, ok := db.CacheNoVersion[pd.GetPackageName()]
	if !ok {
		db.CacheNoVersion[pd.GetPackageName()] = make(map[string]interface{})
	}
	db.CacheNoVersion[pd.GetPackageName()][pd.GetVersion()] = nil

	db.Unlock()

	// Updating Revdeps
	// Given that when we populate the cache we don't have the full db at hand
	// We cycle over reverse dependency of a package to update their entry if they are matching
	// the version selector
	db.Lock()
	toUpdate, ok := db.RevDepsDatabase[pd.GetPackageName()]
	if ok {
		for _, pp := range toUpdate {
			for _, re := range pp.GetRequires() {
				if match, _ := pd.VersionMatchSelector(re.GetVersion(), nil); match {
					db.updateRevDep(pd.GetFingerPrint(), pp.GetFingerPrint(), pp)
				}
			}
		}
	}
	db.Unlock()

	for _, re := range pd.GetRequires() {
		packages, _ := db.FindPackages(re)
		db.Lock()
		for _, pa := range packages {
			db.updateRevDep(pa.GetFingerPrint(), pd.GetFingerPrint(), pd)
			db.updateRevDep(pa.GetPackageName(), pd.GetPackageName(), pd)
		}
		db.updateRevDep(re.GetFingerPrint(), pd.GetFingerPrint(), pd)
		db.updateRevDep(re.GetPackageName(), pd.GetPackageName(), pd)
		db.Unlock()
	}
}

func (db *InMemoryDatabase) getProvide(p *types.Package) (*types.Package, error) {

	db.Lock()

	pa, ok := db.ProvidesDatabase[p.GetPackageName()][p.GetVersion()]
	if !ok {
		versions, ok := db.ProvidesDatabase[p.GetPackageName()]
		defer db.Unlock()

		if !ok {
			return nil, errors.New("No versions found for package")
		}

		for ve, _ := range versions {

			match, err := p.VersionMatchSelector(ve, nil)
			if err != nil {
				return nil, errors.Wrap(err, "Error on match version")
			}
			if match {
				pa, ok := db.ProvidesDatabase[p.GetPackageName()][ve]
				if !ok {
					return nil, errors.New("No versions found for package")
				}
				return pa, nil
			}
		}

		return nil, errors.New("No package provides this")
	}
	db.Unlock()

	return db.FindPackage(pa)
}

func (db *InMemoryDatabase) Clone(to types.PackageDatabase) error {
	return clone(db, to)
}

func (db *InMemoryDatabase) Copy() (types.PackageDatabase, error) {
	return copy(db)
}

func (db *InMemoryDatabase) encodePackage(pd *types.Package) (string, string, error) {
	res, err := pd.JSON()
	if err != nil {
		return "", "", err
	}
	enc := base64.StdEncoding.EncodeToString(res)

	return pd.GetFingerPrint(), enc, nil
}

func (db *InMemoryDatabase) FindPackage(p *types.Package) (*types.Package, error) {

	// Provides: Return the replaced package here
	if provided, err := db.getProvide(p); err == nil {
		return provided, nil
	}

	return db.GetPackage(p.GetFingerPrint())
}

// FindPackages return the list of the packages beloging to cat/name
func (db *InMemoryDatabase) FindPackageVersions(p *types.Package) (types.Packages, error) {
	// Provides: Treat as the replaced package here
	if provided, err := db.getProvide(p); err == nil {
		p = provided
	}
	db.Lock()
	versions, ok := db.CacheNoVersion[p.GetPackageName()]
	db.Unlock()
	if !ok {
		return nil, errors.New("No versions found for package")
	}
	var versionsInWorld []*types.Package
	for ve, _ := range versions {
		w, err := db.FindPackage(&types.Package{Name: p.GetName(), Category: p.GetCategory(), Version: ve})
		if err != nil {
			return nil, errors.Wrap(err, "Cache mismatch - this shouldn't happen")
		}
		versionsInWorld = append(versionsInWorld, w)
	}
	return types.Packages(versionsInWorld), nil
}

// FindPackages return the list of the packages beloging to cat/name (any versions in requested range)
func (db *InMemoryDatabase) FindPackages(p *types.Package) (types.Packages, error) {
	if !p.IsSelector() {
		pack, err := db.FindPackage(p)
		if err != nil {
			return []*types.Package{}, err
		}
		return []*types.Package{pack}, nil
	}
	// Provides: Treat as the replaced package here
	if provided, err := db.getProvide(p); err == nil {
		p = provided
		if !provided.IsSelector() {
			return types.Packages{provided}, nil
		}
	}

	db.Lock()
	var matches []*types.Package
	versions, ok := db.CacheNoVersion[p.GetPackageName()]
	for ve := range versions {
		match, _ := p.SelectorMatchVersion(ve, nil)
		if match {
			matches = append(matches, &types.Package{Name: p.GetName(), Category: p.GetCategory(), Version: ve})
		}
	}
	db.Unlock()
	if !ok {
		return nil, fmt.Errorf("No versions found for: %s", p.HumanReadableString())
	}
	var versionsInWorld []*types.Package
	for _, p := range matches {
		w, err := db.FindPackage(p)
		if err != nil {
			return nil, errors.Wrap(err, "Cache mismatch - this shouldn't happen")
		}
		versionsInWorld = append(versionsInWorld, w)
	}
	return types.Packages(versionsInWorld), nil
}

func (db *InMemoryDatabase) UpdatePackage(p *types.Package) error {

	_, enc, err := db.encodePackage(p)
	if err != nil {
		return err
	}

	return db.Set(p.GetFingerPrint(), enc)
}

func (db *InMemoryDatabase) GetPackages() []string {
	keys := []string{}
	db.Lock()
	defer db.Unlock()
	for k := range db.Database {
		keys = append(keys, k)
	}
	return keys
}

func (db *InMemoryDatabase) Clean() error {
	db.Database = map[string]string{}
	return nil
}

func (db *InMemoryDatabase) GetPackageFiles(p *types.Package) ([]string, error) {

	db.Lock()
	defer db.Unlock()

	pa, ok := db.FileDatabase[p.GetFingerPrint()]
	if !ok {
		return pa, fmt.Errorf("No key found for: %s", p.HumanReadableString())
	}

	return pa, nil
}
func (db *InMemoryDatabase) SetPackageFiles(p *types.PackageFile) error {
	db.Lock()
	defer db.Unlock()
	db.FileDatabase[p.PackageFingerprint] = p.Files
	return nil
}
func (db *InMemoryDatabase) RemovePackageFiles(p *types.Package) error {
	db.Lock()
	defer db.Unlock()
	delete(db.FileDatabase, p.GetFingerPrint())
	return nil
}

func (db *InMemoryDatabase) RemovePackage(p *types.Package) error {
	db.Lock()
	defer db.Unlock()
	if _, exists := db.CacheNoVersion[p.GetPackageName()]; exists {
		delete(db.CacheNoVersion[p.GetPackageName()], p.GetVersion())
	}
	delete(db.Database, p.GetFingerPrint())
	return nil
}
func (db *InMemoryDatabase) World() types.Packages {
	var all []*types.Package
	// FIXME: This should all be locked in the db - for now forbid the solver to be run in threads.
	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err == nil {
			all = append(all, pack)
		}
	}
	return types.Packages(all)
}

func (db *InMemoryDatabase) FindPackageCandidate(p *types.Package) (*types.Package, error) {

	required, err := db.FindPackage(p)
	if err != nil {
		err = nil
		//	return nil, errors.Wrap(err, "Couldn't find required package in db definition")
		packages, err := p.Expand(db)
		//	Info("Expanded", packages, err)
		if err != nil || len(packages) == 0 {
			required = p
			err = errors.Wrap(err, "Candidate not found")
		} else {
			required = packages.Best(nil)
		}
		return required, err
		//required = &types.Package{Name: "test"}
	}

	return required, err

}

func (db *InMemoryDatabase) FindPackageLabel(labelKey string) (types.Packages, error) {
	var ans []*types.Package

	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err != nil {
			return ans, err
		}
		if pack.HasLabel(labelKey) {
			ans = append(ans, pack)
		}
	}

	return types.Packages(ans), nil
}

func (db *InMemoryDatabase) FindPackageLabelMatch(pattern string) (types.Packages, error) {
	var ans []*types.Package

	re := regexp.MustCompile(pattern)
	if re == nil {
		return nil, errors.New("Invalid regex " + pattern + "!")
	}

	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err != nil {
			return ans, err
		}
		if pack.MatchLabel(re) {
			ans = append(ans, pack)
		}
	}

	return types.Packages(ans), nil
}

func (db *InMemoryDatabase) FindPackageMatch(pattern string) (types.Packages, error) {
	var ans []*types.Package

	re := regexp.MustCompile(pattern)
	if re == nil {
		return nil, errors.New("Invalid regex " + pattern + "!")
	}

	for _, k := range db.GetPackages() {
		pack, err := db.GetPackage(k)
		if err != nil {
			return ans, err
		}

		if re.MatchString(pack.HumanReadableString()) {
			ans = append(ans, pack)
		}
	}

	return types.Packages(ans), nil
}

func (db *InMemoryDatabase) FindPackageByFile(pattern string) (types.Packages, error) {
	return findPackageByFile(db, pattern)
}
