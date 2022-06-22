// Copyright © 2019-2021 Ettore Di Giacinto <mudler@gentoo.org>
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

package version

import (
	"errors"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
	semver "github.com/hashicorp/go-version"
	debversion "github.com/knqyf263/go-deb-version"
)

const (
	selectorGreaterThen        = iota
	selectorLessThen           = iota
	selectorGreaterOrEqualThen = iota
	selectorLessOrEqualThen    = iota
	selectorNotEqual           = iota
)

type packageSelector struct {
	Condition int
	Version   string
}

var selectors = map[string]int{
	">=": selectorGreaterOrEqualThen,
	">":  selectorGreaterThen,
	"<=": selectorLessOrEqualThen,
	"<":  selectorLessThen,
	"!":  selectorNotEqual,
}

func readPackageSelector(selector string) packageSelector {
	selectorType := 0
	v := ""

	k := []string{}
	for kk, _ := range selectors {
		k = append(k, kk)
	}

	sort.Slice(k, func(i, j int) bool {
		return len(k[i]) > len(k[j])
	})
	for _, p := range k {
		if strings.HasPrefix(selector, p) {
			selectorType = selectors[p]
			v = strings.TrimPrefix(selector, p)
			break
		}
	}
	return packageSelector{
		Condition: selectorType,
		Version:   v,
	}
}

func semverCheck(vv string, selector string) (bool, error) {
	c, err := semver.NewConstraint(selector)
	if err != nil {
		// Handle constraint not being parsable.

		return false, err
	}

	v, err := semver.NewVersion(vv)
	if err != nil {
		// Handle version not being parsable.

		return false, err
	}

	// Check if the version meets the constraints.
	return c.Check(v), nil
}

// WrappedVersioner uses different means to return unique result that is understendable by Luet
// It tries different approaches to sort, validate, and sanitize to a common versioning format
// that is understendable by the whole code
type WrappedVersioner struct{}

func DefaultVersioner() Versioner {
	return &WrappedVersioner{}
}

func (w *WrappedVersioner) Validate(version string) error {
	if !debversion.Valid(version) {
		return errors.New("invalid version")
	}
	return nil
}

func (w *WrappedVersioner) ValidateSelector(vv string, selector string) bool {
	if vv == "" {
		return true
	}
	vv = w.Sanitize(vv)
	selector = w.Sanitize(selector)

	sel := readPackageSelector(selector)

	selectorV, err := version.NewVersion(sel.Version)
	if err != nil {
		f, _ := semverCheck(vv, selector)
		return f
	}
	v, err := version.NewVersion(vv)
	if err != nil {
		f, _ := semverCheck(vv, selector)
		return f
	}

	switch sel.Condition {
	case selectorGreaterOrEqualThen:
		return v.GreaterThan(selectorV) || v.Equal(selectorV)
	case selectorLessOrEqualThen:
		return v.LessThan(selectorV) || v.Equal(selectorV)
	case selectorLessThen:
		return v.LessThan(selectorV)
	case selectorGreaterThen:
		return v.GreaterThan(selectorV)
	case selectorNotEqual:
		return !v.Equal(selectorV)
	}

	return false
}

func (w *WrappedVersioner) Sanitize(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "_", "-"))
}

func (w *WrappedVersioner) Sort(toSort []string) []string {
	if len(toSort) == 0 {
		return toSort
	}
	var versionsMap map[string]string = make(map[string]string)
	versionsRaw := []string{}
	result := []string{}
	for _, v := range toSort {
		sanitizedVersion := w.Sanitize(v)
		versionsMap[sanitizedVersion] = v
		versionsRaw = append(versionsRaw, sanitizedVersion)
	}

	vs := make([]debversion.Version, len(versionsRaw))
	for i, r := range versionsRaw {
		v, _ := debversion.NewVersion(r)
		vs[i] = v
	}

	sort.Slice(vs, func(i, j int) bool {
		return vs[i].LessThan(vs[j])
	})

	for _, v := range vs {
		result = append(result, versionsMap[v.String()])
	}
	return result
}
