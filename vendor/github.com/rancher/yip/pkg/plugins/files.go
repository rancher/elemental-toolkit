package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/schema"
	"github.com/rancher/yip/pkg/utils"
)

func EnsureFiles(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	for _, file := range s.Files {
		if err := writeFile(l, file, fs, console); err != nil {
			l.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}

func writeFile(l logger.Interface, file schema.File, fs vfs.FS, console Console) error {
	l.Debug("Creating file ", file.Path)
	parentDir := filepath.Dir(file.Path)
	_, err := fs.Stat(parentDir)
	if err != nil {
		l.Debug("Creating parent directories")
		perm := file.Permissions
		if perm < 0700 {
			l.Debug("Adding execution bit to parent directory")
			perm = perm + 0100
		}
		if err = EnsureDirectories(l, schema.Stage{
			Directories: []schema.Directory{
				{
					Path:        parentDir,
					Permissions: perm,
					Owner:       file.Owner,
					Group:       file.Group,
				},
			},
		}, fs, console); err != nil {
			l.Infof("Failed to write %s: %s", parentDir, err)
			return err
		}
	}
	fsfile, err := fs.Create(file.Path)
	if err != nil {
		return err
	}
	defer fsfile.Close()

	d := newDecoder(file.Encoding)
	c, err := d.Decode(file.Content)
	if err != nil {
		return fmt.Errorf("failed decoding content with encoding %s: %s", file.Encoding, err.Error())
	}

	_, err = fsfile.WriteString(templateSysData(l, string(c)))
	if err != nil {
		return err

	}
	err = fs.Chmod(file.Path, os.FileMode(file.Permissions))
	if err != nil {
		return err

	}

	if file.OwnerString != "" {
		// FIXUP: Doesn't support fs. It reads real /etc/passwd files
		uid, gid, err := utils.GetUserDataFromString(file.OwnerString)
		if err != nil {
			return errors.New("Failed getting gid")
		}
		return fs.Chown(file.Path, uid, gid)
	}

	return fs.Chown(file.Path, file.Owner, file.Group)
}
