// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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
	"github.com/crillab/gophersat/bf"
)

type SolverType int

const (
	SolverSingleCoreSimple SolverType = 0
)

// PackageSolver is an interface to a generic package solving algorithm
type PackageSolver interface {
	SetDefinitionDatabase(PackageDatabase)
	Install(p Packages) (PackagesAssertions, error)
	RelaxedInstall(p Packages) (PackagesAssertions, error)

	Uninstall(checkconflicts, full bool, candidate ...*Package) (Packages, error)
	ConflictsWithInstalled(p *Package) (bool, error)
	ConflictsWith(p *Package, ls Packages) (bool, error)
	Conflicts(pack *Package, lsp Packages) (bool, error)

	World() Packages
	Upgrade(checkconflicts, full bool) (Packages, PackagesAssertions, error)

	UpgradeUniverse(dropremoved bool) (Packages, PackagesAssertions, error)
	UninstallUniverse(toremove Packages) (Packages, error)

	SetResolver(PackageResolver)

	Solve() (PackagesAssertions, error)
	//	BestInstall(c Packages) (PackagesAssertions, error)
}

type SolverOptions struct {
	Type        SolverType `yaml:"type,omitempty"`
	Concurrency int        `yaml:"concurrency,omitempty"`
}

// PackageResolver assists PackageSolver on unsat cases
type PackageResolver interface {
	Solve(bf.Formula, PackageSolver) (PackagesAssertions, error)
}

type PackagesAssertions []PackageAssert

type PackageHash struct {
	BuildHash   string
	PackageHash string
}
