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

package database

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"

	storm "github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"go.etcd.io/bbolt"
)

//var BoltInstance types.PackageDatabase

type BoltDatabase struct {
	sync.Mutex
	timeout          time.Duration
	Path             string
	ProvidesDatabase map[string]map[string]*types.Package
}

func checkMigrationSchema(path string) {
	b, err := storm.Open(path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 10 * time.Second}))
	if err != nil {
		return
	}
	defer b.Close()

	for _, m := range migrations {
		m(b)
	}
}

func NewBoltDatabase(path string) types.PackageDatabase {
	checkMigrationSchema(path)

	return &BoltDatabase{
		timeout: 30 * time.Second,
		Path:    path, ProvidesDatabase: map[string]map[string]*types.Package{}}
}

func (db *BoltDatabase) Clone(to types.PackageDatabase) error {
	return clone(db, to)
}

func (db *BoltDatabase) Copy() (types.PackageDatabase, error) {
	return copy(db)
}

func (db *BoltDatabase) Get(s string) (string, error) {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: db.timeout}))
	if err != nil {
		return "", err
	}
	defer bolt.Close()
	var str string
	bolt.Get("solver", s, &str)

	return str, errors.New("Not implemented")
}

func (db *BoltDatabase) Set(k, v string) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: db.timeout}))
	if err != nil {
		return err
	}
	defer bolt.Close()
	return bolt.Set("solver", k, v)
}

func (db *BoltDatabase) Create(id string, v []byte) (string, error) {
	enc := base64.StdEncoding.EncodeToString(v)

	return id, db.Set(id, enc)
}

func (db *BoltDatabase) Retrieve(ID string) ([]byte, error) {
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

// GetRevdeps uses a new inmemory db to calcuate revdeps
// TODO: Have a memory instance for boltdb, so we don't compute each time we get called
// as this is REALLY expensive. But we don't perform usually those operations in a file db.
func (db *BoltDatabase) GetRevdeps(p *types.Package) (types.Packages, error) {
	memory, err := db.Copy()
	if err != nil {
		return nil, errors.New("Failed copying bolt db to memory")
	}
	return memory.GetRevdeps(p)
}

func (db *BoltDatabase) FindPackage(tofind *types.Package) (*types.Package, error) {
	// Provides: Return the replaced package here
	if provided, err := db.getProvide(tofind); err == nil {
		return provided, nil
	}

	p := &types.Package{}
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return nil, err
	}
	defer bolt.Close()

	err = bolt.Select(q.Eq("Name", tofind.GetName()), q.Eq("Category", tofind.GetCategory()), q.Eq("Version", tofind.GetVersion())).Limit(1).First(p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *BoltDatabase) UpdatePackage(p *types.Package) error {
	// TODO: Change, but by query we cannot update by ID
	err := db.RemovePackage(p)
	if err != nil {
		return err
	}
	_, err = db.CreatePackage(p)

	if err != nil {
		return err
	}

	return nil
}

func (db *BoltDatabase) GetPackage(ID string) (*types.Package, error) {
	p := &types.Package{}
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return nil, err
	}
	defer bolt.Close()
	iid, err := strconv.Atoi(ID)
	if err != nil {
		return nil, err
	}
	err = bolt.Select(q.Eq("ID", iid)).Limit(1).First(p)

	//err = bolt.One("id", iid, p)
	return p, err
}

func (db *BoltDatabase) GetPackages() []string {
	ids := []string{}
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return []string{}
	}
	defer bolt.Close()
	// Fetching records one by one (useful when the bucket contains a lot of records)
	query := bolt.Select()

	query.Each(new(types.Package), func(record interface{}) error {
		u := record.(*types.Package)
		ids = append(ids, strconv.Itoa(u.ID))
		return nil
	})
	return ids
}

func (db *BoltDatabase) GetAllPackages(packages chan *types.Package) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return err
	}
	defer bolt.Close()
	var packs []types.Package
	err = bolt.All(&packs)
	if err != nil {
		return err
	}

	for _, r := range packs {
		packages <- &r
	}

	return nil
}

// Encode encodes the package to string.
// It returns an ID which can be used to retrieve the package later on.
func (db *BoltDatabase) CreatePackage(p *types.Package) (string, error) {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return "", errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()

	err = bolt.Save(p)
	if err != nil {
		return "", errors.Wrap(err, "Error saving package to "+db.Path)
	}

	// Create extra cache between package -> []versions
	db.Lock()
	defer db.Unlock()
	// TODO: Replace with a bolt implementation (and not in memory)
	// Provides: Store package provides, we will reuse this when walking deps
	for _, provide := range p.Provides {
		if _, ok := db.ProvidesDatabase[provide.GetPackageName()]; !ok {
			db.ProvidesDatabase[provide.GetPackageName()] = make(map[string]*types.Package)

		}

		db.ProvidesDatabase[provide.GetPackageName()][provide.GetVersion()] = p
	}

	return strconv.Itoa(p.ID), err
}

