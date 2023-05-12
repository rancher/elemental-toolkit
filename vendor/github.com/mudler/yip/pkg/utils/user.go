package utils

import (
	osuser "os/user"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// GetUserDataFromString retrieves uid and gid from a string. The string
// have to be in the "user:group" syntax, or just "user".
func GetUserDataFromString(s string) (int, int, error) {
	user := s
	var u, g string
	if strings.Contains(s, ":") {
		dat := strings.Split(user, ":")
		us, err := osuser.Lookup(dat[0])
		if err != nil {
			return 0, 0, errors.Wrapf(err, "while looking up user %s", dat[0])
		}
		u = us.Uid

		group, err := osuser.LookupGroup(dat[1])
		if err != nil {
			return 0, 0, errors.Wrapf(err, "while looking up group %s", dat[1])
		}
		g = group.Gid
	} else {
		us, err := osuser.Lookup(s)
		if err != nil {
			return 0, 0, errors.Wrapf(err, "while looking up user %s", s)
		}
		u = us.Uid
		g = us.Gid
	}

	uid, err := strconv.Atoi(u)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed converting uid to int")
	}

	gid, err := strconv.Atoi(g)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed converting gid to int")
	}
	return uid, gid, nil
}
