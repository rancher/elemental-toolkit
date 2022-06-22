// Copyright © 2019-2022 Ettore Di Giacinto <mudler@gentoo.org>
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

package installer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/luet/pkg/api/core/config"
	"github.com/mudler/luet/pkg/api/core/logger"
	"github.com/mudler/luet/pkg/helpers"
	"github.com/mudler/luet/pkg/tree"

	"github.com/mudler/luet/pkg/api/core/bus"
	"github.com/mudler/luet/pkg/api/core/types"
	artifact "github.com/mudler/luet/pkg/api/core/types/artifact"
	pkg "github.com/mudler/luet/pkg/database"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	"github.com/mudler/luet/pkg/solver"
	"github.com/pterm/pterm"

	"github.com/pkg/errors"
)

type LuetInstallerOptions struct {
	SolverOptions                                                  types.LuetSolverOptions
	Concurrency                                                    int
	NoDeps                                                         bool
	OnlyDeps                                                       bool
	Force                                                          bool
	PreserveSystemEssentialData                                    bool
	FullUninstall, FullCleanUninstall                              bool
	CheckConflicts                                                 bool
	SolverUpgrade, RemoveUnavailableOnUpgrade, UpgradeNewRevisions bool
	Ask                                                            bool
	DownloadOnly                                                   bool
	Relaxed                                                        bool
	PackageRepositories                                            types.LuetRepositories
	AutoOSCheck                                                    bool

	Context types.Context
}

type LuetInstaller struct {
	Options LuetInstallerOptions
}

type ArtifactMatch struct {
	Package    *types.Package
	Artifact   *artifact.PackageArtifact
	Repository Repository
}

func NewLuetInstaller(opts LuetInstallerOptions) *LuetInstaller {
	return &LuetInstaller{Options: opts}
}

// computeUpgrade returns the packages to be uninstalled and installed in a system to perform an upgrade
// based on the system repositories
func (l *LuetInstaller) computeUpgrade(syncedRepos Repositories, s *System) (types.Packages, types.Packages, error) {
	toInstall := types.Packages{}
	var uninstall types.Packages
	var err error
	// First match packages against repositories by priority
	allRepos := pkg.NewInMemoryDatabase(false)
	syncedRepos.SyncDatabase(allRepos)
	// compute a "big" world
	solv := solver.NewResolver(
		types.SolverOptions{
			Type:        l.Options.SolverOptions.Implementation,
			Concurrency: l.Options.Concurrency},
		s.Database, allRepos, pkg.NewInMemoryDatabase(false),
		solver.NewSolverFromOptions(l.Options.SolverOptions))
	var solution types.PackagesAssertions

	if l.Options.SolverUpgrade {
		uninstall, solution, err = solv.UpgradeUniverse(l.Options.RemoveUnavailableOnUpgrade)
		if err != nil {
			return uninstall, toInstall, errors.Wrap(err, "Failed solving solution for upgrade")
		}
	} else {
		uninstall, solution, err = solv.Upgrade(l.Options.FullUninstall, true)
		if err != nil {
			return uninstall, toInstall, errors.Wrap(err, "Failed solving solution for upgrade")
		}
	}

	for _, assertion := range solution {
		// Be sure to filter from solutions packages already installed in the system
		if _, err := s.Database.FindPackage(assertion.Package); err != nil && assertion.Value {
			toInstall = append(toInstall, assertion.Package)
		}
	}

	if l.Options.UpgradeNewRevisions {
		for _, p := range s.Database.World() {
			matches := syncedRepos.PackageMatches(types.Packages{p})
			if len(matches) == 0 {
				// Package missing. the user should run luet upgrade --universe
				continue
			}
			for _, artefact := range matches[0].Repo.GetIndex() {
				if artefact.CompileSpec.GetPackage() == nil {
					return uninstall, toInstall, errors.New("Package in compilespec empty")

				}
				if artefact.CompileSpec.GetPackage().Matches(p) && artefact.CompileSpec.GetPackage().GetBuildTimestamp() != p.GetBuildTimestamp() {
					toInstall = append(toInstall, matches[0].Package).Unique()
					uninstall = append(uninstall, p).Unique()
				}
			}
		}
	}

	return uninstall, toInstall, nil
}

// Upgrade upgrades a System based on the Installer options. Returns error in case of failure
func (l *LuetInstaller) Upgrade(s *System) error {
	l.Options.Context.Screen("Upgrade")
	syncedRepos, err := l.SyncRepositories()
	if err != nil {
		return err
	}

	l.Options.Context.Info(":thinking: Computing upgrade, please hang tight... :zzz:")
	if l.Options.UpgradeNewRevisions {
		l.Options.Context.Info(":memo: note: will consider new build revisions while upgrading")
	}

	return l.checkAndUpgrade(syncedRepos, s)
}

func (l *LuetInstaller) SyncRepositories() (Repositories, error) {
	l.Options.Context.Spinner()
	defer l.Options.Context.SpinnerStop()

	var errs error
	syncedRepos := Repositories{}

	for _, r := range SystemRepositories(l.Options.PackageRepositories) {
		repo, err := r.Sync(l.Options.Context, false)
		if err == nil {
			syncedRepos = append(syncedRepos, repo)
		} else {
			multierror.Append(errs, fmt.Errorf("failed syncing '%s': %w", r.Name, err))
		}
	}

	// compute what to install and from where
	sort.Sort(syncedRepos)

	return syncedRepos, errs
}