// Dup from memory implementation
func (db *BoltDatabase) getProvide(p *types.Package) (*types.Package, error) {
	db.Lock()
	pa, ok := db.ProvidesDatabase[p.GetPackageName()][p.GetVersion()]
	if !ok {
		versions, ok := db.ProvidesDatabase[p.GetPackageName()]
		db.Unlock()

		if !ok {
			return nil, errors.New(fmt.Sprintf("No versions found for: %s", p.HumanReadableString()))
		}

		for ve, _ := range versions {

			match, err := p.VersionMatchSelector(ve, nil)
			if err != nil {
				return nil, errors.Wrap(err, "Error on match version")
			}
			if match {
				pa, ok := db.ProvidesDatabase[p.GetPackageName()][ve]
				if !ok {
					return nil, errors.New(fmt.Sprintf("No versions found for: %s", p.HumanReadableString()))
				}
				return pa, nil //pick the first (we shouldn't have providers that are conflicting)
				// TODO: A find dbcall here would recurse, but would give chance to have providers of providers
			}
		}

		return nil, errors.New("No package provides this")
	}
	db.Unlock()
	return db.FindPackage(pa)
}

func (db *BoltDatabase) Clean() error {
	db.Lock()
	defer db.Unlock()
	return os.RemoveAll(db.Path)
}

func (db *BoltDatabase) GetPackageFiles(p *types.Package) ([]string, error) {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return []string{}, errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()

	files := bolt.From("files")
	var pf types.PackageFile
	err = files.One("PackageFingerprint", p.GetFingerPrint(), &pf)
	if err != nil {
		return []string{}, errors.Wrap(err, "While finding files")
	}
	return pf.Files, nil
}
func (db *BoltDatabase) SetPackageFiles(p *types.PackageFile) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()

	files := bolt.From("files")
	return files.Save(p)
}
func (db *BoltDatabase) RemovePackageFiles(p *types.Package) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()

	files := bolt.From("files")
	var pf types.PackageFile
	err = files.One("PackageFingerprint", p.GetFingerPrint(), &pf)
	if err != nil {
		return errors.Wrap(err, "While finding files")
	}
	return files.DeleteStruct(&pf)
}

func (db *BoltDatabase) RemovePackage(p *types.Package) error {
	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return errors.Wrap(err, "Error opening boltdb "+db.Path)
	}
	defer bolt.Close()
	var found types.Package
	err = bolt.Select(q.Eq("Name", p.GetName()), q.Eq("Category", p.GetCategory()), q.Eq("Version", p.GetVersion())).Limit(1).Delete(&found)
	if err != nil {
		return errors.New(fmt.Sprintf("Package not found: %s", p.HumanReadableString()))
	}
	return nil
}

func (db *BoltDatabase) World() types.Packages {
	var packs []types.Package

	bolt, err := storm.Open(db.Path, storm.BoltOptions(0600, &bbolt.Options{Timeout: 30 * time.Second}))
	if err != nil {
		return types.Packages([]*types.Package{})
	}
	defer bolt.Close()
	err = bolt.All(&packs)
	if err != nil {
		return types.Packages([]*types.Package{})
	}
	models := make([]*types.Package, len(packs))
	for i, _ := range packs {
		models[i] = &packs[i]
	}

	return types.Packages(models)
}

func (db *BoltDatabase) FindPackageCandidate(p *types.Package) (*types.Package, error) {

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
		//required = &*types.Package{Name: "test"}
	}

	return required, err

}

// FindPackages return the list of the packages beloging to cat/name  (any versions in requested range)
// FIXME: Optimize, see inmemorydb
func (db *BoltDatabase) FindPackages(p *types.Package) (types.Packages, error) {
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

	var versionsInWorld []*types.Package
	for _, w := range db.World() {
		if w.GetName() != p.GetName() || w.GetCategory() != p.GetCategory() {
			continue
		}

		match, err := p.SelectorMatchVersion(w.GetVersion(), nil)
		if err != nil {
			return nil, errors.Wrap(err, "Error on match selector")
		}
		if match {
			versionsInWorld = append(versionsInWorld, w)
		}
	}
	return types.Packages(versionsInWorld), nil
}

// FindPackageVersions return the list of the packages beloging to cat/name
func (db *BoltDatabase) FindPackageVersions(p *types.Package) (types.Packages, error) {
	// Provides: Treat as the replaced package here
	if provided, err := db.getProvide(p); err == nil {
		p = provided
	}

	var versionsInWorld []*types.Package
	for _, w := range db.World() {
		if w.GetName() != p.GetName() || w.GetCategory() != p.GetCategory() {
			continue
		}

		versionsInWorld = append(versionsInWorld, w)
	}
	return types.Packages(versionsInWorld), nil
}

func (db *BoltDatabase) FindPackageLabel(labelKey string) (types.Packages, error) {
	var ans []*types.Package

	for _, pack := range db.World() {
		if pack.HasLabel(labelKey) {
			ans = append(ans, pack)
		}
	}
	return types.Packages(ans), nil
}

func (db *BoltDatabase) FindPackageLabelMatch(pattern string) (types.Packages, error) {
	var ans []*types.Package

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid regex "+pattern+"!")
	}

	for _, pack := range db.World() {
		if pack.MatchLabel(re) {
			ans = append(ans, pack)
		}
	}

	return types.Packages(ans), nil
}

func (db *BoltDatabase) FindPackageByFile(pattern string) (types.Packages, error) {
	return findPackageByFile(db, pattern)
}
func (db *BoltDatabase) FindPackageMatch(pattern string) (types.Packages, error) {
	var ans []*types.Package

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid regex "+pattern+"!")
	}

	for _, pack := range db.World() {
		if re.MatchString(pack.HumanReadableString()) {
			ans = append(ans, pack)
		}
	}

	return types.Packages(ans), nil
}
