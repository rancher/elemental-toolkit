/*

Copyright (C) 2017-2021  Daniele Rondina <geaaru@sabayonlinux.org>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

*/
package gentoo

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	version "github.com/hashicorp/go-version"
)

// ----------------------------------
// Code to move and merge inside luet project
// ----------------------------------

// Package condition
type PackageCond int

const (
	PkgCondInvalid = 0
	// >
	PkgCondGreater = 1
	// >=
	PkgCondGreaterEqual = 2
	// <
	PkgCondLess = 3
	// <=
	PkgCondLessEqual = 4
	// =
	PkgCondEqual = 5
	// !
	PkgCondNot = 6
	// ~
	PkgCondAnyRevision = 7
	// =<pkg>*
	PkgCondMatchVersion = 8
	// !<
	PkgCondNotLess = 9
	// !>
	PkgCondNotGreater = 10
)

const (
	RegexCatString     = `(^[a-z]+[0-9]*[a-z]*[-]*[a-z]+[0-9]*[a-z]*|^virtual)`
	RegexPkgNameString = `([a-zA-Z]*[0-9a-zA-Z\.\-_]*[a-zA-Z0-9]+|[a-zA-Z\-]+[+]+[-]+[0-9a-zA-Z\.]*|[a-zA-Z\-]+[+]+)`
)

type GentooPackage struct {
	Name          string `json:"name,omitempty"`
	Category      string `json:"category,omitempty"`
	Version       string `json:"version,omitempty"`
	VersionSuffix string `json:"version_suffix,omitempty"`
	VersionBuild  string `json:"version_build,omitempty"`
	Slot          string `json:"slot,omitempty"`
	Condition     PackageCond
	Repository    string   `json:"repository,omitempty"`
	UseFlags      []string `json:"use_flags,omitempty"`
	License       string   `json:"license,omitempty"`
}

func (p *GentooPackage) String() string {
	// TODO
	opt := ""
	if p.Version != "" {
		opt = "-"
	}
	return fmt.Sprintf("%s/%s%s%s%s",
		p.Category, p.Name, opt,
		p.Version, p.VersionSuffix)
}

func (p PackageCond) String() (ans string) {
	if p == PkgCondInvalid {
		ans = ""
	} else if p == PkgCondGreater {
		ans = ">"
	} else if p == PkgCondGreaterEqual {
		ans = ">="
	} else if p == PkgCondLess {
		ans = "<"
	} else if p == PkgCondLessEqual {
		ans = "<="
	} else if p == PkgCondEqual {
		ans = "="
	} else if p == PkgCondAnyRevision {
		ans = "~"
	} else if p == PkgCondMatchVersion {
		ans = "=*"
	} else if p == PkgCondNotLess {
		ans = "!<"
	} else if p == PkgCondNotGreater {
		ans = "!>"
	} else if p == PkgCondNot {
		ans = "!"
	}

	return ans
}

func (p PackageCond) Int() (ans int) {
	if p == PkgCondInvalid {
		ans = PkgCondInvalid
	} else if p == PkgCondGreater {
		ans = PkgCondGreater
	} else if p == PkgCondGreaterEqual {
		ans = PkgCondGreaterEqual
	} else if p == PkgCondLess {
		ans = PkgCondLess
	} else if p == PkgCondLessEqual {
		ans = PkgCondLessEqual
	} else if p == PkgCondEqual {
		// To permit correct matching on database
		// we currently use directly package version without =
		ans = PkgCondEqual
	} else if p == PkgCondNot {
		ans = PkgCondNot
	} else if p == PkgCondAnyRevision {
		ans = PkgCondAnyRevision
	} else if p == PkgCondMatchVersion {
		ans = PkgCondMatchVersion
	} else if p == PkgCondNotLess {
		ans = PkgCondNotLess
	} else if p == PkgCondNotGreater {
		ans = PkgCondNotGreater
	}
	return
}

func sanitizeVersion(v string) string {
	// https://devmanual.gentoo.org/ebuild-writing/file-format/index.html
	ans := strings.ReplaceAll(v, "_alpha", "-alpha")
	ans = strings.ReplaceAll(ans, "_beta", "-beta")
	ans = strings.ReplaceAll(ans, "_pre", "-pre")
	ans = strings.ReplaceAll(ans, "_rc", "-rc")
	ans = strings.ReplaceAll(ans, "_p", "-p")

	return ans
}

func (p *GentooPackage) OfPackage(i *GentooPackage) (ans bool) {
	if p.Category == i.Category && p.Name == i.Name {
		ans = true
	} else {
		ans = false
	}
	return
}

