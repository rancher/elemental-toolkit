/*
Copyright © 2021 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package elemental

import (
	"errors"
	"fmt"
	cnst "github.com/rancher-sandbox/elemental-cli/pkg/constants"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
	"github.com/zloylos/grsync"
	"net/http"
	"os"
	"strings"

	"github.com/mudler/yip/pkg/console"
	"github.com/twpayne/go-vfs"
)

// Elemental is the struct meant to self-contain most utils and actions related to Elemental, like installing or applying selinux
type Elemental struct {
	config *v1.RunConfig
}

func NewElemental(config *v1.RunConfig) *Elemental {
	return &Elemental{
		config: config,
	}
}

// PartitionAndFormatDevice creates a new empty partition table on target disk
// and applies the configured disk layout by creating and formatting all
// required partitions
func (c *Elemental) PartitionAndFormatDevice(disk *part.Disk) error {
	c.config.Logger.Infof("Partitioning device...")

	err := c.createPTableAndFirmwarePartitions(disk)
	if err != nil {
		return err
	}

	if c.config.PartTable == v1.GPT && c.config.PartLayout != "" {
		cloudInit := v1.CloudInitRunner(c.config.Logger)
		return cloudInit.Run(
			cnst.PartStage, vfs.OSFS,
			console.NewStandardConsole(console.WithLogger(c.config.Logger)),
		)
	}

	return c.createDataPartitions(disk)
}

func (c *Elemental) createPTableAndFirmwarePartitions(disk *part.Disk) error {
	errCMsg := "Failed creating %s partition"
	errFMsg := "Failed formatting partition: %s"

	c.config.Logger.Debugf("Creating partition table...")
	out, err := disk.NewPartitionTable(c.config.PartTable)
	if err != nil {
		c.config.Logger.Errorf("Failed creating new partition table: %s", out)
		return err
	}

	if c.config.PartTable == v1.GPT && c.config.BootFlag == v1.ESP {
		c.config.Logger.Debugf("Creating EFI partition...")
		efiNum, err := disk.AddPartition(cnst.EfiSize, cnst.EfiFs, cnst.EfiPLabel, v1.ESP)
		if err != nil {
			c.config.Logger.Errorf(errCMsg, cnst.EfiPLabel)
			return err
		}
		out, err = disk.FormatPartition(efiNum, cnst.EfiFs, cnst.EfiLabel)
		if err != nil {
			c.config.Logger.Errorf(errFMsg, out)
			return err
		}
	} else if c.config.PartTable == v1.GPT && c.config.BootFlag == v1.BIOS {
		c.config.Logger.Debugf("Creating Bios partition...")
		_, err = disk.AddPartition(cnst.BiosSize, cnst.BiosFs, cnst.BiosPLabel, v1.BIOS)
		if err != nil {
			c.config.Logger.Errorf(errCMsg, cnst.BiosPLabel)
			return err
		}
	}
	return nil
}

func (c *Elemental) createDataPartitions(disk *part.Disk) error {
	errCMsg := "Failed creating %s partition"
	errFMsg := "Failed formatting partition: %s"

	stateFlags := []string{}
	if c.config.PartTable == v1.MSDOS {
		stateFlags = append(stateFlags, v1.BOOT)
	}
	oemNum, err := disk.AddPartition(c.config.OEMPart.Size, c.config.OEMPart.FS, c.config.OEMPart.PLabel)
	if err != nil {
		c.config.Logger.Errorf(errCMsg, c.config.OEMPart.PLabel)
		return err
	}
	stateNum, err := disk.AddPartition(c.config.StatePart.Size, c.config.StatePart.FS, c.config.StatePart.PLabel, stateFlags...)
	if err != nil {
		c.config.Logger.Errorf(errCMsg, c.config.StatePart.PLabel)
		return err
	}
	recoveryNum, err := disk.AddPartition(c.config.RecoveryPart.Size, c.config.RecoveryPart.FS, c.config.RecoveryPart.PLabel)
	if err != nil {
		c.config.Logger.Errorf(errCMsg, cnst.RecoveryPLabel)
		return err
	}
	persistentNum, err := disk.AddPartition(c.config.PersistentPart.Size, c.config.PersistentPart.FS, c.config.PersistentPart.PLabel)
	if err != nil {
		c.config.Logger.Errorf(errCMsg, c.config.PersistentPart.PLabel)
		return err
	}

	out, err := disk.FormatPartition(oemNum, c.config.OEMPart.FS, c.config.OEMPart.Label)
	if err != nil {
		c.config.Logger.Errorf(errFMsg, out)
		return err
	}
	out, err = disk.FormatPartition(stateNum, c.config.StatePart.FS, c.config.StatePart.Label)
	if err != nil {
		c.config.Logger.Errorf(errFMsg, out)
		return err
	}
	out, err = disk.FormatPartition(recoveryNum, c.config.RecoveryPart.FS, c.config.RecoveryPart.Label)
	if err != nil {
		c.config.Logger.Errorf(errFMsg, out)
		return err
	}
	out, err = disk.FormatPartition(persistentNum, c.config.PersistentPart.FS, c.config.PersistentPart.Label)
	if err != nil {
		c.config.Logger.Errorf(errFMsg, out)
		return err
	}
	return nil
}

// CopyCos will rsync from config.source to config.target
func (c *Elemental) CopyCos() error {
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
func (c *Elemental) CopyCloudConfig() error {
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
func (c *Elemental) SelinuxRelabel(raiseError bool) error {
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
func (c Elemental) CheckNoFormat() error {
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
