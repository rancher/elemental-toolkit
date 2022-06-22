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

package solver

import (

	//. "github.com/mudler/luet/pkg/logger"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/crillab/gophersat/bf"
	"github.com/mudler/luet/pkg/api/core/types"
	pkg "github.com/mudler/luet/pkg/database"
)

var AvailableResolvers = strings.Join([]string{QLearningResolverType}, " ")

// Solver is the default solver for luet
type Solver struct {
	DefinitionDatabase types.PackageDatabase
	SolverDatabase     types.PackageDatabase
	Wanted             types.Packages
	InstalledDatabase  types.PackageDatabase

	Resolver types.PackageResolver
}

// IsRelaxedResolver returns true wether a solver might
// take action on user side, by removing some installation constraints
// or taking automated actions (e.g. qlearning)
func IsRelaxedResolver(t types.LuetSolverOptions) bool {
	return t.Type == QLearningResolverType
}

// NewSolver accepts as argument two lists of packages, the first is the initial set,
// the second represent all the known packages.
func NewSolver(t types.SolverOptions, installed types.PackageDatabase, definitiondb types.PackageDatabase, solverdb types.PackageDatabase) types.PackageSolver {
	return NewResolver(t, installed, definitiondb, solverdb, &Explainer{})
}

func NewSolverFromOptions(t types.LuetSolverOptions) types.PackageResolver {
	switch t.Type {
	case QLearningResolverType:
		if t.LearnRate != 0.0 {
			return NewQLearningResolver(t.LearnRate, t.Discount, t.MaxAttempts, 999999)

		}
		return SimpleQLearningSolver()
	}

	return &Explainer{}

}

// NewResolver accepts as argument two lists of packages, the first is the initial set,
// the second represent all the known packages.
// Using constructors as in the future we foresee warmups for hot-restore solver cache
func NewResolver(t types.SolverOptions, installed types.PackageDatabase, definitiondb types.PackageDatabase, solverdb types.PackageDatabase, re types.PackageResolver) types.PackageSolver {
	var s types.PackageSolver
	switch t.Type {
	default:
		s = &Solver{InstalledDatabase: installed, DefinitionDatabase: definitiondb, SolverDatabase: solverdb, Resolver: re}
	}

	return s
}

// SetDefinitionDatabase is a setter for the definition Database

func (s *Solver) SetDefinitionDatabase(db types.PackageDatabase) {
	s.DefinitionDatabase = db
}

// SetResolver is a setter for the unsat resolver backend
func (s *Solver) SetResolver(r types.PackageResolver) {
	s.Resolver = r
}

func (s *Solver) World() types.Packages {
	return s.DefinitionDatabase.World()
}

func (s *Solver) Installed() types.Packages {

	return s.InstalledDatabase.World()
}

func (s *Solver) noRulesWorld() bool {
	for _, p := range s.World() {
		if len(p.GetConflicts()) != 0 || len(p.GetRequires()) != 0 {
			return false
		}
	}

	return true
}

func (s *Solver) noRulesInstalled() bool {
	for _, p := range s.Installed() {
		if len(p.GetConflicts()) != 0 || len(p.GetRequires()) != 0 {
			return false
		}
	}

	return true
}

func (s *Solver) BuildInstalled() (bf.Formula, error) {
	var formulas []bf.Formula
	var packages types.Packages
	for _, p := range s.Installed() {
		packages = append(packages, p)
		for _, dep := range p.Related(s.InstalledDatabase) {
			packages = append(packages, dep)
		}
	}

	for _, p := range packages {
		solvable, err := p.BuildFormula(s.InstalledDatabase, s.SolverDatabase)
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, solvable)
		formulas = append(formulas, solvable...)

	}
	return bf.And(formulas...), nil

}

// BuildWorld builds the formula which olds the requirements from the package definitions
// which are available (global state)
func (s *Solver) BuildWorld(includeInstalled bool) (bf.Formula, error) {
	var formulas []bf.Formula
	// NOTE: This block should be enabled in case of very old systems with outdated world sets
	if includeInstalled {
		solvable, err := s.BuildInstalled()
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, solvable)
		formulas = append(formulas, solvable)
	}

	for _, p := range s.World() {
		solvable, err := p.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, solvable...)
	}
	return bf.And(formulas...), nil
}

