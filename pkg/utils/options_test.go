package utils

import (
	. "github.com/onsi/gomega"
	v1mock "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"github.com/spf13/afero"
	"k8s.io/mount-utils"
	"testing"
)

func TestOptionsChroot(t *testing.T) {
	c := Chroot{}
	Expect(c.mounter).To(BeNil())
	mounter := mount.FakeMounter{}
	f := WithMounter(&mounter)
	err := f(&c)
	Expect(err).To(BeNil())
	Expect(c.mounter).To(Equal(&mounter))

	c = Chroot{}
	Expect(c.syscall).To(BeNil())
	syscall := v1mock.FakeSyscall{}
	f = WithSyscall(&syscall)
	err = f(&c)
	Expect(err).To(BeNil())
	Expect(c.syscall).To(Equal(&syscall))

	c = Chroot{}
	Expect(c.runner).To(BeNil())
	runner := v1mock.FakeRunner{}
	f = WithRunner(&runner)
	err = f(&c)
	Expect(err).To(BeNil())
	Expect(c.runner).To(Equal(&runner))

	c = Chroot{}
	Expect(c.fs).To(BeNil())
	fs := afero.NewMemMapFs()
	f = WithFS(fs)
	err = f(&c)
	Expect(err).To(BeNil())
	Expect(c.fs).To(Equal(fs))
}