func (p *GentooPackage) GetPackageName() (ans string) {
	ans = fmt.Sprintf("%s/%s", p.Category, p.Name)
	return
}

func (p *GentooPackage) GetPackageNameWithSlot() (ans string) {
	if p.Slot != "0" {
		ans = fmt.Sprintf("%s:%s", p.GetPackageName(), p.Slot)
	} else {
		ans = p.GetPackageName()
	}
	return
}

func (p *GentooPackage) GetP() string {
	return fmt.Sprintf("%s-%s", p.Name, p.GetPV())
}

func (p *GentooPackage) GetPN() string {
	return p.Name
}

func (p *GentooPackage) GetPV() string {
	return fmt.Sprintf("%s", p.Version)
}

func (p *GentooPackage) GetPackageNameWithCond() (ans string) {
	ans = fmt.Sprintf("%s%s", p.Condition.String(), p.GetPackageName())
	return
}

func (p *GentooPackage) GetPVR() (ans string) {
	if p.VersionSuffix != "" {
		ans = fmt.Sprintf("%s%s", p.Version, p.VersionSuffix)
	} else {
		ans = p.GetPV()
	}
	return
}

func (p *GentooPackage) GetPF() string {
	return fmt.Sprintf("%s-%s", p.GetPN(), p.GetPVR())
}

func (p *GentooPackage) getVersions(i *GentooPackage) (*version.Version, *version.Version, error) {
	var v1 *version.Version = nil
	var v2 *version.Version = nil
	var err error

	if p.Category != i.Category {
		return v1, v2, errors.New(
			fmt.Sprintf("Wrong category for package %s", i.Name))
	}

	if p.Name != i.Name {
		return v1, v2, errors.New(
			fmt.Sprintf("Wrong name for package %s", i.Name))
	}

	if p.Version == "" {
		return v1, v2, errors.New(
			fmt.Sprintf("Package without version. I can't compare versions."))
	}

	if i.Version == "" {
		return v1, v2, errors.New(
			fmt.Sprintf("Package supply without version. I can't compare versions."))
	}

	v1s := p.Version
	v2s := i.Version

	if p.VersionBuild != "" {
		v1s = p.Version + "+" + p.VersionBuild
	}
	v1, err = version.NewVersion(v1s)
	if err != nil {
		return nil, nil, err
	}
	if i.VersionBuild != "" {
		v2s = i.Version + "+" + i.VersionBuild
	}
	v2, err = version.NewVersion(v2s)
	if err != nil {
		return nil, nil, err
	}

	return v1, v2, nil
}

func (p *GentooPackage) orderDifferentPkgs(i *GentooPackage, mode int) bool {
	if p.Category != i.Category {
		if mode == 0 {
			return p.Category < i.Category
		}
		return p.Category > i.Category
	}
	if mode == 0 {
		return p.Name < i.Name
	}
	return p.Name > i.Name
}

func (p *GentooPackage) GreaterThan(i *GentooPackage) (bool, error) {
	var ans bool
	if p.Category != i.Category || p.Name != i.Name {
		return p.orderDifferentPkgs(i, 1), nil
	}
	v1, v2, err := p.getVersions(i)
	if err != nil {
		return false, err
	}

	if v1.Equal(v2) {
		// Order suffix and VersionBuild
		versionsSuffix := []string{
			p.VersionSuffix + "+" + p.VersionBuild,
			i.VersionSuffix + "+" + i.VersionBuild,
		}

		sort.Strings(versionsSuffix)
		if versionsSuffix[1] == p.VersionSuffix+"+"+p.VersionBuild {
			ans = true
		} else {
			ans = false
		}

	} else {
		ans = v1.GreaterThan(v2)
	}
	return ans, nil
}

func (p *GentooPackage) LessThan(i *GentooPackage) (bool, error) {
	var ans bool

	if p.Category != i.Category || p.Name != i.Name {
		return p.orderDifferentPkgs(i, 0), nil
	}
	v1, v2, err := p.getVersions(i)
	if err != nil {
		return false, err
	}

	if v1.Equal(v2) {
		// Order suffix and VersionBuild
		versionsSuffix := []string{
			p.VersionSuffix + "+" + p.VersionBuild,
			i.VersionSuffix + "+" + i.VersionBuild,
		}

		sort.Strings(versionsSuffix)
		if versionsSuffix[0] == p.VersionSuffix+"+"+p.VersionBuild {
			ans = true
		} else {
			ans = false
		}

	} else {
		ans = v1.LessThan(v2)
	}
	return ans, nil
}

