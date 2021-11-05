package cos_test

import (
	"github.com/rancher-sandbox/cOS/tests/sut"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS Installer tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects()
	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			s.GatherAllLogs()
		}
	})

	Context("generic tests", func() {
		It("booted from iso", func() {
			out, err := s.Command("cat /proc/cmdline")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("live:CDLABEL"))
		})
		It("has the cdrom mounted", func() {
			out, err := s.Command("grep /dev/sr /etc/mtab")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("iso9660"))
		})
		It("has the LiveOS_rootfs mounted on /", func() {
			out, err := s.Command("df -h /")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("LiveOS_rootfs"))
		})
	})
	PContext("source tests", func() {
		It("from iso", func() {})
		It("from url", func() {})
	})
	PContext("partition layout tests", func() {
		It("with partition layout", func() {})
	})
	PContext("efi/gpt tests", func() {
		It("forces gpt", func() {})
		It("forces efi", func() {})
	})
	PContext("config file tests", func() {
		It("uses a proper config file", func() {})
		It("uses a wrong config file", func() {})
	})
})
