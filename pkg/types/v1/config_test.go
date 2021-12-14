package v1_test

import (
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"testing"
)

func TestSetupStyleDefault(t *testing.T) {
	RegisterTestingT(t)
	c := v1.NewRunConfig(
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(&mount.FakeMounter{}),
		v1.WithRunner(&v1mock.FakeRunner{}),
		v1.WithSyscall(&v1mock.FakeSyscall{}))
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.MSDOS))
	Expect(c.BootFlag).To(Equal(v1.BOOT))
	c = v1.NewRunConfig(
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(&mount.FakeMounter{}),
		v1.WithRunner(&v1mock.FakeRunner{}),
		v1.WithSyscall(&v1mock.FakeSyscall{}))
	c.ForceEfi = true
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.GPT))
	Expect(c.BootFlag).To(Equal(v1.ESP))
	c = v1.NewRunConfig(
		v1.WithFs(afero.NewMemMapFs()),
		v1.WithMounter(&mount.FakeMounter{}),
		v1.WithRunner(&v1mock.FakeRunner{}),
		v1.WithSyscall(&v1mock.FakeSyscall{}))
	c.ForceGpt = true
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.GPT))
	Expect(c.BootFlag).To(Equal(v1.BIOS))
}
