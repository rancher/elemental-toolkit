package utils

import (
	"errors"
	"os"

	"github.com/twpayne/go-vfs"
)

func Touch(s string, perms os.FileMode, fs vfs.FS) error {
	_, err := fs.Stat(s)

	switch {
	case os.IsNotExist(err):
		f, err := fs.Create(s)
		if err != nil {
			return err
		}
		if err = f.Chmod(perms); err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		_, err = fs.Stat(s)
		return err
	case err == nil:
		return nil
	default:
		return errors.New("could not create file")
	}

}

func Exists(s string) bool {
	if _, err := os.Stat(s); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}
