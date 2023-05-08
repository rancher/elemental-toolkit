package common

import "flag"

const upImg = "ghcr.io/davidcassany/elemental-green:v0.10.6-66-g85c4bde8"

var upgradeImg string

func init() {
	flag.StringVar(&upgradeImg, "upgradeImg", upImg, "Default image to use in `upgrade` calls")
}

func UpgradeImage() string {
	return upgradeImg
}
