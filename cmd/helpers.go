/*
Copyright © 2022 - 2026 SUSE LLC

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

package cmd

import (
	"os"

	errors "github.com/rancher/elemental-toolkit/v2/pkg/error"
)

// CheckRoot is a helper to return on PreRunE, so we can add it to commands that require root
func CheckRoot() error {
	if os.Geteuid() != 0 {
		return errors.New("this command requires root privileges", errors.RequiresRoot)
	}
	return nil
}
