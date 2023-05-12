/*
Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
                 Daniele Rondina <geaaru@sabayon.org>

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
	"errors"
	"io/ioutil"
	"path/filepath"
	"regexp"
)

type EntitiesStore struct {
	Users    map[string]UserPasswd
	Groups   map[string]Group
	Shadows  map[string]Shadow
	GShadows map[string]GShadow
}

func NewEntitiesStore() *EntitiesStore {
	return &EntitiesStore{
		Users:    make(map[string]UserPasswd, 0),
		Groups:   make(map[string]Group, 0),
		Shadows:  make(map[string]Shadow, 0),
		GShadows: make(map[string]GShadow, 0),
	}
}

func (s *EntitiesStore) Load(dir string) error {
	var regexConfs = regexp.MustCompile(`.yml$|.yaml$`)

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	p := &Parser{}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !regexConfs.MatchString(file.Name()) {
			continue
		}

		entity, err := p.ReadEntity(filepath.Join(dir, file.Name()))
		if err == nil {
			s.AddEntity(entity)
		}

	}

	return nil
}

func (s *EntitiesStore) AddEntity(e Entity) error {
	var err error
	switch e.GetKind() {
	case UserKind:
		err = s.AddUser((e.(UserPasswd)))
	case GroupKind:
		err = s.AddGroup((e.(Group)))
	case ShadowKind:
		err = s.AddShadow((e.(Shadow)))
	case GShadowKind:
		err = s.AddGShadow((e.(GShadow)))
	default:
		err = errors.New("Invalid entity")
	}

	return err
}

func (s *EntitiesStore) AddUser(u UserPasswd) error {
	if u.Username == "" {
		return errors.New("Invalid username field")
	}
	s.Users[u.Username] = u
	return nil
}

func (s *EntitiesStore) AddGroup(g Group) error {
	if g.Name == "" {
		return errors.New("Invalid group name field")
	}

	s.Groups[g.Name] = g
	return nil
}

func (s *EntitiesStore) AddShadow(e Shadow) error {
	if e.Username == "" {
		return errors.New("Invalid username field")
	}

	s.Shadows[e.Username] = e
	return nil
}

func (s *EntitiesStore) AddGShadow(e GShadow) error {
	if e.Name == "" {
		return errors.New("Invalid name field")
	}

	s.GShadows[e.Name] = e
	return nil
}

func (s *EntitiesStore) GetShadow(name string) (Shadow, bool) {
	if e, ok := s.Shadows[name]; ok {
		return e, true
	} else {
		return Shadow{}, false
	}
}

func (s *EntitiesStore) GetGShadow(name string) (GShadow, bool) {
	if e, ok := s.GShadows[name]; ok {
		return e, true
	} else {
		return GShadow{}, false
	}
}

func (s *EntitiesStore) GetUser(name string) (UserPasswd, bool) {
	if e, ok := s.Users[name]; ok {
		return e, true
	} else {
		return UserPasswd{}, false
	}
}

func (s *EntitiesStore) GetGroup(name string) (Group, bool) {
	if e, ok := s.Groups[name]; ok {
		return e, true
	} else {
		return Group{}, false
	}
}