// BuildWorld builds the formula which olds the requirements from the package definitions
// which are available (global state)
func (s *Solver) BuildPartialWorld(includeInstalled bool) (bf.Formula, error) {
	var formulas []bf.Formula
	// NOTE: This block shouldf be enabled in case of very old systems with outdated world sets
	if includeInstalled {
		solvable, err := s.BuildInstalled()
		if err != nil {
			return nil, err
		}
		//f = bf.And(f, solvable)
		formulas = append(formulas, solvable)
	}

	var packages types.Packages
	for _, p := range s.Wanted {
		//	packages = append(packages, p)
		for _, dep := range p.Related(s.DefinitionDatabase) {
			packages = append(packages, dep)
		}

	}

	for _, p := range packages {
		solvable, err := p.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, solvable...)
	}

	if len(formulas) != 0 {
		return bf.And(formulas...), nil
	}

	return bf.True, nil
}

func (s *Solver) getList(db types.PackageDatabase, lsp types.Packages) (types.Packages, error) {
	var ls types.Packages

	for _, pp := range lsp {
		cp, err := db.FindPackage(pp)
		if err != nil {
			packages, err := pp.Expand(db)
			// Expand, and relax search - if not found pick the same one
			if err != nil || len(packages) == 0 {
				cp = pp
			} else {
				cp = packages.Best(nil)
			}
		}
		ls = append(ls, cp)
	}
	return ls, nil
}

// Conflicts acts like ConflictsWith, but uses package's reverse dependencies to
// determine if it conflicts with the given set
func (s *Solver) Conflicts(pack *types.Package, lsp types.Packages) (bool, error) {
	p, err := s.DefinitionDatabase.FindPackage(pack)
	if err != nil {
		p = pack
	}

	ls, err := s.getList(s.DefinitionDatabase, lsp)
	if err != nil {
		return false, errors.Wrap(err, "Package not found in definition db")
	}

	if s.noRulesWorld() {
		return false, nil
	}

	temporarySet := pkg.NewInMemoryDatabase(false)
	for _, p := range ls {
		temporarySet.CreatePackage(p)
	}

	revdeps, err := temporarySet.GetRevdeps(p)
	if err != nil {
		return false, errors.Wrap(err, "error scanning revdeps")
	}

	var revdepsErr error
	for _, r := range revdeps {
		if revdepsErr == nil {
			revdepsErr = errors.New("")
		}
		revdepsErr = fmt.Errorf("%s\n%s", revdepsErr.Error(), r.HumanReadableString())
	}

	return len(revdeps) != 0, revdepsErr
}

// ConflictsWith return true if a package is part of the requirement set of a list of package
// return false otherwise (and thus it is NOT relevant to the given list)
func (s *Solver) ConflictsWith(pack *types.Package, lsp types.Packages) (bool, error) {
	p, err := s.DefinitionDatabase.FindPackage(pack)
	if err != nil {
		p = pack //Relax search, otherwise we cannot compute solutions for packages not in definitions
		//	return false, errors.Wrap(err, "Package not found in definition db")
	}

	ls, err := s.getList(s.DefinitionDatabase, lsp)
	if err != nil {
		return false, errors.Wrap(err, "Package not found in definition db")
	}

	var formulas []bf.Formula

	if s.noRulesWorld() {
		return false, nil
	}

	encodedP, err := p.Encode(s.SolverDatabase)
	if err != nil {
		return false, err
	}
	P := bf.Var(encodedP)

	r, err := s.BuildWorld(false)
	if err != nil {
		return false, err
	}
	formulas = append(formulas, bf.And(bf.Not(P), r))

	for _, i := range ls {
		if i.Matches(p) {
			continue
		}
		// XXX: Skip check on any of its requires ?  ( Drop to avoid removing system packages when selecting an uninstall)
		// if i.RequiresContains(p) {
		// 	fmt.Println("Requires found")
		// 	continue
		// }

		encodedI, err := i.Encode(s.SolverDatabase)
		if err != nil {
			return false, err
		}
		I := bf.Var(encodedI)
		formulas = append(formulas, bf.And(I, r))
	}
	model := bf.Solve(bf.And(formulas...))
	if model == nil {
		return true, nil
	}

	return false, nil

}

func (s *Solver) ConflictsWithInstalled(p *types.Package) (bool, error) {
	return s.ConflictsWith(p, s.Installed())
}

