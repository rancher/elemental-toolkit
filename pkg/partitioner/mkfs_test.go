package partitioner_test

import (
	. "github.com/onsi/gomega"
	part "github.com/rancher-sandbox/elemental-cli/pkg/partitioner"
	mocks "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"testing"
)

func TestApply(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.NewTestRunnerV2()
	cmds := [][]string{
		{"mkfs.xfs", "-L", "OEM", "/some/device"},
		{"mkfs.vfat", "-i", "EFI", "/some/device"},
	}
	mkfs := part.NewMkfsCall("/some/device", "xfs", "OEM", runner)
	_, err := mkfs.Apply()
	Expect(err).To(BeNil())
	mkfs = part.NewMkfsCall("/some/device", "vfat", "EFI", runner)
	_, err = mkfs.Apply()
	Expect(err).To(BeNil())
	Expect(runner.CmdsMatch(cmds)).To(BeNil())
	mkfs = part.NewMkfsCall("/some/device", "btrfs", "OEM", runner)
	_, err = mkfs.Apply()
	Expect(err).NotTo(BeNil())
}
