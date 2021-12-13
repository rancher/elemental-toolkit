package v1_test

import (
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/afero"
	"testing"
)

func TestSetupStyleDefault(t *testing.T) {
	RegisterTestingT(t)
	c := v1.NewRunConfig(v1.WithFs(afero.NewMemMapFs()))
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.MSDOS))
	Expect(c.BootFlag).To(Equal(v1.BOOT))
	c = v1.NewRunConfig(v1.WithFs(afero.NewMemMapFs()))
	c.ForceEfi = true
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.GPT))
	Expect(c.BootFlag).To(Equal(v1.ESP))
	c = v1.NewRunConfig(v1.WithFs(afero.NewMemMapFs()))
	c.ForceGpt = true
	c.SetupStyle()
	Expect(c.PartTable).To(Equal(v1.GPT))
	Expect(c.BootFlag).To(Equal(v1.BIOS))
}
