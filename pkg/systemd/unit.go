/*
Copyright © 2022 - 2023 SUSE LLC

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

package systemd

import (
	"path/filepath"

	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
)

type Unit struct {
	Name    string
	Content []byte
}

func NewUnit(name string, content []byte) *Unit {
	return &Unit{
		Name:    name,
		Content: content,
	}
}

func Install(fs v1.FS, unit *Unit) error {
	return fs.WriteFile(filepath.Join("/usr/lib/systemd/system", unit.Name), unit.Content, 0644)
}

func Enable(runner v1.Runner, unit *Unit) error {
	_, err := runner.Run("systemctl", "enable", unit.Name)
	return err
}
