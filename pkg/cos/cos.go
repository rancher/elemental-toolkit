package cos

import (
	"errors"
	"fmt"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
	"github.com/zloylos/grsync"
	"net/http"
	"os"
	"strings"
)

// Cos is the struct meant to self-contain most utils and actions related to cos, like installing or applying selinux
type Cos struct {
	config *v1.RunConfig
}

func NewCos(config *v1.RunConfig) *Cos {
	return &Cos{
		config: config,
	}
}

// CopyCos will rsync from config.source to config.target
func (c *Cos) CopyCos() error {
	c.config.Logger.Infof("Copying cOS..")
	// Make sure the values have a / at the end.
	var source, target string
	if strings.HasSuffix(c.config.Source, "/") == false {
		source = fmt.Sprintf("%s/", c.config.Source)
	} else {
		source = c.config.Source
	}

	if strings.HasSuffix(c.config.Target, "/") == false {
		target = fmt.Sprintf("%s/", c.config.Target)
	} else {
		target = c.config.Target
	}

	// 1 - rsync all the system from source to target
	task := grsync.NewTask(
		source,
		target,
		grsync.RsyncOptions{
			Quiet:   false,
			Archive: true,
			XAttrs:  true,
			ACLs:    true,
			Exclude: []string{"mnt", "proc", "sys", "dev", "tmp"},
		},
	)

	if err := task.Run(); err != nil {
		return err
	}
	c.config.Logger.Infof("Finished copying cOS..")
	return nil
}

// CopyCloudConfig will check if there is a cloud init in the config and store it on the target
func (c *Cos) CopyCloudConfig() error {
	if c.config.CloudInit != "" {
		client := &http.Client{}
		customConfig := fmt.Sprintf("%s/oem/99_custom.yaml", c.config.Target)
		c.config.Logger.Infof("Trying to copy cloud config file %s to %s", c.config.CloudInit, customConfig)

		if err :=
			utils.GetUrl(client, c.config.Logger, c.config.CloudInit, customConfig); err != nil {
			return err
		}

		if err := os.Chmod(customConfig, 0600); err != nil {
			return err
		}
		c.config.Logger.Infof("Finished copying cloud config file to %s", c.config.CloudInit, customConfig)
	}
	return nil
}

// SelinuxRelabel will relabel the system if it finds the binary and the context
func (c *Cos) SelinuxRelabel(raiseError bool) error {
	var err error

	contextFile := fmt.Sprintf("%s/etc/selinux/targeted/contexts/files/file_contexts", c.config.Target)

	_, err = c.config.Fs.Stat(contextFile)
	contextExists := err == nil

	if utils.CommandExists("setfiles") && contextExists {
		_, err = c.config.Runner.Run("setfiles", "-r", c.config.Target, contextFile, c.config.Target)
	}

	// In the original code this can error out and we dont really care
	// I guess that to maintain backwards compatibility we have to do the same, we dont care if it raises an error
	// but we still add the possibility to return an error if we want to change it in the future to be more strict?
	if raiseError && err != nil {
		return err
	} else {
		return nil
	}
}

// CheckNoFormat will make sure that if we set the no format option, the system doesnt already contain a cos system
// by checking the active/passive labels. If they are set then we check if we have the force flag, which means that we
// don't care and proceed to overwrite
func (c Cos) CheckNoFormat() error {
	if c.config.NoFormat {
		// User asked for no format, lets check if there is already those labeled partitions in the disk
		for _, label := range []string{c.config.ActiveLabel, c.config.PassiveLabel} {
			found, err := utils.FindLabel(c.config.Runner, label)
			if err != nil {
				return err
			}
			if found != "" {
				if c.config.Force {
					msg := fmt.Sprintf("Forcing overwrite of existing partitions due to `force` flag")
					c.config.Logger.Infof(msg)
					return nil
				} else {
					msg := fmt.Sprintf("There is already an active deployment in the system, use '--force' flag to overwrite it")
					c.config.Logger.Error(msg)
					return errors.New(msg)
				}
			}
		}
	}
	return nil
}
