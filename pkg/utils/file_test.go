package utils

import (
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"testing"
)

func TestSelinuxRelabel(t *testing.T) {
	// I cant seem to mock  exec.LookPath so it will always fail tor un due setfiles not being in the system :/
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	// This is actually failing but not sure we should return an error
	Expect(selinuxRelabel("/", fs, true)).ToNot(BeNil())
	fs = afero.NewMemMapFs()
	_, _ = fs.Create("/etc/selinux/targeted/contexts/files/file_contexts")
	Expect(selinuxRelabel("/", fs, false)).To(BeNil())
}