func (l *LuetInstaller) Swap(toRemove types.Packages, toInstall types.Packages, s *System) error {
	syncedRepos, err := l.SyncRepositories()
	if err != nil {
		return err
	}

	toRemoveFinal := types.Packages{}
	for _, p := range toRemove {
		packs, _ := s.Database.FindPackages(p)
		if len(packs) == 0 {
			return errors.New("Package " + p.HumanReadableString() + " not found in the system")
		}
		for _, pp := range packs {
			toRemoveFinal = append(toRemoveFinal, pp)
		}
	}
	o := Option{
		FullUninstall:      false,
		Force:              true,
		CheckConflicts:     false,
		FullCleanUninstall: false,
		NoDeps:             l.Options.NoDeps,
		OnlyDeps:           false,
	}

	return l.swap(o, syncedRepos, toRemoveFinal, toInstall, s)
}

func (l *LuetInstaller) computeSwap(o Option, syncedRepos Repositories, toRemove types.Packages, toInstall types.Packages, s *System) (map[string]ArtifactMatch, types.Packages, types.PackagesAssertions, types.PackageDatabase, error) {

	allRepos := pkg.NewInMemoryDatabase(false)
	syncedRepos.SyncDatabase(allRepos)

	toInstall = syncedRepos.ResolveSelectors(toInstall)

	// First check what would have been done
	installedtmp, err := s.Database.Copy()
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "Failed create temporary in-memory db")
	}

	systemAfterChanges := &System{Database: installedtmp}

	packs, err := l.computeUninstall(o, systemAfterChanges, toRemove...)
	if err != nil && !o.Force {
		l.Options.Context.Error("Failed computing uninstall for ", packsToList(toRemove))
		return nil, nil, nil, nil, errors.Wrap(err, "computing uninstall "+packsToList(toRemove))
	}
	for _, p := range packs {
		err = systemAfterChanges.Database.RemovePackage(p)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed removing package from database")
		}
	}

	match, packages, assertions, allRepos, err := l.computeInstall(o, syncedRepos, toInstall, systemAfterChanges)
	for _, p := range toInstall {
		assertions = append(assertions, types.PackageAssert{Package: p, Value: true})
	}
	return match, packages, assertions, allRepos, err
}

func (l *LuetInstaller) swap(o Option, syncedRepos Repositories, toRemove types.Packages, toInstall types.Packages, s *System) error {

	match, packages, assertions, allRepos, err := l.computeSwap(o, syncedRepos, toRemove, toInstall, s)
	if err != nil {
		return errors.Wrap(err, "failed computing package replacement")
	}

	if l.Options.Ask {
		// if len(toRemove) > 0 {
		// 	l.Options.Context.Info(":recycle: Packages that are going to be removed from the system:\n ", Yellow(packsToList(toRemove)).BgBlack().String())
		// }

		// if len(match) > 0 {
		// 	l.Options.Context.Info("Packages that are going to be installed in the system:")
		// 	//	l.Options.Context.Info("Packages that are going to be installed in the system: \n ", Green(matchesToList(match)).BgBlack().String())
		// 	printMatches(match)
		// }
		l.Options.Context.Info(":zap: Proposed version changes to the system:\n ")
		printMatchUpgrade(match, toRemove)

		l.Options.Context.Info("By going forward, you are also accepting the licenses of the packages that you are going to install in your system.")
		if l.Options.Context.Ask() {
			l.Options.Ask = false // Don't prompt anymore
		} else {
			return errors.New("Aborted by user")
		}
	}

	// First match packages against repositories by priority
	if err := l.download(syncedRepos, match); err != nil {
		return errors.Wrap(err, "Pre-downloading packages")
	}

	if err := l.checkFileconflicts(match, false, s); err != nil {
		if !l.Options.Force {
			return errors.Wrap(err, "file conflict found")
		} else {
			l.Options.Context.Warning("file conflict found", err.Error())
		}
	}

	if l.Options.DownloadOnly {
		return nil
	}

	ops, err := l.generateRunOps(toRemove, match, Option{
		Force:              o.Force,
		NoDeps:             false,
		OnlyDeps:           o.OnlyDeps,
		RunFinalizers:      false,
		CheckFileConflicts: false,
	}, o, syncedRepos, packages, assertions, allRepos, s)
	if err != nil {
		return errors.Wrap(err, "failed computing installer options")
	}

	err = l.runOps(ops, s)
	if err != nil {
		return errors.Wrap(err, "failed running installer options")
	}

	toFinalize, err := l.getFinalizers(allRepos, assertions, match, o.NoDeps)
	if err != nil {
		return errors.Wrap(err, "failed getting package to finalize")
	}

	return s.ExecuteFinalizers(l.Options.Context, toFinalize)
}

type Option struct {
	Force              bool
	NoDeps             bool
	CheckConflicts     bool
	FullUninstall      bool
	FullCleanUninstall bool
	OnlyDeps           bool
	RunFinalizers      bool

	CheckFileConflicts bool
}

type operation struct {
	Option  Option
	Package *types.Package
}

type installOperation struct {
	operation
	Reposiories Repositories
	Packages    types.Packages
	Assertions  types.PackagesAssertions
	Database    types.PackageDatabase
	Matches     map[string]ArtifactMatch
}

// installerOp is the operation that is sent to the
// upgradeWorker's channel (todo)
type installerOp struct {
	Uninstall []operation
	Install   []installOperation
}

