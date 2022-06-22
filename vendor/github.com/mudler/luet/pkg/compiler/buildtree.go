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
	"encoding/json"
	"sort"
)

func rankMapStringInt(values map[string]int) []string {
	type kv struct {
		Key   string
		Value int
	}
	var ss []kv
	for k, v := range values {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})
	ranked := make([]string, len(values))
	for i, kv := range ss {
		ranked[i] = kv.Key
	}
	return ranked
}

type BuildTree struct {
	order map[string]int
}

func (bt *BuildTree) Increase(s string) {
	if bt.order == nil {
		bt.order = make(map[string]int)
	}
	if _, ok := bt.order[s]; !ok {
		bt.order[s] = 0
	}

	bt.order[s]++
}

func (bt *BuildTree) Reset(s string) {
	if bt.order == nil {
		bt.order = make(map[string]int)
	}
	bt.order[s] = 0
}

func (bt *BuildTree) Level(s string) int {
	return bt.order[s]
}

func ints(input []int) []int {
	u := make([]int, 0, len(input))
	m := make(map[int]bool)

	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}

	return u
}

func (bt *BuildTree) AllInLevel(l int) []string {
	var all []string
	for k, v := range bt.order {
		if v == l {
			all = append(all, k)
		}
	}
	return all
}

func (bt *BuildTree) Order(compilationTree map[string]map[string]interface{}) {
	sentinel := false
	for !sentinel {
		sentinel = true

	LEVEL:
		for _, l := range bt.AllLevels() {

			for _, j := range bt.AllInLevel(l) {
				for _, k := range bt.AllInLevel(l) {
					if j == k {
						continue
					}
					if _, ok := compilationTree[j][k]; ok {
						if bt.Level(j) == bt.Level(k) {
							bt.Increase(j)
							sentinel = false
							break LEVEL
						}
					}
				}
			}
		}
	}
}

func (bt *BuildTree) AllLevels() []int {
	var all []int
	for _, v := range bt.order {
		all = append(all, v)
	}

	sort.Sort(sort.IntSlice(all))

	return ints(all)
}

func (bt *BuildTree) JSON() (string, error) {
	type buildjob struct {
		Jobs []string `json:"packages"`
	}

	result := []buildjob{}
	for _, l := range bt.AllLevels() {
		result = append(result, buildjob{Jobs: bt.AllInLevel(l)})
	}
	dat, err := json.Marshal(&result)
	return string(dat), err
}
