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

package utils

import (
	"fmt"
	"github.com/spf13/afero"
	"os/exec"
)


func SelinuxRelabel(target string, fs afero.Fs, raiseError bool) error {
	var err error

	contextFile := fmt.Sprintf("%s/etc/selinux/targeted/contexts/files/file_contexts", target)

	_, err = fs.Stat(contextFile)
	contextExists := err == nil

	if commandExists("setfiles") && contextExists {
		_, err = exec.Command("setfiles", "-r", target, contextFile, target).CombinedOutput()
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