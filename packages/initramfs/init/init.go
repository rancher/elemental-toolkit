package main

import (
	"github.com/u-root/u-root/pkg/libinit"
	"github.com/u-root/u-root/pkg/ulog"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type initCmds struct {
	cmds []*exec.Cmd
}

var (
	debug   = func(string, ...interface{}) {}
	modules = []string{
		"af_packet", "e1000e", "e1000", "dm_mod",
		"ahci", "virtio_blk", "virtio_pci", "pata_acpi", "ahcpi-plaftorm", "libahcpi-platform", "ata_piix",
		"ohci_pci", "ehci_pci", "loop", "ext4", "isofs", "squashfs",
		"ata_generic", "cdrom", "sd_mod", "sr_mod", "ext2", "uas", "usb_storage", "usbcore", "paride",
		"scsi_mod", "usb_common", "ehci_hcd", "uhci_hcd", "ohci_hcd",
		"ehci_pci", "xhci_pci", "xhci_hcd", "virtio_blk", "virtio_pci",
		"part_msdos", "usbms", "usbhid", "hid-generic", "vfat", "nls_iso8859_1", "nls_cp437",
	}

	dirs = []string{"/mnt", "/run"}
)

func modprobe(s string) (string, error) {
	cmd := exec.Command("/usr/sbin/modprobe", s)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(stdoutStderr), nil
}

func depmod() error {
	cmd := exec.Command("/usr/sbin/depmod", "-a")
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func loadModules() error {
	// TODO: Report failed kernel modules load to a file available in
	// runtime /var/log
	// TODO: run depmod() if no alias file is found
	log.Println("Loading kernel modules", strings.Join(modules, " "))
	for _, m := range modules {
		modprobe(m) // Skip error and log output for now
	}

	// probe for modules to load udev-style
	drivers := probeKernelModules()
	log.Println("Loading detected kernel modules", strings.Join(drivers, " "))
	for _, k := range drivers {
		modprobe(k) // Skip error and log output for now
	}
	return nil
}

func ensureDirs() error {
	for _, d := range dirs {
		os.MkdirAll(d, os.ModePerm)
	}
	return nil
}

func initCmd() *initCmds {
	ctty := libinit.WithTTYControl(true)

	ensureDirs()
	loadModules()

	return &initCmds{
		cmds: []*exec.Cmd{
			libinit.Command("/bbin/dhclient", ctty, libinit.WithArguments("-ipv6=false")),
			libinit.Command("/loader", libinit.WithCloneFlags(syscall.CLONE_NEWPID), ctty),
			libinit.Command("/bin/sh", ctty),
		},
	}
}

func main() {
	log.Printf("Welcome to cOS!")
	log.SetPrefix("init: ")

	//	debug = log.Printf

	if err := ulog.KernelLog.SetConsoleLogLevel(ulog.KLogEmergency); err != nil {
		log.Printf("Could not set log level: %v", err)
	}

	libinit.SetEnv()
	libinit.CreateRootfs()
	libinit.NetInit()

	ic := initCmd()

	if err := ulog.KernelLog.SetConsoleLogLevel(ulog.KLogNotice); err != nil {
		log.Printf("Could not set log level: %v", err)
	}

	cmdCount := libinit.RunCommands(debug, ic.cmds...)
	if cmdCount == 0 {
		log.Printf("No suitable executable found in %v", ic.cmds)
	}

	log.Printf("Waiting for orphaned children")
	libinit.WaitOrphans()
	log.Printf("All commands are done")
}