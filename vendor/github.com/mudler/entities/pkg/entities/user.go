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
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	xusers "github.com/mauromorales/xpasswd/pkg/users"
	permbits "github.com/phayes/permbits"
	"github.com/pkg/errors"

	"github.com/gofrs/flock"
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
	list := xusers.NewUserList()
	list.SetPath(path)
	err := list.Load()
	if err != nil {
		return 0, errors.Wrap(err, "Failed parsing passwd")
	}

	id, err := list.GenerateUIDInRange(HumanIDMin, HumanIDMax)
	if err != nil {
		return 0, errors.Wrap(err, "Failed generating a unique uid")
	}

	return id, nil
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

	list := xusers.NewUserList()
	list.SetPath(path)
	users, err := list.GetAll()
	if err != nil && len(users) == 0 {
		return ans, errors.Wrap(err, "Failed loading user list")
	}

	_, err = permbits.Stat(path)
	if err != nil {
		return ans, errors.Wrap(err, "Failed getting permissions")
	}

	for _, user := range users {
		username := user.Username()
		uid, err := user.UID()
		if err != nil {
			fmt.Println(fmt.Sprintf(
				"WARN: Found invalid uid for user %s: %s.\nSetting 0. Check the file soon.",
				username, err.Error(),
			))
			uid = 0
		}

		gid, err := user.GID()
		if err != nil {
			fmt.Println(fmt.Sprintf(
				"WARN: Found invalid gid for user %s and uid %d: %s",
				username, uid, err.Error(),
			))
			// Set gid with the same value of uid
			gid = uid
		}

		ans[username] = UserPasswd{
			Username: username,
			Password: user.Password(),
			Uid:      uid,
			Gid:      gid,
			Info:     user.RealName(),
			Homedir:  user.HomeDir(),
			Shell:    user.Shell(),
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
	d, err := RetryForDuration()
	if err != nil {
		return errors.Wrap(err, "Failed getting delay")
	}

	baseName := filepath.Base(s)
	fileLock := flock.New(fmt.Sprintf("/var/lock/%s.lock", baseName))
	defer os.Remove(fileLock.Path())
	defer fileLock.Close()

	lockCtx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	i, err := RetryIntervalDuration()
	if err != nil {
		return errors.Wrap(err, "Failed getting interval")
	}
	locked, err := fileLock.TryLockContext(lockCtx, i)
	if err != nil || !locked {
		return errors.Wrap(err, "Failed locking file")
	}

	input, err := os.ReadFile(s)
	if err != nil {
		return errors.Wrap(err, "Could not read input file")
	}
	permissions, err := permbits.Stat(s)
	if err != nil {
		return errors.Wrap(err, "Failed getting permissions")
	}
	lines := bytes.Replace(input, []byte(u.String()+"\n"), []byte(""), 1)

	err = os.WriteFile(s, []byte(lines), os.FileMode(permissions))
	if err != nil {
		return errors.Wrap(err, "Could not write")
	}

	return nil
}

func (u UserPasswd) Create(s string) error {
	s = UserDefault(s)
	d, err := RetryForDuration()
	if err != nil {
		return errors.Wrap(err, "Failed getting delay")
	}

	baseName := filepath.Base(s)
	fileLock := flock.New(fmt.Sprintf("/var/lock/%s.lock", baseName))
	defer os.Remove(fileLock.Path())
	defer fileLock.Close()
	lockCtx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	i, err := RetryIntervalDuration()
	if err != nil {
		return errors.Wrap(err, "Failed getting interval")
	}
	locked, err := fileLock.TryLockContext(lockCtx, i)
	if err != nil || !locked {
		return errors.Wrap(err, "Failed locking file")
	}

	u, err = u.prepare(s)
	if err != nil {
		return errors.Wrap(err, "Failed entity preparation")
	}

	list := xusers.NewUserList()
	list.SetPath(s)
	err = list.Load()
	if err != nil {
		return errors.Wrap(err, "Failed parsing passwd")
	}

	user := list.Get(u.Username)
	if user != nil {
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
		d, err := RetryForDuration()
		if err != nil {
			return errors.Wrap(err, "Failed getting delay")
		}

		baseName := filepath.Base(s)
		fileLock := flock.New(fmt.Sprintf("/var/lock/%s.lock", baseName))
		defer os.Remove(fileLock.Path())
		defer fileLock.Close()
		lockCtx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()

		i, err := RetryIntervalDuration()
		if err != nil {
			return errors.Wrap(err, "Failed getting interval")
		}
		locked, err := fileLock.TryLockContext(lockCtx, i)
		if err != nil || !locked {
			return errors.Wrap(err, "Failed locking file")
		}

		input, err := os.ReadFile(s)
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
		err = os.WriteFile(s, []byte(output), os.FileMode(permissions))
		if err != nil {
			return errors.Wrap(err, "Could not write")
		}

	} else {
		// Add it
		return u.Create(s)
	}

	return nil
}
