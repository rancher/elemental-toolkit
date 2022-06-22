package plugins

import (
	"fmt"
	"os"
	osuser "os/user"
	"sort"
	"strconv"

	"github.com/pkg/errors"

	"github.com/hashicorp/go-multierror"
	"github.com/joho/godotenv"
	entities "github.com/mudler/entities/pkg/entities"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
	passwd "github.com/willdonnelly/passwd"
)

func createUser(fs vfs.FS, u schema.User, console Console) error {
	pass := u.PasswordHash
	if u.LockPasswd {
		pass = "!"
	}

	userShadow := &entities.Shadow{
		Username:    u.Name,
		Password:    pass,
		LastChanged: "now",
	}

	etcgroup, err := fs.RawPath("/etc/group")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/group")
	}

	etcshadow, err := fs.RawPath("/etc/shadow")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/shadow")
	}

	etcpasswd, err := fs.RawPath("/etc/passwd")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/passwd")
	}

	useradd, err := fs.RawPath("/etc/default/useradd")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/default/useradd")
	}

	// Set default home and shell
	usrDefaults := map[string]string{}
	usrDefaults["SHELL"] = "/bin/sh"
	usrDefaults["HOME"] = fmt.Sprintf("/home")

	// Load default home and shell from `/etc/default/useradd`
	if _, err = os.Stat(useradd); err == nil {
		usrDefaults, err = godotenv.Read(useradd)
		if err != nil {
			return errors.Wrapf(err, "could not parse '%s'", useradd)
		}
	}

	primaryGroup := u.Name
	gid := 1000

	if u.PrimaryGroup != "" {
		gr, err := osuser.LookupGroup(u.PrimaryGroup)
		if err != nil {
			return errors.Wrap(err, "could not resolve primary group of user")
		}
		gid, _ = strconv.Atoi(gr.Gid)
		primaryGroup = u.PrimaryGroup
	} else {
		// Create a new group after the user name
		all, _ := entities.ParseGroup(etcgroup)
		if len(all) != 0 {
			usedGids := []int{}
			for _, entry := range all {
				usedGids = append(usedGids, *entry.Gid)
			}
			sort.Ints(usedGids)
			if len(usedGids) == 0 {
				return errors.New("no new guid found")
			}
			gid = usedGids[len(usedGids)-1]
			gid++
		}

	}

	updateGroup := entities.Group{
		Name:     primaryGroup,
		Password: "x",
		Gid:      &gid,
		Users:    u.Name,
	}
	updateGroup.Apply(etcgroup, false)

	uid := 1000
	if u.UID != "" {
		// User defined-uid
		uid, err = strconv.Atoi(u.UID)
		if err != nil {
			return errors.Wrap(err, "invalid uid defined")
		}
	} else {
		// find an available uid if there are others already
		all, _ := passwd.ParseFile(etcpasswd)
		if len(all) != 0 {
			usedUids := []int{}
			for _, entry := range all {
				uid, _ := strconv.Atoi(entry.Uid)
				usedUids = append(usedUids, uid)
			}
			sort.Ints(usedUids)
			if len(usedUids) == 0 {
				return errors.New("no new UID found")
			}
			uid = usedUids[len(usedUids)-1]
			uid++
		}
	}

	if u.Homedir == "" {
		u.Homedir = fmt.Sprintf("%s/%s", usrDefaults["HOME"], u.Name)
	}

	if u.Shell == "" {
		u.Shell = usrDefaults["SHELL"]
	}

	userInfo := &entities.UserPasswd{
		Username: u.Name,
		Password: "x",
		Info:     u.GECOS,
		Homedir:  u.Homedir,
		Gid:      gid,
		Shell:    u.Shell,
		Uid:      uid,
	}

	if err := userInfo.Apply(etcpasswd, false); err != nil {
		return err
	}

	if err := userShadow.Apply(etcshadow, false); err != nil {
		return err
	}

	if !u.NoCreateHome {
		homedir, err := fs.RawPath(u.Homedir)
		if err != nil {
			return errors.Wrap(err, "getting rawpath for homedir")
		}
		os.MkdirAll(homedir, 0755)
		os.Chown(homedir, uid, gid)
	}

	groups, _ := entities.ParseGroup(etcgroup)
	for name, group := range groups {
		for _, w := range u.Groups {
			if w == name {
				group.Users = u.Name
				group.Apply(etcgroup, false)
			}
		}
	}

	return nil
}

func setUserPass(fs vfs.FS, username, password string) error {
	etcshadow, err := fs.RawPath("/etc/shadow")
	if err != nil {
		return errors.Wrap(err, "getting rawpath for /etc/shadow")
	}
	userShadow := &entities.Shadow{
		Username:    username,
		Password:    password,
		LastChanged: "now",
	}
	if err := userShadow.Apply(etcshadow, false); err != nil {
		return err
	}
	return nil
}

func User(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	for u, p := range s.Users {
		r := &p
		r.Name = u
		if !p.Exists() {
			if err := createUser(fs, *r, console); err != nil {
				errs = multierror.Append(errs, err)
			}
		} else if p.PasswordHash != "" {
			if err := setUserPass(fs, r.Name, r.PasswordHash); err != nil {
				return err
			}
		}

		if len(p.SSHAuthorizedKeys) > 0 {
			SSH(l, schema.Stage{SSHKeys: map[string][]string{r.Name: r.SSHAuthorizedKeys}}, fs, console)
		}

	}
	return errs
}
