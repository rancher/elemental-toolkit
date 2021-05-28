package cos_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Upgrade tests - Images signed", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
	})

	AfterEach(func() {
		s.Reset()
	})
	Context("After install", func() {
		When("upgrading", func() {
			It("fails if verify is enabled on an unsigned version", func() {
				// Using releases-amd64 as those images are not signed
				out, err := s.Command("cos-upgrade --docker-image quay.io/costoolkit/releases-opensuse:cos-system-0.5.1")
				Expect(err).To(HaveOccurred())
				Expect(out).Should(ContainSubstring("failed verifying image"))
			})
		})
	})
})
