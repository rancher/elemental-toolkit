package plugins

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/twpayne/go-vfs/v4"
	passwd "github.com/willdonnelly/passwd"

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

	current, err := passwd.ParseFile(f)
	if err != nil {
		return fmt.Errorf("Failed parsing passwd: %s", err.Error())
	}

	data, ok := current[user]
	if !ok {
		return fmt.Errorf("user %s not found", user)
	}

	uid, err := strconv.Atoi(data.Uid)
	if err != nil {
		return fmt.Errorf("Failed getting uid: %s", err.Error())
	}

	gid, err := strconv.Atoi(data.Gid)
	if err != nil {
		return fmt.Errorf("Failed getting gid: %s", err.Error())
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