// UninstallUniverse takes a list of candidate package and return a list of packages that would be removed
// in order to purge the candidate. Uses the solver to check constraints and nothing else
//
// It can be compared to the counterpart Uninstall as this method acts like a uninstall --full
// it removes all the packages and its deps. taking also in consideration other packages that might have
// revdeps
func (s *Solver) UninstallUniverse(toremove types.Packages) (types.Packages, error) {

	if s.noRulesInstalled() {
		return s.getList(s.InstalledDatabase, toremove)
	}

	// resolve to packages from the db
	toRemove, err := s.getList(s.InstalledDatabase, toremove)
	if err != nil {
		return nil, errors.Wrap(err, "Package not found in definition db")
	}

	var formulas []bf.Formula
	r, err := s.BuildInstalled()
	if err != nil {
		return nil, errors.Wrap(err, "Package not found in definition db")
	}

	// SAT encode the clauses against the world
	for _, p := range toRemove.Unique() {
		encodedP, err := p.Encode(s.InstalledDatabase)
		if err != nil {
			return nil, errors.Wrap(err, "Package not found in definition db")
		}
		P := bf.Var(encodedP)
		formulas = append(formulas, bf.And(bf.Not(P), r))
	}

	markedForRemoval := types.Packages{}
	model := bf.Solve(bf.And(formulas...))
	if model == nil {
		return nil, errors.New("Failed finding a solution")
	}
	assertion, err := DecodeModel(model, s.InstalledDatabase)
	if err != nil {
		return nil, errors.Wrap(err, "while decoding model from solution")
	}
	for _, a := range assertion {
		if !a.Value {
			if p, err := s.InstalledDatabase.FindPackage(a.Package); err == nil {
				markedForRemoval = append(markedForRemoval, p)
			}

		}
	}
	return markedForRemoval, nil
}

// UpgradeUniverse mark packages for removal and returns a solution. It considers
// the Universe db as authoritative
// See also on the subject: https://arxiv.org/pdf/1007.1021.pdf
func (s *Solver) UpgradeUniverse(dropremoved bool) (types.Packages, types.PackagesAssertions, error) {
	// we first figure out which aren't up-to-date
	// which has to be removed
	// and which needs to be upgraded
	notUptodate := types.Packages{}
	removed := types.Packages{}
	toUpgrade := types.Packages{}
	replacements := map[*types.Package]*types.Package{}

	// TODO: this is memory expensive, we need to optimize this
	universe, err := s.DefinitionDatabase.Copy()
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed copying db")
	}

	for _, p := range s.Installed() {
		universe.CreatePackage(p)
	}

	// Grab all the installed ones, see if they are eligible for update
	for _, p := range s.Installed() {
		available, err := s.DefinitionDatabase.FindPackageVersions(p)
		if len(available) == 0 || err != nil {
			removed = append(removed, p)
			continue
		}

		bestmatch := available.Best(nil)
		// Found a better version available
		if !bestmatch.Matches(p) {
			notUptodate = append(notUptodate, p)
			toUpgrade = append(toUpgrade, bestmatch)
			replacements[p] = bestmatch
		}
	}

	var formulas []bf.Formula

	// Build constraints for the whole defdb
	r, err := s.BuildWorld(true)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't build world constraints")
	}

	// Treat removed packages from universe as marked for deletion
	if dropremoved {
		// SAT encode the clauses against the world
		for _, p := range removed.Unique() {
			encodedP, err := p.Encode(universe)
			if err != nil {
				return nil, nil, errors.Wrap(err, "couldn't encode package")
			}
			P := bf.Var(encodedP)
			formulas = append(formulas, bf.And(bf.Not(P), r))
		}
	}

	for old, new := range replacements {
		oldP, err := old.Encode(universe)
		if err != nil {
			return nil, nil, errors.Wrap(err, "couldn't encode package")
		}
		oldencodedP := bf.Var(oldP)
		newP, err := new.Encode(universe)
		if err != nil {
			return nil, nil, errors.Wrap(err, "couldn't encode package")
		}
		newEncodedP := bf.Var(newP)

		//solvable, err := old.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)
		solvablenew, err := new.BuildFormula(s.DefinitionDatabase, s.SolverDatabase)

		formulas = append(formulas, bf.And(bf.Not(oldencodedP), bf.And(append(solvablenew, newEncodedP)...)))
	}

	//formulas = append(formulas, r)

	markedForRemoval := types.Packages{}

	if len(formulas) == 0 {
		return types.Packages{}, types.PackagesAssertions{}, nil
	}
	model := bf.Solve(bf.And(formulas...))
	if model == nil {
		return nil, nil, errors.New("Failed finding a solution")
	}

	assertion, err := DecodeModel(model, universe)
	if err != nil {
		return nil, nil, errors.Wrap(err, "while decoding model from solution")
	}
	for _, a := range assertion {
		if !a.Value {
			if p, err := s.InstalledDatabase.FindPackage(a.Package); err == nil {
				markedForRemoval = append(markedForRemoval, p)
			}

		}

	}
	return markedForRemoval, assertion, nil
}

