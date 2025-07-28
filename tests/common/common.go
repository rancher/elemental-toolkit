/*
Copyright © 2022 - 2025 SUSE LLC

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

package common

import "flag"

const upImg = "ghcr.io/rancher/elemental-toolkit/elemental-green:v0.10.7-g3e4a3c56"

var upgradeImg string

func init() {
	flag.StringVar(&upgradeImg, "upgradeImg", upImg, "Default image to use in `upgrade` calls")
}

func UpgradeImage() string {
	return upgradeImg
}