func (l *LuetInstaller) runOps(ops []installerOp, s *System) error {
	all := make(chan installerOp)

	wg := new(sync.WaitGroup)
	systemLock := &sync.Mutex{}

	// Do the real install
	for i := 0; i < l.Options.Concurrency; i++ {
		wg.Add(1)
		go l.installerOpWorker(i, wg, systemLock, all, s)
	}

	for _, c := range ops {
		all <- c
	}
	close(all)
	wg.Wait()

	return nil
}

// TODO: use installerOpWorker in place of all the other workers.
// This one is general enough to read a list of operations and execute them.
func (l *LuetInstaller) installerOpWorker(i int, wg *sync.WaitGroup, systemLock *sync.Mutex, c <-chan installerOp, s *System) error {
	defer wg.Done()

	for p := range c {

		installedFiles := map[string]interface{}{}
		for _, pp := range p.Install {
			artMatch := pp.Matches[pp.Package.GetFingerPrint()]
			art, err := l.getPackage(artMatch, l.Options.Context)
			if err != nil {
				installedFiles = map[string]interface{}{}
				break
			}

			l, err := art.FileList()
			if err != nil {
				installedFiles = map[string]interface{}{}
				break
			}
			for _, f := range l {
				installedFiles[f] = nil
			}
		}

		for _, pp := range p.Uninstall {

			l.Options.Context.Debug("Replacing package inplace")
			toUninstall, uninstall, err := l.generateUninstallFn(pp.Option, s, installedFiles, pp.Package)
			if err != nil {
				l.Options.Context.Debug("Skipping uninstall, fail to generate uninstall function, error: " + err.Error())
				continue
			}
			systemLock.Lock()
			err = uninstall()
			systemLock.Unlock()

			if err != nil {
				l.Options.Context.Error("Failed uninstall for ", packsToList(toUninstall))
				continue
			}
		}
		for _, pp := range p.Install {
			artMatch := pp.Matches[pp.Package.GetFingerPrint()]
			ass := pp.Assertions.Search(pp.Package.GetFingerPrint())
			packageToInstall, _ := pp.Packages.Find(pp.Package.GetPackageName())

			systemLock.Lock()
			err := l.install(
				pp.Option,
				pp.Reposiories,
				map[string]ArtifactMatch{pp.Package.GetFingerPrint(): artMatch},
				types.Packages{packageToInstall},
				types.PackagesAssertions{*ass},
				pp.Database,
				s,
			)
			systemLock.Unlock()
			if err != nil {
				l.Options.Context.Error(err)
			}
		}
	}

	return nil
}

// checks wheter we can uninstall and install in place and compose installer worker ops
func (l *LuetInstaller) generateRunOps(
	toUninstall types.Packages, installMatch map[string]ArtifactMatch, installOpt, uninstallOpt Option,
	syncedRepos Repositories, toInstall types.Packages, solution types.PackagesAssertions, allRepos types.PackageDatabase, s *System) (resOps []installerOp, err error) {

	uOpts := []operation{}
	for _, u := range toUninstall {
		uOpts = append(uOpts, operation{Package: u, Option: uninstallOpt})
	}
	iOpts := []installOperation{}
	for _, u := range installMatch {
		iOpts = append(iOpts, installOperation{
			operation: operation{
				Package: u.Package,
				Option:  installOpt,
			},
			Matches:     installMatch,
			Packages:    toInstall,
			Reposiories: syncedRepos,
			Assertions:  solution,
			Database:    allRepos,
		})
	}
	resOps = append(resOps, installerOp{
		Uninstall: uOpts,
		Install:   iOpts,
	})

	return resOps, nil
}

func (l *LuetInstaller) checkAndUpgrade(r Repositories, s *System) error {
	uninstall, toInstall, err := l.computeUpgrade(r, s)
	if err != nil {
		return errors.Wrap(err, "failed computing upgrade")
	}

	if len(toInstall) == 0 && len(uninstall) == 0 {
		l.Options.Context.Info("Nothing to upgrade")
		return nil
	} else {
		l.Options.Context.Info(":zap: Proposed version changes to the system:\n ")
		printUpgradeList(toInstall, uninstall)
	}

	// We don't want any conflict with the installed to raise during the upgrade.
	// In this way we both force uninstalls and we avoid to check with conflicts
	// against the current system state which is pending to deletion
	// E.g. you can't check for conflicts for an upgrade of a new version of A
	// if the old A results installed in the system. This is due to the fact that
	// now the solver enforces the constraints and explictly denies two packages
	// of the same version installed.
	o := Option{
		FullUninstall:      false,
		Force:              true,
		CheckConflicts:     false,
		FullCleanUninstall: false,
		NoDeps:             true,
		OnlyDeps:           false,
	}

	if l.Options.Ask {
		l.Options.Context.Info("By going forward, you are also accepting the licenses of the packages that you are going to install in your system.")
		if l.Options.Context.Ask() {
			l.Options.Ask = false // Don't prompt anymore
			return l.swap(o, r, uninstall, toInstall, s)
		} else {
			return errors.New("Aborted by user")
		}
	}

	bus.Manager.Publish(bus.EventPreUpgrade, struct{ Uninstall, Install types.Packages }{Uninstall: uninstall, Install: toInstall})

	err = l.swap(o, r, uninstall, toInstall, s)

	bus.Manager.Publish(bus.EventPostUpgrade, struct {
		Error              error
		Uninstall, Install types.Packages
	}{Uninstall: uninstall, Install: toInstall, Error: err})

	if err != nil {
		return err
	}

	if l.Options.AutoOSCheck {
		l.Options.Context.Info("Performing automatic oscheck")
		packs := s.OSCheck(l.Options.Context)
		if len(packs) > 0 {
			p := ""
			for _, r := range packs {
				p += " " + r.HumanReadableString()
			}
			l.Options.Context.Info("Following packages requires reinstallation: " + p)
			return l.swap(o, r, packs, packs, s)
		}
		l.Options.Context.Info("OSCheck done")
	}

	return err
}

