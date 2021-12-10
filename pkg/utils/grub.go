package utils

import (
	"fmt"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/afero"
	"runtime"
	"strings"
)

type Grub struct {
	disk string
	config v1.RunConfig
	fs afero.Fs
	runner v1.Runner
}

func NewGrub(config v1.RunConfig, opts...GrubOptions) *Grub{
	g := &Grub{
		config: config,
		fs: afero.NewOsFs(),
		runner: &v1.RealRunner{},
	}

	for _, o := range opts {
		err := o(g)
		if err != nil {
			return nil
		}
	}

	return g
}

func (g Grub) Install()  error {
	var grubargs []string
	var arch, grubdir, tty, finalContent string

	switch runtime.GOARCH {
	case "arm64":
		arch = "arm64"
	default:
		arch = "x86_64"
	}
	g.config.Logger.Info("Installing GRUB..")

	if g.config.Tty == "" {
		// Get current tty and remove /dev/ from its name
		out, err := g.runner.Run("tty")
		tty = strings.TrimPrefix(strings.TrimSpace(string(out)), "/dev/")
		if err != nil { return err }
	} else {
		tty = g.config.Tty
	}

	// In the original script, errors from here get ignored...
	// TODO: Check if we should fail on dir creation failure
	_ = g.fs.Mkdir(fmt.Sprintf("%s/proc", g.config.Target), 0644)
	_ = g.fs.Mkdir(fmt.Sprintf("%s/dev", g.config.Target), 0644)
	_ = g.fs.Mkdir(fmt.Sprintf("%s/sys", g.config.Target), 0644)
	_ = g.fs.Mkdir(fmt.Sprintf("%s/tmp", g.config.Target), 0644)

	efiExists, _ := afero.Exists(g.fs, "/sys/firmware/efi")

	if g.config.ForceEfi || efiExists {
		g.config.Logger.Infof("Installing grub efi for arch %s", arch)
		grubargs = append(
			grubargs,
			fmt.Sprintf("--target=%s-efi", arch),
			fmt.Sprintf("--efi-directory=%s/boot/efi", g.config.Target),
			)
	}

	// TODO: May be empty, this should be move to RunConfig init once we have struct init in there
	if g.config.StateDir == "" { g.config.StateDir = "/run/initramfs/cos-state" }

	grubargs = append(
		grubargs,
		fmt.Sprintf("--root-directory=%s", g.config.StateDir),
		fmt.Sprintf("--boot-directory=%s", g.config.StateDir),
		fmt.Sprintf("--removable=%s", g.config.Device),
		)

	g.config.Logger.Debugf("Running grub with the following args: %s", grubargs)
	_, err := g.runner.Run("grub2-install", grubargs...)
	if err != nil { return err }


	grub1dir := fmt.Sprintf("%s/grub", g.config.StateDir)
	grub2dir := fmt.Sprintf("%s/grub2", g.config.StateDir)

	// Select the proper dir for grub
	if ok, _ := afero.IsDir(g.fs, grub1dir); ok { grubdir = grub1dir }
	if ok, _ := afero.IsDir(g.fs, grub2dir); ok { grubdir = grub2dir }
	g.config.Logger.Infof("Found grub config dir %s", grubdir)

	// TODO: May be empty, this should be move to RunConfig init once we have struct init in there
	if g.config.GrubConf == "" { g.config.GrubConf = "/etc/cos/grub.cfg" }

	grubConf, err := afero.ReadFile(g.fs, g.config.GrubConf)

	grubConfTarget, err := g.fs.Create(fmt.Sprintf("%s/grub.cfg", grubdir))
	defer grubConfTarget.Close()

	ttyExists, _ := afero.Exists(g.fs, fmt.Sprintf("/dev/%s", tty))

	if ttyExists && tty != "" && tty != "console" && tty != "tty1" {
		// We need to add a tty to the grub file
		g.config.Logger.Infof("Adding extra tty (%s) to grub.cfg", tty)
		finalContent = strings.Replace(string(grubConf), "console=tty1", fmt.Sprintf("console=tty1 console=%s", tty), -1)
	} else {
		// We don't add anything, just read the file
		finalContent = string(grubConf)
	}

	g.config.Logger.Infof("Copying grub contents from %s to %s", g.config.GrubConf, fmt.Sprintf("%s/grub.cfg", grubdir))
	_, err = grubConfTarget.WriteString(finalContent)
	if err != nil { return err }

	g.config.Logger.Infof("Grub install to device %s complete", g.config.Device)
	return nil
}