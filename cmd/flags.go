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

package cmd

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/rancher/elemental-toolkit/v2/pkg/types"
)

// addCosignFlags adds flags related to cosign
func addCosignFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("cosign", false, "Enable cosign verification (requires images with signatures)")
	cmd.Flags().String("cosign-key", "", "Sets the URL of the public key to be used by cosign validation")
}

// addPowerFlags adds flags related to power
func addPowerFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("reboot", false, "Reboot the system after install")
	cmd.Flags().Bool("poweroff", false, "Shutdown the system after install")
}

// addSharedInstallUpgradeFlags add flags shared between install, upgrade and reset
func addSharedInstallUpgradeFlags(cmd *cobra.Command) {
	addResetFlags(cmd)
	addRecoverySystemFlag(cmd)
	addSquashFsCompressionFlags(cmd)
}

// addResetFlags add flags shared between reset, install and upgrade
func addResetFlags(cmd *cobra.Command) {
	cmd.Flags().String("directory", "", "Use directory as source to install from")
	_ = cmd.Flags().MarkDeprecated("directory", "'directory' is deprecated please use 'system' instead")

	cmd.Flags().StringP("docker-image", "d", "", "Install a specified container image")
	_ = cmd.Flags().MarkDeprecated("docker-image", "'docker-image' is deprecated please use 'system' instead")

	cmd.Flags().Bool("verify", false, "Enable mtree checksum verification (requires images manifests generated with mtree separately)")
	cmd.Flags().Bool("strict", false, "Enable strict check of hooks (They need to exit with 0)")

	addSnapshotLabelsFlag(cmd)
	addTLSVerifyFlag(cmd)
	addSystemFlag(cmd)
	addCosignFlags(cmd)
	addPowerFlags(cmd)
}

// addSystemFlag adds system flag to define source OS
func addSystemFlag(cmd *cobra.Command) {
	cmd.Flags().String("system", "", "Sets the system image source and its type (e.g. 'docker:registry.org/image:tag')")
}

// addRecoverySystemFlag adds the recovery-system.uri flag to define recovery source OS
func addRecoverySystemFlag(cmd *cobra.Command) {
	cmd.Flags().String("recovery-system.uri", "", "Sets the recovery image source and its type (e.g. 'docker:registry.org/image:tag')")
}

// addSnapshotLabelsFlag adds labels to be applied to any newly created snapshot, applies to install, reset, upgrade, and upgrade-recovery actions.
func addSnapshotLabelsFlag(cmd *cobra.Command) {
	cmd.Flags().StringToString("snapshot-labels", map[string]string{}, "Add labels to the to the system (ex. --snapshot-labels my-label=foo,my-other-label=bar)")
}

// addLocalImageFlag add local image flag shared between install, pull-image, upgrade
func addLocalImageFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("local", false, "Use an image from local cache")
}

// addVerifyRegistryFlag add local image flag shared between install, pull-image, upgrade
func addTLSVerifyFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("tls-verify", true, "Require HTTPS and verify certificates of registries (default: true)")
}

func adaptDockerImageAndDirectoryFlagsToSystem(flags *pflag.FlagSet) {
	systemFlag := "system"
	doc, _ := flags.GetString("docker-image")
	if doc != "" {
		_ = flags.Set(systemFlag, fmt.Sprintf("docker:%s", doc))
	}
	dir, _ := flags.GetString("directory")
	if dir != "" {
		_ = flags.Set(systemFlag, fmt.Sprintf("dir:%s", dir))
	}
}

func validateCosignFlags(log types.Logger, flags *pflag.FlagSet) error {
	cosignKey, _ := flags.GetString("cosign-key")
	cosign, _ := flags.GetBool("cosign")

	if cosignKey != "" && !cosign {
		return errors.New("'cosign-key' requires 'cosign' option to be enabled")
	}

	if cosign && cosignKey == "" {
		log.Warnf("No 'cosign-key' option set, keyless cosign verification is experimental")
	}
	return nil
}

func validateSourceFlags(_ types.Logger, flags *pflag.FlagSet) error {
	msg := "flags docker-image, directory and system are mutually exclusive, please only set one of them"
	system, _ := flags.GetString("system")
	directory, _ := flags.GetString("directory")
	dockerImg, _ := flags.GetString("docker-image")
	// docker-image, directory and system are mutually exclusive. Can't have your cake and eat it too.
	if system != "" && (directory != "" || dockerImg != "") {
		return errors.New(msg)
	}
	if directory != "" && dockerImg != "" {
		return errors.New(msg)
	}
	return nil
}

func validatePowerFlags(_ types.Logger, flags *pflag.FlagSet) error {
	reboot, _ := flags.GetBool("reboot")
	poweroff, _ := flags.GetBool("poweroff")
	if reboot && poweroff {
		return errors.New("'reboot' and 'poweroff' are mutually exclusive options")
	}
	return nil
}

// validateUpgradeFlags is a helper call to check all the flags for the upgrade command
func validateInstallUpgradeFlags(log types.Logger, flags *pflag.FlagSet) error {
	if err := validateSourceFlags(log, flags); err != nil {
		return err
	}
	if err := validateCosignFlags(log, flags); err != nil {
		return err
	}
	return validatePowerFlags(log, flags)
}

// addPlatformFlags adds the arch flag for build commands
func addPlatformFlags(cmd *cobra.Command) {
	cmd.Flags().String("arch", "", "Arch to build the image for")
	_ = cmd.Flags().MarkDeprecated("arch", "'arch' is deprecated please use 'platform' instead")
	cmd.Flags().String("platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), "Platform to build the image for")
}

type enum struct {
	Allowed []string
	Value   string
}

// newEnum give a list of allowed flag parameters, where the second argument is the default
func newEnumFlag(allowed []string, d string) *enum {
	return &enum{
		Allowed: allowed,
		Value:   d,
	}
}

func (a enum) String() string {
	return a.Value
}

func (a *enum) Set(p string) error {
	isIncluded := func(opts []string, val string) bool {
		for _, opt := range opts {
			if val == opt {
				return true
			}
		}
		return false
	}
	if !isIncluded(a.Allowed, p) {
		return fmt.Errorf("'%s' is not included in: %s", p, strings.Join(a.Allowed, ","))
	}
	a.Value = p
	return nil
}

func (a *enum) Type() string {
	return "string"
}

func addSquashFsCompressionFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayP("squash-compression", "x", []string{}, "cmd options for compression to pass to mksquashfs. Full cmd including --comp as the whole values will be passed to mksquashfs. For a full list of options please check mksquashfs manual. (default value: '-comp xz -Xbcj ARCH')")
	cmd.Flags().Bool("squash-no-compression", false, "Disable squashfs compression. Overrides any values on squash-compression")
}
