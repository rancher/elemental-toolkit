package cos_test

import (
	"github.com/rancher-sandbox/cOS/tests/sut"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cOS Recovery deploy tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(sut.TimeoutRawDiskTest)
	})

	Context("after running recovery from the raw_disk image", func() {
		It("uses cos-deploy to install", func() {
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Recovery))

			out, err := s.Command("cos-deploy")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Deployment done, now you might want to reboot"))

			s.Reboot(sut.TimeoutRawDiskTest)
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
		})
	})
})