func (l *LuetInstaller) Install(cp types.Packages, s *System) error {
	l.Options.Context.Screen("Install")
	syncedRepos, err := l.SyncRepositories()
	if err != nil {
		return err
	}

	if len(s.Database.World()) > 0 && !l.Options.Relaxed {
		l.Options.Context.Info(":thinking: Checking for available upgrades")
		if err := l.checkAndUpgrade(syncedRepos, s); err != nil {
			return errors.Wrap(err, "while checking upgrades before install")
		}
	}

	o := Option{
		NoDeps:             l.Options.NoDeps,
		Force:              l.Options.Force,
		OnlyDeps:           l.Options.OnlyDeps,
		CheckFileConflicts: true,
		RunFinalizers:      true,
	}
	match, packages, assertions, allRepos, err := l.computeInstall(o, syncedRepos, cp, s)
	if err != nil {
		return err
	}

	allInstalled := true

	// Resolvers might decide to remove some packages from being installed
	if !solver.IsRelaxedResolver(l.Options.SolverOptions) {
		for _, p := range cp {
			found := false

			if p.HasVersionDefined() {
				f, err := s.Database.FindPackage(p)
				if f != nil && err == nil {
					found = true
					continue
				}
			} else {
				vers, _ := s.Database.FindPackageVersions(p) // If was installed, it is found, as it was filtered
				if len(vers) >= 1 {
					found = true
					continue
				}
			}

			allInstalled = false
			for _, m := range match {
				if m.Package.GetName() == p.GetName() {
					found = true
				}
				for _, pack := range m.Package.GetProvides() {
					if pack.GetName() == p.GetName() {
						found = true
					}
				}
			}

			if !found {
				return fmt.Errorf("package '%s' not found", p.HumanReadableString())
			}
		}
	}

	// Check if we have to process something, or return to the user an error
	if len(match) == 0 {
		l.Options.Context.Info("No packages to install")
		if !solver.IsRelaxedResolver(l.Options.SolverOptions) && !allInstalled {
			return fmt.Errorf("could not find packages to install from the repositories in the system")
		}
		return nil
	}

	l.Options.Context.Info("Packages that are going to be installed in the system:")

	printMatches(match)

	if l.Options.Ask {
		l.Options.Context.Info("By going forward, you are also accepting the licenses of the packages that you are going to install in your system.")
		if l.Options.Context.Ask() {
			l.Options.Ask = false // Don't prompt anymore
			return l.install(o, syncedRepos, match, packages, assertions, allRepos, s)
		} else {
			return errors.New("Aborted by user")
		}
	}
	return l.install(o, syncedRepos, match, packages, assertions, allRepos, s)
}

func (l *LuetInstaller) download(syncedRepos Repositories, toDownload map[string]ArtifactMatch) error {

	// Don't attempt to download stuff that is already in cache
	missArtifacts := false
	for _, m := range toDownload {
		c := m.Repository.Client(l.Options.Context)
		_, err := c.CacheGet(m.Artifact)
		if err != nil {
			missArtifacts = true
		}
	}

	if !missArtifacts {
		l.Options.Context.Debug("Packages already in cache, skipping download")
		return nil
	}

	// Download packages into cache in parallel.
	all := make(chan ArtifactMatch)

	var wg = new(sync.WaitGroup)

	ctx := l.Options.Context.Clone()

	// Check if the terminal is big enough to display a progress bar
	// https://github.com/pterm/pterm/blob/4c725e56bfd9eb38e1c7b9dec187b50b93baa8bd/progressbar_printer.go#L190
	w, _, err := logger.GetTerminalSize()

	var pb *pterm.ProgressbarPrinter
	if logger.IsTerminal() && err == nil && w > 100 {
		area, _ := pterm.DefaultArea.Start()
		pb = pterm.DefaultProgressbar.WithPrintTogether(area).WithTotal(len(toDownload)).WithTitle("Downloading packages")
		pb, _ = pb.Start()
		ctx.SetAnnotation("progressbar", pb)

		defer area.Stop()
	}

	// Download
	for i := 0; i < l.Options.Concurrency; i++ {
		wg.Add(1)
		go l.downloadWorker(i, wg, pb, all, ctx)
	}
	for _, c := range toDownload {
		all <- c
	}
	close(all)
	wg.Wait()
	return nil
}

// Reclaim adds packages to the system database
// if files from artifacts in the repositories are found
// in the system target
func (l *LuetInstaller) Reclaim(s *System) error {
	syncedRepos, err := l.SyncRepositories()
	if err != nil {
		return err
	}

	var toMerge []ArtifactMatch = []ArtifactMatch{}

	for _, repo := range syncedRepos {
		for _, artefact := range repo.GetIndex() {
			l.Options.Context.Debug("Checking if",
				artefact.CompileSpec.GetPackage().HumanReadableString(),
				"from", repo.GetName(), "is installed")
		FILES:
			for _, f := range artefact.Files {
				if fileHelper.Exists(filepath.Join(s.Target, f)) {
					p, err := repo.GetTree().GetDatabase().FindPackage(artefact.CompileSpec.GetPackage())
					if err != nil {
						return err
					}
					l.Options.Context.Info(":mag: Found package:", p.HumanReadableString())
					toMerge = append(toMerge, ArtifactMatch{Artifact: artefact, Package: p})
					break FILES
				}
			}
		}
	}

	for _, match := range toMerge {
		pack := match.Package
		vers, _ := s.Database.FindPackageVersions(pack)

		if len(vers) >= 1 {
			l.Options.Context.Warning("Filtering out package " + pack.HumanReadableString() + ", already reclaimed")
			continue
		}
		_, err := s.Database.CreatePackage(pack)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Failed creating package")
		}
		s.Database.SetPackageFiles(&types.PackageFile{PackageFingerprint: pack.GetFingerPrint(), Files: match.Artifact.Files})
		l.Options.Context.Info(":zap:Reclaimed package:", pack.HumanReadableString())
	}
	l.Options.Context.Info("Done!")

	return nil
}

