package cos_test

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

func TestTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cOS Installer test Suite")
}

func CheckPartitionValues(diskLayout sut.DiskLayout, entry sut.PartitionEntry) {
	part, err := diskLayout.GetPartition(entry.Label)
	Expect(err).To(BeNil())
	Expect((part.Size / 1024) / 1024).To(Equal(entry.Size))
	Expect(part.FsType).To(Equal(entry.FsType))
}

var squashfs bool

func init() {
	flag.BoolVar(&squashfs, "squashfs", false, "Sets the installation of squashfs recovery")
}
