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

package types

import (
	"fmt"
	"runtime"

	"gopkg.in/yaml.v2"
)

type LuetRepository struct {
	Name           string            `json:"name" yaml:"name" mapstructure:"name"`
	Description    string            `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description"`
	Urls           []string          `json:"urls" yaml:"urls" mapstructure:"urls"`
	Type           string            `json:"type" yaml:"type" mapstructure:"type"`
	Mode           string            `json:"mode,omitempty" yaml:"mode,omitempty" mapstructure:"mode,omitempty"`
	Priority       int               `json:"priority,omitempty" yaml:"priority,omitempty" mapstructure:"priority"`
	Enable         bool              `json:"enable" yaml:"enable" mapstructure:"enable"`
	Cached         bool              `json:"cached,omitempty" yaml:"cached,omitempty" mapstructure:"cached,omitempty"`
	Authentication map[string]string `json:"auth,omitempty" yaml:"auth,omitempty" mapstructure:"auth,omitempty"`
	TreePath       string            `json:"treepath,omitempty" yaml:"treepath,omitempty" mapstructure:"treepath"`
	MetaPath       string            `json:"metapath,omitempty" yaml:"metapath,omitempty" mapstructure:"metapath"`
	Verify         bool              `json:"verify,omitempty" yaml:"verify,omitempty" mapstructure:"verify"`
	Arch           string            `json:"arch,omitempty" yaml:"arch,omitempty" mapstructure:"arch"`

	ReferenceID string `json:"reference,omitempty" yaml:"reference,omitempty" mapstructure:"reference"`

	// Incremented value that identify revision of the repository in a user-friendly way.
	Revision int `json:"revision,omitempty" yaml:"-" mapstructure:"-"`
	// Epoch time in seconds
	LastUpdate string `json:"last_update,omitempty" yaml:"-" mapstructure:"-"`
}

func (r *LuetRepository) String() string {
	return fmt.Sprintf("[%s] prio: %d, type: %s, enable: %t, cached: %t",
		r.Name, r.Priority, r.Type, r.Enable, r.Cached)
}

// Enabled returns a boolean indicating if the repository should be considered enabled or not
func (r *LuetRepository) Enabled() bool {
	return r.Arch != "" && r.Arch == runtime.GOARCH && !r.Enable || r.Enable
}

type LuetRepositories []LuetRepository

func (l LuetRepositories) Enabled() (res LuetRepositories) {
	for _, r := range l {
		if r.Enabled() {
			res = append(res, r)
		}
	}
	return
}

func NewLuetRepository(name, t, descr string, urls []string, priority int, enable, cached bool) *LuetRepository {
	return &LuetRepository{
		Name:           name,
		Description:    descr,
		Urls:           urls,
		Type:           t,
		Priority:       priority,
		Enable:         enable,
		Cached:         cached,
		Authentication: make(map[string]string),
	}
}

func NewEmptyLuetRepository() *LuetRepository {
	return &LuetRepository{
		Priority:       9999,
		Authentication: make(map[string]string),
	}
}

func LoadRepository(data []byte) (*LuetRepository, error) {
	ans := NewEmptyLuetRepository()
	err := yaml.Unmarshal(data, &ans)
	if err != nil {
		return nil, err
	}
	return ans, nil
}