func (l *LuetInstaller) computeInstall(o Option, syncedRepos Repositories, cp types.Packages, s *System) (map[string]ArtifactMatch, types.Packages, types.PackagesAssertions, types.PackageDatabase, error) {
	var p types.Packages
	toInstall := map[string]ArtifactMatch{}
	allRepos := pkg.NewInMemoryDatabase(false)
	var solution types.PackagesAssertions

	// Check if the package is installed first
	for _, pi := range cp {
		vers, _ := s.Database.FindPackageVersions(pi)

		if len(vers) >= 1 {
			//	l.Options.Context.Warning("Filtering out package " + pi.HumanReadableString() + ", it has other versions already installed. Uninstall one of them first ")
			continue
			//return errors.New("Package " + pi.GetFingerPrint() + " has other versions already installed. Uninstall one of them first: " + strings.Join(vers, " "))

		}
		p = append(p, pi)
	}

	if len(p) == 0 {
		return toInstall, p, solution, allRepos, nil
	}
	// First get metas from all repos (and decodes trees)

	// First match packages against repositories by priority
	//	matches := syncedRepos.PackageMatches(p)

	// compute a "big" world
	syncedRepos.SyncDatabase(allRepos)
	p = syncedRepos.ResolveSelectors(p)
	var packagesToInstall types.Packages
	var err error

	if !o.NoDeps {
		solv := solver.NewResolver(types.SolverOptions{
			Type:        l.Options.SolverOptions.Implementation,
			Concurrency: l.Options.Concurrency},
			s.Database, allRepos, pkg.NewInMemoryDatabase(false),
			solver.NewSolverFromOptions(l.Options.SolverOptions),
		)

		if l.Options.Relaxed {
			solution, err = solv.RelaxedInstall(p)
		} else {
			solution, err = solv.Install(p)
		}
		/// TODO: PackageAssertions needs to be a map[fingerprint]pack so lookup is in O(1)
		if err != nil && !o.Force {
			return toInstall, p, solution, allRepos, errors.Wrap(err, "Failed solving solution for package")
		}
		// Gathers things to install
		for _, assertion := range solution {
			if assertion.Value {
				if _, err := s.Database.FindPackage(assertion.Package); err == nil {
					// skip matching if it is installed already
					continue
				}
				packagesToInstall = append(packagesToInstall, assertion.Package)
			}
		}
	} else if !o.OnlyDeps {
		for _, currentPack := range p {
			if _, err := s.Database.FindPackage(currentPack); err == nil {
				// skip matching if it is installed already
				continue
			}
			packagesToInstall = append(packagesToInstall, currentPack)
		}
	}
	// Gathers things to install
	for _, currentPack := range packagesToInstall {
		// Check if package is already installed.

		matches := syncedRepos.PackageMatches(types.Packages{currentPack})
		if len(matches) == 0 {
			return toInstall, p, solution, allRepos, errors.New("Failed matching solutions against repository for " + currentPack.HumanReadableString() + " where are definitions coming from?!")
		}
	A:
		for _, artefact := range matches[0].Repo.GetIndex() {
			if artefact.CompileSpec.GetPackage() == nil {
				return toInstall, p, solution, allRepos, errors.New("Package in compilespec empty")
			}
			if matches[0].Package.Matches(artefact.CompileSpec.GetPackage()) {
				currentPack.SetBuildTimestamp(artefact.CompileSpec.GetPackage().GetBuildTimestamp())
				// Filter out already installed
				if _, err := s.Database.FindPackage(currentPack); err != nil {
					toInstall[currentPack.GetFingerPrint()] = ArtifactMatch{Package: currentPack, Artifact: artefact, Repository: matches[0].Repo}
				}
				break A
			}
		}
	}
	return toInstall, p, solution, allRepos, nil
}

func (l *LuetInstaller) getFinalizers(allRepos types.PackageDatabase, solution types.PackagesAssertions, toInstall map[string]ArtifactMatch, nodeps bool) ([]*types.Package, error) {
	var toFinalize []*types.Package
	if !nodeps {
		// TODO: Lower those errors as l.Options.Context.Warning
		for _, w := range toInstall {
			if !fileHelper.Exists(w.Package.Rel(tree.FinalizerFile)) {
				continue
			}
			// Finalizers needs to run in order and in sequence.
			ordered, err := solution.Order(allRepos, w.Package.GetFingerPrint())
			if err != nil {
				return toFinalize, errors.Wrap(err, "While order a solution for "+w.Package.HumanReadableString())
			}
		ORDER:
			for _, ass := range ordered {
				if ass.Value {
					installed, ok := toInstall[ass.Package.GetFingerPrint()]
					if !ok {
						// It was a dep already installed in the system, so we can skip it safely
						continue ORDER
					}
					treePackage, err := installed.Repository.GetTree().GetDatabase().FindPackage(ass.Package)
					if err != nil {
						return toFinalize, errors.Wrap(err, "Error getting package "+ass.Package.HumanReadableString())
					}

					toFinalize = append(toFinalize, treePackage)
				}
			}
		}
	} else {
		for _, c := range toInstall {
			if !fileHelper.Exists(c.Package.Rel(tree.FinalizerFile)) {
				continue
			}
			treePackage, err := c.Repository.GetTree().GetDatabase().FindPackage(c.Package)
			if err != nil {
				return toFinalize, errors.Wrap(err, "Error getting package "+c.Package.HumanReadableString())
			}
			toFinalize = append(toFinalize, treePackage)
		}
	}
	return toFinalize, nil
}