func (p *GentooPackage) LessThanOrEqual(i *GentooPackage) (bool, error) {
	var ans bool
	if p.Category != i.Category || p.Name != i.Name {
		return p.orderDifferentPkgs(i, 0), nil
	}
	v1, v2, err := p.getVersions(i)
	if err != nil {
		return false, err
	}

	if v1.Equal(v2) {
		// Order suffix and VersionBuild
		versionsSuffix := []string{
			p.VersionSuffix + "+" + p.VersionBuild,
			i.VersionSuffix + "+" + i.VersionBuild,
		}

		sort.Strings(versionsSuffix)
		if versionsSuffix[0] == p.VersionSuffix+"+"+p.VersionBuild {
			ans = true
		} else {
			ans = false
		}

	} else {
		ans = v1.LessThanOrEqual(v2)
	}
	return ans, nil
}

func (p *GentooPackage) GreaterThanOrEqual(i *GentooPackage) (bool, error) {
	var ans bool

	if p.Category != i.Category || p.Name != i.Name {
		return p.orderDifferentPkgs(i, 1), nil
	}
	v1, v2, err := p.getVersions(i)
	if err != nil {
		return false, err
	}

	if v1.Equal(v2) {
		// Order suffix and VersionBuild
		versionsSuffix := []string{
			p.VersionSuffix + "+" + p.VersionBuild,
			i.VersionSuffix + "+" + i.VersionBuild,
		}

		sort.Strings(versionsSuffix)
		if versionsSuffix[1] == p.VersionSuffix+"+"+p.VersionBuild {
			ans = true
		} else {
			ans = false
		}

	} else {
		ans = v1.LessThanOrEqual(v2)
	}
	return ans, nil
}

func (p *GentooPackage) Equal(i *GentooPackage) (bool, error) {
	v1, v2, err := p.getVersions(i)
	if err != nil {
		return false, err
	}
	ans := v1.Equal(v2)

	if ans && (p.VersionSuffix != i.VersionSuffix || p.VersionBuild != i.VersionBuild) {
		ans = false
	}

	return ans, nil
}

func (p *GentooPackage) Admit(i *GentooPackage) (bool, error) {
	var ans bool = false
	var v1 *version.Version = nil
	var v2 *version.Version = nil
	var err error

	if p.Category != i.Category {
		return false, errors.New(
			fmt.Sprintf("Wrong category for package %s", i.Name))
	}

	if p.Name != i.Name {
		return false, errors.New(
			fmt.Sprintf("Wrong name for package %s", i.Name))
	}

	// Check Slot
	if p.Slot != "" && i.Slot != "" && p.Slot != i.Slot {
		return false, nil
	}

	v1s := p.Version
	v2s := i.Version

	if v1s != "" {
		if p.VersionBuild != "" {
			v1s = p.Version + "+" + p.VersionBuild
		}
		v1, err = version.NewVersion(v1s)
		if err != nil {
			return false, err
		}
	}
	if v2s != "" {
		if i.VersionBuild != "" {
			v2s = i.Version + "+" + i.VersionBuild
		}
		v2, err = version.NewVersion(v2s)
		if err != nil {
			return false, err
		}
	}

	// If package doesn't define version admit all versions of the package.
	if p.Version == "" {
		ans = true
	} else {
		if p.Condition == PkgCondInvalid || p.Condition == PkgCondEqual {
			// case 1: source-pkg-1.0 and dest-pkg-1.0 or dest-pkg without version
			if i.Version != "" && i.Version == p.Version && p.VersionSuffix == i.VersionSuffix &&
				p.VersionBuild == i.VersionBuild {
				ans = true
			}
		} else if p.Condition == PkgCondAnyRevision {
			if v1 != nil && v2 != nil {
				ans = v1.Equal(v2)
			}
		} else if p.Condition == PkgCondMatchVersion {
			// TODO: case of 7.3* where 7.30 is accepted.
			if v1 != nil && v2 != nil {
				segments := v1.Segments()
				n := strings.Count(p.Version, ".")
				switch n {
				case 0:
					segments[0]++
				case 1:
					segments[1]++
				case 2:
					segments[2]++
				default:
					segments[len(segments)-1]++
				}
				nextVersion := strings.Trim(strings.Replace(fmt.Sprint(segments), " ", ".", -1), "[]")
				constraints, err := version.NewConstraint(
					fmt.Sprintf(">= %s, < %s", p.Version, nextVersion),
				)
				if err != nil {
					return false, err
				}
				ans = constraints.Check(v2)
			}
		} else if v1 != nil && v2 != nil {

			switch p.Condition {
			case PkgCondGreaterEqual:
				ans = v2.GreaterThanOrEqual(v1)
			case PkgCondLessEqual:
				ans = v2.LessThanOrEqual(v1)
			case PkgCondGreater:
				ans = v2.GreaterThan(v1)
			case PkgCondLess:
				ans = v2.LessThan(v1)
			case PkgCondNot:
				ans = !v2.Equal(v1)
			}

		}

	}

	return ans, nil
}

