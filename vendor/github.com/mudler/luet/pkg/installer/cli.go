// Copyright © 2021 Ettore Di Giacinto <mudler@gentoo.org>
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
	"sort"
	"strings"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pterm/pterm"
)

func packsToList(p types.Packages) string {
	var packs []string

	for _, pp := range p {
		packs = append(packs, pp.HumanReadableString())
	}

	sort.Strings(packs)
	return strings.Join(packs, " ")
}

func printList(p types.Packages) {
	fmt.Println()
	d := pterm.TableData{{"Program Name", "Version", "License"}}
	for _, m := range p {
		d = append(d, []string{
			fmt.Sprintf("%s/%s", m.GetCategory(), m.GetName()),
			pterm.LightGreen(m.GetVersion()), m.GetLicense()})
	}
	pterm.DefaultTable.WithHasHeader().WithData(d).Render()
	fmt.Println()
}

func printUpgradeList(install, uninstall types.Packages) {
	fmt.Println()

	d := pterm.TableData{{"Old version", "New version", "License"}}
	for _, m := range uninstall {
		if p, err := install.Find(m.GetPackageName()); err == nil {
			d = append(d, []string{
				pterm.LightRed(m.HumanReadableString()),
				pterm.LightGreen(p.HumanReadableString()), m.GetLicense()})
		} else {
			d = append(d, []string{
				pterm.LightRed(m.HumanReadableString()), ""})
		}
	}
	for _, m := range install {
		if _, err := uninstall.Find(m.GetPackageName()); err != nil {
			d = append(d, []string{"",
				pterm.LightGreen(m.HumanReadableString()), m.GetLicense()})
		}
	}
	pterm.DefaultTable.WithHasHeader().WithData(d).Render()
	fmt.Println()

}

func printMatchUpgrade(artefacts map[string]ArtifactMatch, uninstall types.Packages) {
	p := types.Packages{}

	for _, a := range artefacts {
		p = append(p, a.Package)
	}

	printUpgradeList(p, uninstall)
}

func printMatches(artefacts map[string]ArtifactMatch) {
	fmt.Println()
	d := pterm.TableData{{"Program Name", "Version", "License", "Repository"}}
	for _, m := range artefacts {
		d = append(d, []string{
			m.Package.HumanReadableString(),
			pterm.LightGreen(m.Package.GetVersion()), m.Package.GetLicense(), m.Repository.GetName()})
	}
	pterm.DefaultTable.WithHasHeader().WithData(d).Render()
	fmt.Println()
}