func (l *LuetInstaller) checkFileconflicts(toInstall map[string]ArtifactMatch, checkSystem bool, s *System) error {
	l.Options.Context.Info("Checking for file conflicts..")
	defer s.Clean() // Release memory

	filesToInstall := map[string]interface{}{}
	for _, m := range toInstall {
		l.Options.Context.Debug("Checking file conflicts for", m.Package.HumanReadableString())

		a, err := l.getPackage(m, l.Options.Context)
		if err != nil && !l.Options.Force {
			return errors.Wrap(err, "Failed downloading package")
		}
		files, err := a.FileList()
		if err != nil && !l.Options.Force {
			return errors.Wrapf(err, "Could not get filelist for %s", a.CompileSpec.Package.HumanReadableString())
		}

		for _, f := range files {
			if _, ok := filesToInstall[f]; ok {
				return fmt.Errorf(
					"file conflict between packages to be installed",
				)
			}
			if checkSystem {
				exists, p, err := s.ExistsPackageFile(f)
				if err != nil {
					return errors.Wrap(err, "failed checking into system db")
				}
				if exists {
					return fmt.Errorf(
						"file conflict between '%s' and '%s' ( file: %s )",
						p.HumanReadableString(),
						m.Package.HumanReadableString(),
						f,
					)
				}
			}
			filesToInstall[f] = nil
		}
	}
	l.Options.Context.Info("Done checking for file conflicts..")

	return nil
}

func (l *LuetInstaller) install(o Option, syncedRepos Repositories, toInstall map[string]ArtifactMatch, p types.Packages, solution types.PackagesAssertions, allRepos types.PackageDatabase, s *System) error {

	// Download packages in parallel first
	if err := l.download(syncedRepos, toInstall); err != nil {
		return errors.Wrap(err, "Downloading packages")
	}

	if o.CheckFileConflicts {
		// Check file conflicts
		if err := l.checkFileconflicts(toInstall, true, s); err != nil {
			if !l.Options.Force {
				return errors.Wrap(err, "file conflict found")
			} else {
				l.Options.Context.Warning("file conflict found", err.Error())
			}
		}
	}

	if l.Options.DownloadOnly {
		return nil
	}

	all := make(chan ArtifactMatch)

	wg := new(sync.WaitGroup)
	installLock := &sync.Mutex{}

	// Do the real install
	for i := 0; i < l.Options.Concurrency; i++ {
		wg.Add(1)
		go l.installerWorker(i, wg, installLock, all, s)
	}

	for _, c := range toInstall {
		all <- c
	}
	close(all)
	wg.Wait()

	for _, c := range toInstall {
		// Annotate to the system that the package was installed
		_, err := s.Database.CreatePackage(c.Package)
		if err != nil && !o.Force {
			return errors.Wrap(err, "Failed creating package")
		}
		bus.Manager.Publish(bus.EventPackageInstall, c)
	}

	if !o.RunFinalizers {
		return nil
	}

	toFinalize, err := l.getFinalizers(allRepos, solution, toInstall, o.NoDeps)
	if err != nil {
		return errors.Wrap(err, "failed getting package to finalize")
	}

	return s.ExecuteFinalizers(l.Options.Context, toFinalize)
}

func (l *LuetInstaller) getPackage(a ArtifactMatch, ctx types.Context) (artifact *artifact.PackageArtifact, err error) {
	cli := a.Repository.Client(ctx)

	artifact, err = cli.DownloadArtifact(a.Artifact)
	if err != nil {
		return nil, errors.Wrap(err, "Error on download artifact")
	}

	err = artifact.Verify()
	if err != nil {
		return nil, errors.Wrap(err, "Artifact integrity check failure")
	}
	return artifact, nil
}

func (l *LuetInstaller) installPackage(m ArtifactMatch, s *System) error {

	a, err := l.getPackage(m, l.Options.Context)
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Failed downloading package")
	}

	files, err := a.FileList()
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "Could not open package archive")
	}

	err = a.Unpack(l.Options.Context, s.Target, true)
	if err != nil && !l.Options.Force {
		return errors.Wrap(err, "error met while unpacking package "+a.Path)
	}

	// First create client and download
	// Then unpack to system
	return s.Database.SetPackageFiles(&types.PackageFile{PackageFingerprint: m.Package.GetFingerPrint(), Files: files})
}

func (l *LuetInstaller) downloadWorker(i int, wg *sync.WaitGroup, pb *pterm.ProgressbarPrinter, c <-chan ArtifactMatch, ctx types.Context) error {
	defer wg.Done()

	for p := range c {
		// TODO: Keep trace of what was added from the tar, and save it into system
		_, err := l.getPackage(p, ctx)
		if err != nil {
			l.Options.Context.Error("Failed downloading package "+p.Package.GetName(), err.Error())
			return errors.Wrap(err, "Failed downloading package "+p.Package.GetName())
		} else {
			l.Options.Context.Success(":package: Package ", p.Package.HumanReadableString(), "downloaded")
		}
		if pb != nil {
			pb.Increment()
		}
	}

	return nil
}

