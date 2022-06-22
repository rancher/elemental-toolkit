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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	permbits "github.com/phayes/permbits"
	"github.com/pkg/errors"
	passwd "github.com/willdonnelly/passwd"
)

func UserDefault(s string) string {
	if s == "" {
		// Check environment override before to use default.
		s = os.Getenv(ENTITY_ENV_DEF_PASSWD)
		if s == "" {
			s = "/etc/passwd"
		}
	}
	return s
}

func userGetFreeUid(path string) (int, error) {
	uidStart, uidEnd := DynamicRange()
	mUids := make(map[int]*UserPasswd)
	ans := -1

	current, err := ParseUser(path)
	if err != nil {
		return ans, err
	}

	for _, e := range current {
		mUids[e.Uid] = &e
	}

	for i := uidStart; i >= uidEnd; i-- {
		if _, ok := mUids[i]; !ok {
			ans = i
			break
		}
	}

	if ans < 0 {
		return ans, errors.New("No free UID found")
	}

	return ans, nil
}

type UserPasswd struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Uid      int    `yaml:"uid"`
	Gid      int    `yaml:"gid"`
	Group    string `yaml:"group"`
	Info     string `yaml:"info"`
	Homedir  string `yaml:"homedir"`
	Shell    string `yaml:"shell"`
}

func ParseUser(path string) (map[string]UserPasswd, error) {
	ans := make(map[string]UserPasswd, 0)

	current, err := passwd.ParseFile(path)
	if err != nil {
		return ans, errors.Wrap(err, "Failed parsing passwd")
	}
	_, err = permbits.Stat(path)
	if err != nil {
		return ans, errors.Wrap(err, "Failed getting permissions")
	}

	for k, v := range current {

		uid, err := strconv.Atoi(v.Uid)
		if err != nil {
			fmt.Println(fmt.Sprintf(
				"WARN: Found invalid uid for user %s: %s.\nSetting 0. Check the file soon.",
				k, err.Error(),
			))
			uid = 0
		}

		gid, err := strconv.Atoi(v.Gid)
		if err != nil {
			fmt.Println(fmt.Sprintf(
				"WARN: Found invalid gid for user %s and uid %d: %s",
				k, uid, err.Error(),
			))
			// Set gid with the same value of uid
			gid = uid
		}

		ans[k] = UserPasswd{
			Username: k,
			Password: v.Pass,
			Uid:      uid,
			Gid:      gid,
			Info:     v.Gecos,
			Homedir:  v.Home,
			Shell:    v.Shell,
		}
	}

	return ans, nil
}

func (u UserPasswd) GetKind() string { return UserKind }

func (u UserPasswd) prepare(s string) (UserPasswd, error) {

	if u.Uid < 0 {
		// POST: dynamic user

		uid, err := userGetFreeUid(s)
		if err != nil {
			return u, err
		}
		u.Uid = uid
	}

	if u.Group != "" {
		// POST: gid must be retrieved by existing file.
		mGroups, err := ParseGroup(GroupsDefault(""))
		if err != nil {
			return u, errors.Wrap(err, "Error on retrieve group information")
		}

		g, ok := mGroups[u.Group]
		if !ok {
			return u, errors.Wrap(err, fmt.Sprintf("The group %s is not present", u.Group))
		}

		u.Gid = *g.Gid
		// Avoid this operation if prepare is called multiple times.
		u.Group = ""
	}

	if u.Info == "" {
		u.Info = "Created by entities"
	}

	return u, nil
}

func (u UserPasswd) String() string {
	return strings.Join([]string{u.Username,
		u.Password,
		strconv.Itoa(u.Uid),
		strconv.Itoa(u.Gid),
		u.Info,
		u.Homedir,
		u.Shell,
	}, ":")
}

func (u UserPasswd) Delete(s string) error {
	s = UserDefault(s)
	input, err := ioutil.ReadFile(s)
	if err != nil {
		return errors.Wrap(err, "Could not read input file")
	}
	permissions, err := permbits.Stat(s)
	if err != nil {
		return errors.Wrap(err, "Failed getting permissions")
	}
	lines := bytes.Replace(input, []byte(u.String()+"\n"), []byte(""), 1)

	err = ioutil.WriteFile(s, []byte(lines), os.FileMode(permissions))
	if err != nil {
		return errors.Wrap(err, "Could not write")
	}

	return nil
}

func (u UserPasswd) Create(s string) error {
	s = UserDefault(s)

	u, err := u.prepare(s)
	if err != nil {
		return errors.Wrap(err, "Failed entity preparation")
	}

	current, err := passwd.ParseFile(s)
	if err != nil {
		return errors.Wrap(err, "Failed parsing passwd")
	}
	if _, ok := current[u.Username]; ok {
		return errors.New("Entity already present")
	}
	permissions, err := permbits.Stat(s)
	if err != nil {
		return errors.Wrap(err, "Failed getting permissions")
	}
	f, err := os.OpenFile(s, os.O_APPEND|os.O_WRONLY, os.FileMode(permissions))
	if err != nil {
		return errors.Wrap(err, "Could not read")
	}

	defer f.Close()

	if _, err = f.WriteString(u.String() + "\n"); err != nil {
		return errors.Wrap(err, "Could not write")
	}
	return nil
}

func (u UserPasswd) Apply(s string, safe bool) error {
	if u.Username == "" {
		return errors.New("Empty username field")
	}

	s = UserDefault(s)

	u, err := u.prepare(s)
	if err != nil {
		return errors.Wrap(err, "Failed entity preparation")
	}

	current, err := ParseUser(s)
	if err != nil {
		return err
	}

	permissions, err := permbits.Stat(s)
	if err != nil {
		return errors.Wrap(err, "Failed getting permissions")
	}

	if safe {
		mUids := make(map[int]*UserPasswd)

		// Create uids map to check uid mismatch
		// Maybe could be done always
		for _, e := range current {
			mUids[e.Uid] = &e
		}

		if e, present := mUids[u.Uid]; present {
			if e.Username != u.Username {
				return errors.Wrap(err,
					fmt.Sprintf("Uid %d is already used on user %s",
						u.Uid, e.Username))
			}
		}
	}

	if _, ok := current[u.Username]; ok {

		input, err := ioutil.ReadFile(s)
		if err != nil {
			return errors.Wrap(err, "Could not read input file")
		}

		lines := strings.Split(string(input), "\n")

		for i, line := range lines {
			if entityIdentifier(line) == u.Username {
				if !safe {
					lines[i] = u.String()
				}
			}
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(s, []byte(output), os.FileMode(permissions))
		if err != nil {
			return errors.Wrap(err, "Could not write")
		}

	} else {
		// Add it
		return u.Create(s)
	}

	return nil
}