func inPackage(list []*types.Package, p *types.Package) bool {
	for _, l := range list {
		if l.AtomMatches(p) {
			return true
		}
	}
	return false
}

// Compute upgrade between packages if specified, or all if none is specified
func (s *Solver) computeUpgrade(ppsToUpgrade, ppsToNotUpgrade []*types.Package) func(defDB types.PackageDatabase, installDB types.PackageDatabase) (types.Packages, types.Packages, types.PackageDatabase, []*types.Package) {
	return func(defDB types.PackageDatabase, installDB types.PackageDatabase) (types.Packages, types.Packages, types.PackageDatabase, []*types.Package) {
		toUninstall := types.Packages{}
		toInstall := types.Packages{}

		// we do this in memory so we take into account of provides, and its faster
		universe, _ := defDB.Copy()
		installedcopy := pkg.NewInMemoryDatabase(false)
		for _, p := range installDB.World() {
			installedcopy.CreatePackage(p)
			packages, err := universe.FindPackageVersions(p)

			if err == nil && len(packages) != 0 {
				best := packages.Best(nil)

				// This make sure that we don't try to upgrade something that was specified
				// specifically to not be marked for upgrade
				// At the same time, makes sure that if we mark a package to look for upgrades
				// it doesn't have to be in the blacklist (the packages to NOT upgrade)
				if !best.Matches(p) &&
					((len(ppsToUpgrade) == 0 && len(ppsToNotUpgrade) == 0) ||
						(inPackage(ppsToUpgrade, p) && !inPackage(ppsToNotUpgrade, p)) ||
						(len(ppsToUpgrade) == 0 && !inPackage(ppsToNotUpgrade, p))) {
					toUninstall = append(toUninstall, p)
					toInstall = append(toInstall, best)
				}
			}
		}
		return toUninstall, toInstall, installedcopy, ppsToUpgrade
	}
}

func assertionToMemDB(assertions types.PackagesAssertions) types.PackageDatabase {
	db := pkg.NewInMemoryDatabase(false)
	for _, a := range assertions {
		if a.Value {
			db.CreatePackage(a.Package)
		}
	}
	return db
}

func (s *Solver) upgrade(psToUpgrade, psToNotUpgrade types.Packages, fn func(defDB types.PackageDatabase, installDB types.PackageDatabase) (types.Packages, types.Packages, types.PackageDatabase, []*types.Package), defDB types.PackageDatabase, installDB types.PackageDatabase, checkconflicts, full bool) (types.Packages, types.PackagesAssertions, error) {

	toUninstall, toInstall, installedcopy, packsToUpgrade := fn(defDB, installDB)
	s2 := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, installedcopy, defDB, pkg.NewInMemoryDatabase(false))
	s2.SetResolver(s.Resolver)
	if !full {
		ass := types.PackagesAssertions{}
		for _, i := range toInstall {
			ass = append(ass, types.PackageAssert{Package: i, Value: true})
		}
	}
	// Then try to uninstall the versions in the system, and store that tree
	r, err := s.Uninstall(checkconflicts, false, toUninstall.Unique()...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Could not compute upgrade - couldn't uninstall candidates ")
	}
	for _, z := range r {
		err = installedcopy.RemovePackage(z)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Could not compute upgrade - couldn't remove copy of package targetted for removal")
		}
	}

	if len(toInstall) == 0 {
		ass := types.PackagesAssertions{}
		for _, i := range installDB.World() {
			ass = append(ass, types.PackageAssert{Package: i, Value: true})
		}
		return toUninstall, ass, nil
	}
	assertions, err := s2.RelaxedInstall(toInstall.Unique())

	wantedSystem := assertionToMemDB(assertions)

	fn = s.computeUpgrade(types.Packages{}, types.Packages{})
	if len(packsToUpgrade) > 0 {
		// If we have packages in input,
		// compute what we are looking to upgrade.
		// those are assertions minus packsToUpgrade

		var selectedPackages []*types.Package

		for _, p := range assertions {
			if p.Value && !inPackage(psToUpgrade, p.Package) {
				selectedPackages = append(selectedPackages, p.Package)
			}
		}
		fn = s.computeUpgrade(selectedPackages, psToNotUpgrade)
	}

	_, toInstall, _, _ = fn(defDB, wantedSystem)
	if len(toInstall) > 0 {
		_, toInstall, ass := s.upgrade(psToUpgrade, psToNotUpgrade, fn, defDB, wantedSystem, checkconflicts, full)
		return toUninstall, toInstall, ass
	}
	return toUninstall, assertions, err
}