func (l *LuetInstaller) installerWorker(i int, wg *sync.WaitGroup, installLock *sync.Mutex, c <-chan ArtifactMatch, s *System) error {
	defer wg.Done()

	for p := range c {
		// TODO: Keep trace of what was added from the tar, and save it into system
		installLock.Lock()
		err := l.installPackage(p, s)
		installLock.Unlock()
		if err != nil && !l.Options.Force {
			//TODO: Uninstall, rollback.
			l.Options.Context.Error("Failed installing package "+p.Package.GetName(), err.Error())
			return errors.Wrap(err, "Failed installing package "+p.Package.GetName())
		}
		if err == nil {
			l.Options.Context.Info(":package: Package ", p.Package.HumanReadableString(), "installed")
		} else if err != nil && l.Options.Force {
			l.Options.Context.Info(":package: Package ", p.Package.HumanReadableString(), "installed with failures (forced install)")
		}
	}

	return nil
}

func checkAndPrunePath(ctx types.Context, target, path string) {
	// check if now the target path is empty
	targetPath := filepath.Dir(path)

	if target == targetPath {
		return
	}

	fi, err := os.Lstat(targetPath)
	if err != nil {
		//	l.Options.Context.Warning("Dir not found (it was before?) ", err.Error())
		return
	}

	switch mode := fi.Mode(); {
	case mode.IsDir():
		files, err := ioutil.ReadDir(targetPath)
		if err != nil {
			ctx.Warning("Failed reading folder", targetPath, err.Error())
			return
		}
		if len(files) != 0 {
			ctx.Debug("Preserving not-empty folder", targetPath)
			return
		}
	}
	if err = os.Remove(targetPath); err != nil {
		ctx.Warning("Failed removing file (maybe not present in the system target anymore ?)", targetPath, err.Error())
	}
}

// We will try to cleanup every path from the file, if the folders left behind are empty
func pruneEmptyFilePath(ctx types.Context, target string, path string) {
	checkAndPrunePath(ctx, target, path)

	// A path is for e.g. /usr/bin/bar
	// we want to create an array
	// as "/usr", "/usr/bin", "/usr/bin/bar",
	// excluding the target (in the case above was /)
	paths := strings.Split(path, string(os.PathSeparator))
	currentPath := filepath.Join(string(os.PathSeparator), paths[0])
	allPaths := []string{}
	if strings.HasPrefix(currentPath, target) && target != currentPath {
		allPaths = append(allPaths, currentPath)
	}
	for _, p := range paths[1:] {
		currentPath = filepath.Join(currentPath, p)
		if strings.HasPrefix(currentPath, target) && target != currentPath {
			allPaths = append(allPaths, currentPath)
		}
	}
	helpers.ReverseAny(allPaths)
	for _, p := range allPaths {
		checkAndPrunePath(ctx, target, p)
	}
}

func (l *LuetInstaller) pruneFile(f string, s *System, cp *config.ConfigProtect) {
	target := filepath.Join(s.Target, f)

	if !l.Options.Context.GetConfig().ConfigProtectSkip && cp.Protected(f) {
		l.Options.Context.Debug("Preserving protected file:", f)
		return
	}

	l.Options.Context.Debug("Removing", target)
	if l.Options.PreserveSystemEssentialData &&
		strings.HasPrefix(f, l.Options.Context.GetConfig().System.PkgsCachePath) ||
		strings.HasPrefix(f, l.Options.Context.GetConfig().System.DatabasePath) {
		l.Options.Context.Warning("Preserve ", f, " which is required by luet ( you have to delete it manually if you really need to)")
		return
	}

	fi, err := os.Lstat(target)
	if err != nil {
		l.Options.Context.Debug("File not found (it was before?) ", err.Error())
		return
	}
	switch mode := fi.Mode(); {
	case mode.IsDir():
		files, err := ioutil.ReadDir(target)
		if err != nil {
			l.Options.Context.Debug("Failed reading folder", target, err.Error())
		}
		if len(files) != 0 {
			l.Options.Context.Debug("Preserving not-empty folder", target)
			return
		}
	}

	if err = os.Remove(target); err != nil {
		l.Options.Context.Debug("Failed removing file (maybe not present in the system target anymore ?)", target, err.Error())
	} else {
		l.Options.Context.Debug("Removed", target)
	}

	pruneEmptyFilePath(l.Options.Context, s.Target, target)
}

func (l *LuetInstaller) configProtectForPackage(p *types.Package, s *System, files []string) *config.ConfigProtect {

	var cp *config.ConfigProtect

	if !l.Options.Context.GetConfig().ConfigProtectSkip {
		annotationDir, _ := p.Annotations[types.ConfigProtectAnnotation]
		cp = config.NewConfigProtect(annotationDir)
		cp.Map(files, l.Options.Context.GetConfig().ConfigProtectConfFiles)
	}

	return cp
}

func (l *LuetInstaller) pruneFiles(files []string, cp *config.ConfigProtect, s *System) {

	toRemove, notPresent := fileHelper.OrderFiles(s.Target, files)

	// Remove from target
	for _, f := range append(toRemove, notPresent...) {
		l.pruneFile(f, s, cp)
	}
}

