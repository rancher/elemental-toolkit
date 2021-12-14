package v1_test

import (
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"testing"
)

func TestSyscall(t *testing.T) {
	RegisterTestingT(t)
	r := v1.RealSyscall{}
	err := r.Chroot("/tmp/")
	// We need elevated privs to chroot so this should fail
	Expect(err).ToNot(BeNil())
}
