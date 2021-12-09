package utils

import (
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"testing"
)

func TestSelinuxRelabel(t *testing.T) {
	// This testing is ridiculous as this function can fail or not and we dont really care, we cannot return the proper error
	// So we are not really testing
	// Also, I cant seem to mock  exec.LookPath so it will always fail tor un due setfiles not being in the system :/
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	// This is actually failing but not sure we should return an error
	Expect(selinuxRelabel("/", fs)).To(BeNil())
	fs = afero.NewMemMapFs()
	_, _ = fs.Create("/etc/selinux/targeted/contexts/files/file_contexts")
	Expect(selinuxRelabel("/", fs)).To(BeNil())
}
