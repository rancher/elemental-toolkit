// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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

package entities

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const (
	UserKind    = "user"
	ShadowKind  = "shadow"
	GroupKind   = "group"
	GShadowKind = "gshadow"
)

type EntitiesParser interface {
	ReadEntity(entity string) (Entity, error)
}

type Signature struct {
	Kind string `yaml:"kind"`
}

type Parser struct{}

func (p Parser) ReadEntityFromBytes(yamlFile []byte) (Entity, error) {

	var signature Signature
	err := yaml.Unmarshal(yamlFile, &signature)
	if err != nil {
		return nil, errors.Wrap(err, "Failed while parsing entity file")
	}

	switch signature.Kind {
	case UserKind:
		var user UserPasswd

		err = yaml.Unmarshal(yamlFile, &user)
		if err != nil {
			return nil, errors.Wrap(err, "Failed while parsing entity file")
		}
		return user, nil
	case ShadowKind:
		var shad Shadow

		err = yaml.Unmarshal(yamlFile, &shad)
		if err != nil {
			return nil, errors.Wrap(err, "Failed while parsing entity file")
		}
		return shad, nil
	case GroupKind:
		var group Group

		err = yaml.Unmarshal(yamlFile, &group)
		if err != nil {
			return nil, errors.Wrap(err, "Failed while parsing entity file")
		}
		return group, nil

	case GShadowKind:
		var group GShadow

		err = yaml.Unmarshal(yamlFile, &group)
		if err != nil {
			return nil, errors.Wrap(err, "Failed while parsing entity file")
		}
		return group, nil
	}

	return nil, errors.New("Unsupported format")
}
func (p Parser) ReadEntity(entity string) (Entity, error) {
	yamlFile, err := ioutil.ReadFile(entity)
	if err != nil {
		return nil, errors.Wrap(err, "Failed while reading entity file")
	}
	return p.ReadEntityFromBytes(yamlFile)

}
