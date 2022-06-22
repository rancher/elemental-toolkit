package plugins

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/pkg/errors"
	"github.com/twpayne/go-vfs"
	passwd "github.com/willdonnelly/passwd"
)

const (
	sshDir         = ".ssh"
	authorizedFile = "authorized_keys"
	passwdFile     = "/etc/passwd"
)

var keyProviders = map[string]string{
	"github": "https://github.com/%s.keys",
	"gitlab": "https://gitlab.com/%s.keys",
}

func SSH(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error

	for u, keys := range s.SSHKeys {
		if err := ensureKeys(u, keys, fs); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

func getRemotePubKey(key string) (string, error) {
	url, err := url.Parse(key)
	if err != nil {
		return "", err
	}

	if providerURL, ok := keyProviders[url.Scheme]; ok {
		key = fmt.Sprintf(providerURL, url.Opaque)
	}

	out, err := download(key)
	if err != nil {
		return "", errors.Wrap(err, "failed while downloading key")
	}
	return out, err
}

func ensure(file string, fs vfs.FS) (os.FileInfo, error) {
	info, err := fs.Stat(file)
	if os.IsNotExist(err) {
		f, err := fs.Create(file)
		if err != nil {
			return info, err
		}
		if err = f.Chmod(0600); err != nil {
			return info, err
		}
		if err = f.Close(); err != nil {
			return info, err
		}
		info, err = fs.Stat(file)
		if err != nil {
			return info, errors.Wrapf(err, "cannot stat %s", file)
		}
	} else if err != nil {
		return info, err
	}

	return info, nil
}

func authorizeSSHKey(key, file string, uid, gid int, fs vfs.FS) error {
	var err error

	if utils.IsUrl(key) {
		key, err = getRemotePubKey(key)
		if err != nil {
			return errors.Wrap(err, "failed fetching ssh key")
		}
	}

	info, err := ensure(file, fs)
	if err != nil {
		return errors.Wrapf(err, "while ensuring %s exists", file)
	}

	bytes, err := fs.ReadFile(file)
	if err != nil {
		return err
	}

	// Don't do anything if key is already present
	if strings.Contains(string(bytes), key) {
		return nil
	}

	perm := info.Mode().Perm()

	bytes = append(bytes, []byte(key)...)
	bytes = append(bytes, '\n')

	if err = fs.WriteFile(file, bytes, perm); err != nil {
		return err
	}
	return fs.Chown(file, uid, gid)
}

func ensureKeys(user string, keys []string, fs vfs.FS) error {
	var errs error
	f, err := fs.RawPath(passwdFile)

	current, err := passwd.ParseFile(f)
	if err != nil {
		return errors.Wrap(err, "Failed parsing passwd")
	}

	data, ok := current[user]
	if !ok {
		return fmt.Errorf("user %s not found", user)
	}

	uid, err := strconv.Atoi(data.Uid)
	if err != nil {
		return errors.Wrap(err, "Failed getting uid")
	}

	gid, err := strconv.Atoi(data.Gid)
	if err != nil {
		return errors.Wrap(err, "Failed getting gid")
	}

	homeDir := data.Home

	userSSHDir := path.Join(homeDir, sshDir)
	if _, err := fs.Stat(userSSHDir); os.IsNotExist(err) {
		if err = fs.Mkdir(userSSHDir, 0700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if err = fs.Chown(userSSHDir, uid, gid); err != nil {
		errs = multierror.Append(errs, err)
	}

	userAuthorizedFile := path.Join(userSSHDir, authorizedFile)
	for _, key := range keys {
		if err = authorizeSSHKey(key, userAuthorizedFile, uid, gid, fs); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}