func (s *Solver) Upgrade(checkconflicts, full bool) (types.Packages, types.PackagesAssertions, error) {

	installedcopy := pkg.NewInMemoryDatabase(false)
	err := s.InstalledDatabase.Clone(installedcopy)
	if err != nil {
		return nil, nil, err
	}
	return s.upgrade(types.Packages{}, types.Packages{}, s.computeUpgrade(types.Packages{}, types.Packages{}), s.DefinitionDatabase, installedcopy, checkconflicts, full)
}

// Uninstall takes a candidate package and return a list of packages that would be removed
// in order to purge the candidate. Returns error if unsat.
func (s *Solver) Uninstall(checkconflicts, full bool, packs ...*types.Package) (types.Packages, error) {
	if len(packs) == 0 {
		return types.Packages{}, nil
	}
	var res types.Packages

	toRemove := types.Packages{}

	for _, c := range packs {
		candidate, err := s.InstalledDatabase.FindPackage(c)
		if err != nil {

			//	return nil, errors.Wrap(err, "Couldn't find required package in db definition")
			packages, err := c.Expand(s.InstalledDatabase)
			//	Info("Expanded", packages, err)
			if err != nil || len(packages) == 0 {
				candidate = c
			} else {
				candidate = packages.Best(nil)
			}
			//Relax search, otherwise we cannot compute solutions for packages not in definitions
			//	return nil, errors.Wrap(err, "Package not found between installed")
		}

		toRemove = append(toRemove, candidate)
	}

	// Build a fake "Installed" - Candidate and its requires tree
	var InstalledMinusCandidate types.Packages

	// We are asked to not perform a full uninstall (checking all the possible requires that could
	// be removed). Let's only check if we can remove the selected package
	if !full && checkconflicts {
		for _, candidate := range toRemove {
			if conflicts, err := s.Conflicts(candidate, s.Installed()); conflicts {
				return nil, errors.Wrap(err, "while searching for "+candidate.HumanReadableString()+" conflicts")
			}
		}
		return toRemove, nil
	}

	// TODO: Can be optimized
	for _, i := range s.Installed() {
		matched := false
		for _, candidate := range toRemove {
			if !i.Matches(candidate) {
				contains, err := candidate.RequiresContains(s.SolverDatabase, i)
				if err != nil {
					return nil, errors.Wrap(err, "Failed getting installed list")
				}
				if !contains {
					matched = true
				}

			}
		}
		if matched {
			InstalledMinusCandidate = append(InstalledMinusCandidate, i)
		}
	}

	s2 := NewSolver(types.SolverOptions{Type: types.SolverSingleCoreSimple}, pkg.NewInMemoryDatabase(false), s.InstalledDatabase, pkg.NewInMemoryDatabase(false))
	s2.SetResolver(s.Resolver)

	// Get the requirements to install the candidate
	asserts, err := s2.RelaxedInstall(toRemove)
	if err != nil {
		return nil, err
	}
	for _, a := range asserts {
		if a.Value {
			if !checkconflicts {
				res = append(res, a.Package)
				continue
			}

			c, err := s.ConflictsWithInstalled(a.Package)
			if err != nil {
				return nil, err
			}

			// If doesn't conflict with installed we just consider it for removal and look for the next one
			if !c {
				res = append(res, a.Package)
				continue
			}

			// If does conflicts, give it another chance by checking conflicts if in case we didn't installed our candidate and all the required packages in the system
			c, err = s.ConflictsWith(a.Package, InstalledMinusCandidate)
			if err != nil {
				return nil, err
			}
			if !c {
				res = append(res, a.Package)
			}

		}

	}

	return res, nil
}

