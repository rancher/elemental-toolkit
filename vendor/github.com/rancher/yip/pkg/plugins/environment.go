package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/twpayne/go-vfs/v4"

	"github.com/rancher/yip/pkg/logger"
	"github.com/rancher/yip/pkg/schema"
	"github.com/rancher/yip/pkg/utils"
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
		return fmt.Errorf("failed touching environment file: %s", err.Error())
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
