package plugins

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/mudler/yip/pkg/utils"
	"github.com/pkg/errors"
	"github.com/twpayne/go-vfs"
)

const environmentFile = "/etc/environment"
const envFilePerm uint32 = 0644

func Environment(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	if len(s.Environment) == 0 {
		return nil
	}
	environment := s.EnvironmentFile
	if environment == "" {
		environment = environmentFile
	}

	parentDir := filepath.Dir(environment)
	_, err := fs.Stat(parentDir)
	if err != nil {
		perm := envFilePerm
		if perm < 0700 {
			perm = perm + 0100
		}
		if err = EnsureDirectories(l, schema.Stage{
			Directories: []schema.Directory{
				{
					Path:        parentDir,
					Permissions: perm,
					Owner:       os.Getuid(),
					Group:       os.Getgid(),
				},
			},
		}, fs, console); err != nil {
			return err
		}
	}

	if err := utils.Touch(environment, os.ModePerm, fs); err != nil {
		return errors.Wrap(err, "failed touching environment file")
	}

	content, err := fs.ReadFile(environment)
	if err != nil {
		return err
	}

	env, _ := godotenv.Unmarshal(string(content))
	for key, val := range s.Environment {
		env[key] = templateSysData(l, val)
	}

	p, err := fs.RawPath(environment)
	if err != nil {
		return err
	}
	err = godotenv.Write(env, p)
	if err != nil {
		return err
	}

	return fs.Chmod(environment, os.FileMode(envFilePerm))
}
