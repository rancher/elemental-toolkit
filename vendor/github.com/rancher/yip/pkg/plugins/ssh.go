package plugins

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mauromorales/xpasswd/pkg/users"
	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/schema"
	"github.com/rancher/yip/pkg/utils"
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
		return "", fmt.Errorf("failed downloading key: %s", err.Error())
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
			return info, fmt.Errorf("cannot stat '%s': %s", file, err.Error())
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
			return fmt.Errorf("failed fetching ssh key: %s", err.Error())
		}
	}

	info, err := ensure(file, fs)
	if err != nil {
		return fmt.Errorf("failed ensuring %s exists: %s", file, err.Error())
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

	list := users.NewUserList()
	list.SetPath(f)
	err = list.Load()
	if err != nil {
		return fmt.Errorf("Failed parsing passwd: %s", err.Error())
	}

	data := list.Get(user)
	if data == nil {
		return fmt.Errorf("user %s not found", user)
	}

	uid, err := data.UID()
	if err != nil {
		return fmt.Errorf("Failed getting uid: %s", err.Error())
	}

	gid, err := data.GID()
	if err != nil {
		return fmt.Errorf("Failed getting gid: %s", err.Error())
	}

	homeDir := data.HomeDir()

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