func (l *LuetInstaller) uninstall(p *types.Package, s *System) error {
	files, err := s.Database.GetPackageFiles(p)
	if err != nil {
		return errors.Wrap(err, "Failed getting installed files")
	}

	cp := l.configProtectForPackage(p, s, files)

	l.pruneFiles(files, cp, s)

	err = l.removePackage(p, s)
	if err != nil {
		return errors.Wrap(err, "Failed removing package files from database")
	}

	l.Options.Context.Info(":recycle: ", p.HumanReadableString(), "Removed :heavy_check_mark:")
	return nil
}

func (l *LuetInstaller) removePackage(p *types.Package, s *System) error {
	err := s.Database.RemovePackageFiles(p)
	if err != nil {
		return errors.Wrap(err, "Failed removing package files from database")
	}
	err = s.Database.RemovePackage(p)
	if err != nil {
		return errors.Wrap(err, "Failed removing package from database")
	}

	bus.Manager.Publish(bus.EventPackageUnInstall, p)
	return nil
}

func (l *LuetInstaller) computeUninstall(o Option, s *System, packs ...*types.Package) (types.Packages, error) {

	var toUninstall types.Packages
	// compute uninstall from all world - remove packages in parallel - run uninstall finalizer (in order) TODO - mark the uninstallation in db
	// Get installed definition
	checkConflicts := o.CheckConflicts
	full := o.FullUninstall
	// if o.Force == true { // IF forced, we want to remove the package and all its requires
	// 	checkConflicts = false
	// 	full = false
	// }

	// Create a temporary DB with the installed packages
	// so the solver is much faster finding the deptree
	// First check what would have been done
	installedtmp, err := s.Database.Copy()
	if err != nil {
		return toUninstall, errors.Wrap(err, "Failed create temporary in-memory db")
	}

	if !o.NoDeps {
		solv := solver.NewResolver(
			types.SolverOptions{
				Type:        l.Options.SolverOptions.Implementation,
				Concurrency: l.Options.Concurrency,
			},
			installedtmp,
			installedtmp,
			pkg.NewInMemoryDatabase(false),
			solver.NewSolverFromOptions(l.Options.SolverOptions))
		var solution types.Packages
		var err error
		if o.FullCleanUninstall {
			solution, err = solv.UninstallUniverse(packs)
			if err != nil {
				return toUninstall, errors.Wrap(err, "Could not solve the uninstall constraints. Tip: try with --solver-type qlearning or with --force, or by removing packages excluding their dependencies with --nodeps")
			}
		} else {
			solution, err = solv.Uninstall(checkConflicts, full, packs...)
			if err != nil && !l.Options.Force {
				return toUninstall, errors.Wrap(err, "Could not solve the uninstall constraints. Tip: try with --solver-type qlearning or with --force, or by removing packages excluding their dependencies with --nodeps")
			}
		}

		toUninstall = append(toUninstall, solution...)
	} else {
		toUninstall = append(toUninstall, packs...)
	}

	return toUninstall, nil
}

func (l *LuetInstaller) generateUninstallFn(o Option, s *System, filesToInstall map[string]interface{}, packs ...*types.Package) (types.Packages, func() error, error) {
	for _, p := range packs {
		if packs, _ := s.Database.FindPackages(p); len(packs) == 0 {
			return nil, nil, errors.New(fmt.Sprintf("Package %s not found in the system", p.HumanReadableString()))
		}
	}

	toUninstall, err := l.computeUninstall(o, s, packs...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "while computing uninstall")
	}

	uninstall := func() error {
		for _, p := range toUninstall {
			if len(filesToInstall) == 0 {
				err := l.uninstall(p, s)
				if err != nil && !o.Force {
					return errors.Wrap(err, "Uninstall failed")
				}

			} else {
				files, err := s.Database.GetPackageFiles(p)
				if err != nil && !o.Force {
					return errors.Wrap(err, "Failed getting installed files")
				}

				cp := l.configProtectForPackage(p, s, files)

				toPrune := []string{}
				for _, f := range files {
					if _, exists := filesToInstall[f]; !exists {
						toPrune = append(toPrune, f)
					}
				}
				l.Options.Context.Debug("calculated files for removal", toPrune)
				l.pruneFiles(toPrune, cp, s)

				err = l.removePackage(p, s)
				if err != nil && !o.Force {
					return errors.Wrap(err, "Failed removing package")
				}

			}
		}
		return nil
	}

	return toUninstall, uninstall, nil
}

func (l *LuetInstaller) Uninstall(s *System, packs ...*types.Package) error {
	l.Options.Context.Screen("Uninstall")

	l.Options.Context.Spinner()
	o := Option{
		FullUninstall:      l.Options.FullUninstall,
		Force:              l.Options.Force,
		CheckConflicts:     l.Options.CheckConflicts,
		FullCleanUninstall: l.Options.FullCleanUninstall,
	}
	toUninstall, uninstall, err := l.generateUninstallFn(o, s, map[string]interface{}{}, packs...)
	if err != nil {
		return errors.Wrap(err, "while computing uninstall")
	}
	l.Options.Context.SpinnerStop()

	if len(toUninstall) == 0 {
		l.Options.Context.Info("Nothing to do")
		return nil
	}

	if l.Options.Ask {
		l.Options.Context.Info(":recycle: Packages that are going to be removed from the system:")
		printList(toUninstall)
		if l.Options.Context.Ask() {
			l.Options.Ask = false // Don't prompt anymore
			return uninstall()
		} else {
			return errors.New("Aborted by user")
		}
	}
	return uninstall()
}
