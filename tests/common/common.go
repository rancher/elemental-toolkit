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

package common

import "flag"

const defaultUpgradeImage = "ghcr.io/rancher/elemental-toolkit/elemental-green:v1.1.4"
const defaultToolkitImage = "ghcr.io/rancher/elemental-toolkit/elemental-cli:v1.1.4"

var upgradeImage string
var toolkitImage string

func init() {
	flag.StringVar(&upgradeImage, "upgrade-image", defaultUpgradeImage, "Default image to use in `upgrade` calls")
	flag.StringVar(&toolkitImage, "toolkit-image", defaultToolkitImage, "Default image to use when calling `upgrade`")
}

func UpgradeImage() string {
	return upgradeImage
}

func ToolkitImage() string {
	return toolkitImage
}
