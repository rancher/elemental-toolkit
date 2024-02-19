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

package systemd

import (
	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
)

type Unit struct {
	Name string
}

func NewUnit(name string) *Unit {
	return &Unit{
		Name: name,
	}
}

func Enable(runner v2.Runner, unit *Unit) error {
	_, err := runner.Run("systemctl", "enable", unit.Name)
	return err
}

func Start(runner v2.Runner, unit *Unit) error {
	_, err := runner.Run("systemctl", "start", unit.Name)
	return err
}
