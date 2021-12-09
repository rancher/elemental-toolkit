package v1

import (
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"testing"
)

func TestSetupStyleDefault(t *testing.T) {
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	c := RunConfig{}
	c.setupStyle(fs)
	Expect(c.PartTable).To(Equal(MSDOS))
	Expect(c.BootFlag).To(Equal(BOOT))
	c = RunConfig{
		ForceEfi: true,
	}
	c.setupStyle(fs)
	Expect(c.PartTable).To(Equal(GPT))
	Expect(c.BootFlag).To(Equal(ESP))
	c = RunConfig{
		ForceGpt: true,
	}
	c.setupStyle(fs)
	Expect(c.PartTable).To(Equal(GPT))
	Expect(c.BootFlag).To(Equal(BIOS))
}