// return category, package, version, slot, condition
func ParsePackageStr(pkg string) (*GentooPackage, error) {
	if pkg == "" {
		return nil, errors.New("Invalid package string")
	}

	ans := GentooPackage{
		Slot:         "0",
		Condition:    PkgCondInvalid,
		VersionBuild: "",
	}

	// Check if pkg string contains inline use flags
	regexUses := regexp.MustCompile(
		"\\[([a-z]*[-]*[0-9]*[,]*)+\\]*$",
	)
	mUses := regexUses.FindAllString(pkg, -1)
	if len(mUses) > 0 {
		ans.UseFlags = strings.Split(
			pkg[len(pkg)-len(mUses[0])+1:len(pkg)-1],
			",",
		)
		pkg = pkg[:len(pkg)-len(mUses[0])]
	}

	if strings.HasPrefix(pkg, ">=") {
		pkg = pkg[2:]
		ans.Condition = PkgCondGreaterEqual
	} else if strings.HasPrefix(pkg, ">") {
		pkg = pkg[1:]
		ans.Condition = PkgCondGreater
	} else if strings.HasPrefix(pkg, "<=") {
		pkg = pkg[2:]
		ans.Condition = PkgCondLessEqual
	} else if strings.HasPrefix(pkg, "<") {
		pkg = pkg[1:]
		ans.Condition = PkgCondLess
	} else if strings.HasPrefix(pkg, "=") {
		pkg = pkg[1:]
		if strings.HasSuffix(pkg, "*") {
			ans.Condition = PkgCondMatchVersion
			pkg = pkg[0 : len(pkg)-1]
		} else {
			ans.Condition = PkgCondEqual
		}
	} else if strings.HasPrefix(pkg, "~") {
		pkg = pkg[1:]
		ans.Condition = PkgCondAnyRevision
	} else if strings.HasPrefix(pkg, "!<") {
		pkg = pkg[2:]
		ans.Condition = PkgCondNotLess
	} else if strings.HasPrefix(pkg, "!>") {
		pkg = pkg[2:]
		ans.Condition = PkgCondNotGreater
	} else if strings.HasPrefix(pkg, "!") {
		pkg = pkg[1:]
		ans.Condition = PkgCondNot
	}

	regexVerString := fmt.Sprintf("[-](%s|%s|%s|%s|%s|%s)((%s|%s|%s|%s|%s|%s||%s)+)*",
		// Version regex
		// 1.1
		"[0-9]+[.][0-9]+[a-z]*",
		// 1
		"[0-9]+[a-z]*",
		// 1.1.1
		"[0-9]+[.][0-9]+[.][0-9]+[a-z]*",
		// 1.1.1.1
		"[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[a-z]*",
		// 1.1.1.1.1
		"[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[a-z]*",
		// 1.1.1.1.1.1
		"[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[.][0-9]+[a-z]*",
		// suffix
		"-r[0-9]+",
		"_p[0-9]+",
		"_pre[0-9]*",
		"_rc[0-9]+",
		// handle also rc without number
		"_rc",
		"_alpha[0-9-a-z]*",
		"_beta[0-9-a-z]*",
	)

	// The slash is used also in slot.
	if strings.Index(pkg, "/") < 0 {
		return nil, errors.New(fmt.Sprintf("Invalid package string %s", pkg))
	}

	ans.Category = pkg[:strings.Index(pkg, "/")]
	pkgname := pkg[strings.Index(pkg, "/")+1:]

	// Validate category

	regexPkg := regexp.MustCompile(
		fmt.Sprintf("%s$", RegexCatString),
	)

	matches := regexPkg.FindAllString(ans.Category, -1)
	if len(matches) > 1 {
		return nil, errors.New(fmt.Sprintf("Invalid category %s", ans.Category))
	}

	hasBuild, _ := regexp.MatchString(
		fmt.Sprintf("(%s%s([[:]{1,2}[0-9a-zA-Z]*]*)*[+])",
			RegexPkgNameString, regexVerString),
		pkgname,
	)

	if hasBuild {
		// Check if build number is present
		buildIdx := strings.LastIndex(pkgname, "+")
		if buildIdx > 0 {
			// <pre-release> ::= <dot-separated pre-release identifiers>
			//
			// <dot-separated pre-release identifiers> ::=
			//      <pre-release identifier> | <pre-release identifier> "."
			//      <dot-separated pre-release identifiers>
			//
			// <build> ::= <dot-separated build identifiers>
			//
			// <dot-separated build identifiers> ::= <build identifier>
			//      | <build identifier> "." <dot-separated build identifiers>
			//
			// <pre-release identifier> ::= <alphanumeric identifier>
			//                            | <numeric identifier>
			//
			// <build identifier> ::= <alphanumeric identifier>
			//      | <digits>
			//
			// <alphanumeric identifier> ::= <non-digit>
			//      | <non-digit> <identifier characters>
			//      | <identifier characters> <non-digit>
			//      | <identifier characters> <non-digit> <identifier characters>
			ans.VersionBuild = pkgname[buildIdx+1:]
			pkgname = pkgname[0:buildIdx]
		}
	}

	// Check if there are use flags annotation
	if strings.Index(pkgname, "[") > 0 {
		useFlags := pkgname[strings.Index(pkgname, "[")+1 : strings.Index(pkgname, "]")]
		ans.UseFlags = strings.Split(useFlags, ",")
		p := pkgname[0:strings.Index(pkgname, "[")]
		if strings.Index(pkgname, "]") < len(pkgname) {
			p = p + pkgname[strings.Index(pkgname, "]")+1:len(pkgname)]
		}
		pkgname = p
	}

	// Check if has repository
	if strings.Contains(pkgname, "::") {
		words := strings.Split(pkgname, "::")
		ans.Repository = words[1]
		pkgname = words[0]
	}

	// Check if has slot
	if strings.Contains(pkgname, ":") {
		words := strings.Split(pkgname, ":")
		ans.Slot = words[1]
		pkgname = words[0]
	}

	// TODO: I don't like this but i don't want to die.
	// Could be handled better maybe with regex lookahead match in the future.

	if strings.HasSuffix(pkgname, "dpi") {
		// POST: skip versioning match
		ans.Name = pkgname
	} else {

		regexPkg = regexp.MustCompile(
			fmt.Sprintf("%s$", regexVerString),
		)

		matches = regexPkg.FindAllString(pkgname, -1)

		// NOTE: Now suffix comples like _alpha_rc1 are not supported.
		if len(matches) > 0 {
			// Check if there patch
			if strings.Contains(matches[0], "_p") {
				ans.Version = matches[0][1:strings.Index(matches[0], "_p")]
				ans.VersionSuffix = matches[0][strings.Index(matches[0], "_p"):]
			} else if strings.Contains(matches[0], "_rc") {
				ans.Version = matches[0][1:strings.Index(matches[0], "_rc")]
				ans.VersionSuffix = matches[0][strings.Index(matches[0], "_rc"):]
			} else if strings.Contains(matches[0], "_alpha") {
				ans.Version = matches[0][1:strings.Index(matches[0], "_alpha")]
				ans.VersionSuffix = matches[0][strings.Index(matches[0], "_alpha"):]
			} else if strings.Contains(matches[0], "_beta") {
				ans.Version = matches[0][1:strings.Index(matches[0], "_beta")]
				ans.VersionSuffix = matches[0][strings.Index(matches[0], "_beta"):]
			} else if strings.Contains(matches[0], "-r") {
				ans.Version = matches[0][1:strings.Index(matches[0], "-r")]
				ans.VersionSuffix = matches[0][strings.Index(matches[0], "-r"):]
			} else {
				ans.Version = matches[0][1:]
			}
			ans.Name = pkgname[0 : len(pkgname)-len(ans.Version)-1-len(ans.VersionSuffix)]
		} else {
			ans.Name = pkgname
		}
	}

	// Set condition if there isn't a prefix but only a version
	if ans.Condition == PkgCondInvalid && ans.Version != "" {
		ans.Condition = PkgCondEqual
	}

	return &ans, nil
}

type GentooPackageSorter []GentooPackage

func (p GentooPackageSorter) Len() int      { return len(p) }
func (p GentooPackageSorter) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p GentooPackageSorter) Less(i, j int) bool {
	ans, _ := p[i].LessThan(&p[j])
	return ans
}
