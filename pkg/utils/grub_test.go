package utils

import (
	"bytes"
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"testing"
)

func TestGrubInstall(t *testing.T) {
	RegisterTestingT(t)
	buf := &bytes.Buffer{}
	logger := log.New()
	logger.SetOutput(buf)
	fs := afero.NewMemMapFs()
	_, _ = fs.Create("/etc/cos/grub.cfg")

	config := v1.RunConfig{
		Device: "/dev/test",
		Logger: logger,
		Fs:     fs,
	}
	runner := v1mock.FakeRunner{}

	grub := NewGrub(
		&config,
		WithRunnerGrub(&runner),
	)

	err := grub.Install()

	Expect(err).To(BeNil())
	Expect(buf).To(ContainSubstring("Installing GRUB.."))
	Expect(buf).To(ContainSubstring("Grub install to device /dev/test complete"))
	Expect(buf).ToNot(ContainSubstring("efi"))
}

func TestGrubInstallCfgContents(t *testing.T) {
	RegisterTestingT(t)
	buf := &bytes.Buffer{}
	logger := log.New()
	logger.SetOutput(buf)
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("/state/grub2/", 0666)
	_ = fs.MkdirAll("/etc/cos/", 0666)
	err := afero.WriteFile(fs, "/etc/cos/grub.cfg", []byte("console=tty1"), 0644)
	Expect(err).To(BeNil())

	config := v1.RunConfig{
		Device:   "/dev/test",
		Tty:      "",
		Logger:   logger,
		StateDir: "/state",
		GrubConf: "/etc/cos/grub.cfg",
		Fs:       fs,
	}
	runner := v1mock.FakeRunner{}

	grub := NewGrub(
		&config,
		WithRunnerGrub(&runner),
	)

	err = grub.Install()

	Expect(err).To(BeNil())
	Expect(buf.String()).ToNot(ContainSubstring("Adding extra tty (serial) to grub.cfg"))
	targetGrub, err := afero.ReadFile(fs, "/state/grub2/grub.cfg")
	Expect(err).To(BeNil())
	// Should not be modified at all
	Expect(targetGrub).To(ContainSubstring("console=tty1"))
}

func TestGrubInstallEfiX86_64Force(t *testing.T) {
	RegisterTestingT(t)
	buf := &bytes.Buffer{}
	logger := log.New()
	logger.SetOutput(buf)
	logger.SetLevel(log.DebugLevel)
	fs := afero.NewMemMapFs()
	_, _ = fs.Create("/etc/cos/grub.cfg")

	config := v1.RunConfig{
		Device:   "/dev/test",
		ForceEfi: true,
		Logger:   logger,
		Fs:       fs,
	}
	runner := v1mock.FakeRunner{}

	grub := NewGrub(
		&config,
		WithRunnerGrub(&runner),
	)

	err := grub.Install()

	Expect(err).To(BeNil())
	Expect(buf.String()).To(ContainSubstring("--target=x86_64-efi"))
	Expect(buf.String()).To(ContainSubstring("--efi-directory"))
	Expect(buf.String()).To(ContainSubstring("Installing grub efi for arch x86_64"))
}

func TestGrubInstallEfiX86_64NotForced(t *testing.T) {
	RegisterTestingT(t)
	buf := &bytes.Buffer{}
	logger := log.New()
	logger.SetOutput(buf)
	logger.SetLevel(log.DebugLevel)
	fs := afero.NewMemMapFs()
	_, _ = fs.Create("/etc/cos/grub.cfg")
	_, _ = fs.Create("/sys/firmware/efi")

	config := v1.RunConfig{
		Device: "/dev/test",
		Logger: logger,
		Fs:     fs,
	}
	runner := v1mock.FakeRunner{}

	grub := NewGrub(
		&config,
		WithRunnerGrub(&runner),
	)

	err := grub.Install()

	Expect(err).To(BeNil())
	Expect(buf.String()).To(ContainSubstring("--target=x86_64-efi"))
	Expect(buf.String()).To(ContainSubstring("--efi-directory"))
	Expect(buf.String()).To(ContainSubstring("Installing grub efi for arch x86_64"))
}

func TestGrubInstallTty(t *testing.T) {
	RegisterTestingT(t)
	buf := &bytes.Buffer{}
	logger := log.New()
	logger.SetOutput(buf)
	fs := afero.NewMemMapFs()
	_, _ = fs.Create("/etc/cos/grub.cfg")
	_, _ = fs.Create("/dev/serial")

	config := v1.RunConfig{
		Device: "/dev/test",
		Tty:    "serial",
		Logger: logger,
		Fs:     fs,
	}
	runner := v1mock.FakeRunner{}

	grub := NewGrub(
		&config,
		WithRunnerGrub(&runner),
	)

	err := grub.Install()

	Expect(err).To(BeNil())
	Expect(buf.String()).To(ContainSubstring("Adding extra tty (serial) to grub.cfg"))
}

func TestGrubInstallTtyConfig(t *testing.T) {
	RegisterTestingT(t)
	buf := &bytes.Buffer{}
	logger := log.New()
	logger.SetOutput(buf)
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("/state/grub2/", 0666)
	_ = fs.MkdirAll("/etc/cos/", 0666)
	err := afero.WriteFile(fs, "/etc/cos/grub.cfg", []byte("console=tty1"), 0644)
	Expect(err).To(BeNil())
	_, _ = fs.Create("/dev/serial")

	config := v1.RunConfig{
		Device:   "/dev/test",
		Tty:      "serial",
		Logger:   logger,
		StateDir: "/state",
		GrubConf: "/etc/cos/grub.cfg",
		Fs:       fs,
	}
	runner := v1mock.FakeRunner{}

	grub := NewGrub(
		&config,
		WithRunnerGrub(&runner),
	)

	err = grub.Install()

	Expect(err).To(BeNil())
	Expect(buf.String()).To(ContainSubstring("Adding extra tty (serial) to grub.cfg"))
	targetGrub, err := afero.ReadFile(fs, "/state/grub2/grub.cfg")
	Expect(err).To(BeNil())
	Expect(targetGrub).To(ContainSubstring("console=tty1 console=serial"))
}
