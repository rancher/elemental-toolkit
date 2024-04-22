/*
Copyright © 2022 - 2024 SUSE LLC

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

package partitioner

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

func EncryptDevice(runner types.Runner, device, mappedName string, slots []types.KeySlot) error {
	logger := runner.GetLogger()

	if len(slots) == 0 {
		return fmt.Errorf("Needs at least 1 key-slot to encrypt %s", device)
	}

	firstSlot := slots[0]

	cmd := runner.InitCmd("cryptsetup", "luksFormat", "--key-slot", fmt.Sprintf("%d", firstSlot.Slot), device, "-")
	err := unlockCmd(cmd, firstSlot)
	if err != nil {
		logger.Errorf("Error generating unlock command for device '%s': %s", device, err.Error())
		return err
	}

	stdout, err := runner.RunCmd(cmd)
	if err != nil {
		logger.Errorf("Error formatting device %s: %s", device, stdout)
		return err
	}

	cmd = runner.InitCmd("cryptsetup", "open", device, mappedName)

	if err = unlockCmd(cmd, firstSlot); err != nil {
		return err
	}

	stdout, err = runner.RunCmd(cmd)
	if err != nil {
		logger.Errorf("Error opening device %s: %s", device, stdout)
	}

	return err
}

func unlockCmd(cmd *exec.Cmd, slot types.KeySlot) error {
	if slot.Passphrase != "" {
		cmd.Stdin = strings.NewReader(string(slot.Passphrase))
		return nil
	}

	if slot.KeyFile != "" {
		cmd.Args = append(cmd.Args, "--key-file", slot.KeyFile)
		return nil
	}

	return errors.New("Unknown key slot authorization")
}
