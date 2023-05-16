/*
Copyright © 2022 - 2023 SUSE LLC

Copyright © 2015-2017 Docker, Inc.

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

package providers

import (
	"os"
)

type FileProvider string

func (p FileProvider) String() string {
	return string(p)
}

func (p FileProvider) Probe() bool {
	_, err := os.Stat(string(p))
	return err == nil
}

func (p FileProvider) Extract() ([]byte, error) {
	return os.ReadFile(string(p))
}