// BuildFormula builds the main solving formula that is evaluated by the sat solver.
func (s *Solver) BuildFormula() (bf.Formula, error) {
	var formulas []bf.Formula
	r, err := s.BuildWorld(false)
	if err != nil {
		return nil, err
	}

	for _, wanted := range s.Wanted {

		encodedW, err := wanted.Encode(s.SolverDatabase)
		if err != nil {
			return nil, err
		}
		W := bf.Var(encodedW)
		//	allW = append(allW, W)
		installedWorld := s.Installed()
		//TODO:Optimize
		if len(installedWorld) == 0 {
			formulas = append(formulas, W) //bf.And(bf.True, W))
			continue
		}

		for _, installed := range installedWorld {
			encodedI, err := installed.Encode(s.SolverDatabase)
			if err != nil {
				return nil, err
			}
			I := bf.Var(encodedI)
			formulas = append(formulas, bf.And(W, I))
		}

	}

	formulas = append(formulas, r)
	return bf.And(formulas...), nil
}

func (s *Solver) solve(f bf.Formula) (map[string]bool, bf.Formula, error) {
	model := bf.Solve(f)
	if model == nil {
		return model, f, errors.New("Unsolvable")
	}

	return model, f, nil
}

// Solve builds the formula given the current state and returns package assertions
func (s *Solver) Solve() (types.PackagesAssertions, error) {
	var model map[string]bool
	var err error

	f, err := s.BuildFormula()

	if err != nil {
		return nil, err
	}

	model, _, err = s.solve(f)
	if err != nil && s.Resolver != nil {
		return s.Resolver.Solve(f, s)
	}

	if err != nil {
		return nil, err
	}

	return DecodeModel(model, s.SolverDatabase)
}

// Install given a list of packages, returns package assertions to indicate the packages that must be installed in the system in order
// to statisfy all the constraints
func (s *Solver) RelaxedInstall(c types.Packages) (types.PackagesAssertions, error) {

	coll, err := s.getList(s.DefinitionDatabase, c)
	if err != nil {
		return nil, errors.Wrap(err, "Packages not found in definition db")
	}

	s.Wanted = coll

	if s.noRulesWorld() {
		var ass types.PackagesAssertions
		for _, p := range s.Installed() {
			ass = append(ass, types.PackageAssert{Package: p, Value: true})

		}
		for _, p := range s.Wanted {
			ass = append(ass, types.PackageAssert{Package: p, Value: true})
		}
		return ass, nil
	}
	assertions, err := s.Solve()
	if err != nil {
		return nil, err
	}

	return assertions, nil
}

// Install returns the assertions necessary in order to install the packages in
// a system.
// It calculates the best result possible, trying to maximize new packages.
func (s *Solver) Install(c types.Packages) (types.PackagesAssertions, error) {
	assertions, err := s.RelaxedInstall(c)
	if err != nil {
		return nil, err
	}

	systemAfterInstall := pkg.NewInMemoryDatabase(false)

	toUpgrade := types.Packages{}
	toNotUpgrade := types.Packages{}
	for _, p := range c {
		if p.GetVersion() == ">=0" || p.GetVersion() == ">0" {
			toUpgrade = append(toUpgrade, p)
		} else {
			toNotUpgrade = append(toNotUpgrade, p)
		}
	}
	for _, p := range assertions {
		if p.Value {
			systemAfterInstall.CreatePackage(p.Package)
			if !inPackage(c, p.Package) && !inPackage(toUpgrade, p.Package) && !inPackage(toNotUpgrade, p.Package) {
				toUpgrade = append(toUpgrade, p.Package)
			}
		}
	}

	if len(toUpgrade) == 0 {
		return assertions, nil
	}

	toUninstall, _, _, _ := s.computeUpgrade(toUpgrade, toNotUpgrade)(s.DefinitionDatabase, systemAfterInstall)
	if len(toUninstall) > 0 {
		// do partial upgrade based on input.
		// IF there is no version specified in the input, or >=0 is specified,
		// then compute upgrade for those
		_, newassertions, err := s.upgrade(toUpgrade, toNotUpgrade, s.computeUpgrade(toUpgrade, toNotUpgrade), s.DefinitionDatabase, systemAfterInstall, false, false)
		if err != nil {
			// TODO: Emit warning.
			// We were not able to compute upgrades (maybe for some pinned packages, or a conflict)
			// so we return the relaxed result
			return assertions, nil
		}

		// Protect if we return no assertion at all
		if len(newassertions) == 0 && len(assertions) > 0 {
			return assertions, nil
		}
		return newassertions, nil
	}

	return assertions, nil
}
