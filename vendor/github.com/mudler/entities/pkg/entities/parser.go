/*
Copyright © 2020 Ettore Di Giacinto <mudler@mocaccino.org>
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/


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
