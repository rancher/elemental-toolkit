package cos_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/cOS/tests/sut"
)

var _ = Describe("cOS Deploy tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(360)
	})

	AfterEach(func() {
		// Try to gather mtree logs on failure
		if CurrentGinkgoTestDescription().Failed {
			s.GatherAllLogs()
		}
	})

	Context("From recovery", func() {
		When("deploying again", func() {
			It("deploys/resets the system", func() {
				err := s.ChangeBootOnce(sut.Recovery)
				Expect(err).ToNot(HaveOccurred())
				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Recovery))
				_, err = s.Command(fmt.Sprintf("cos-deploy --docker-image %s:cos-system-%s", s.GreenRepo, s.TestVersion))
				Expect(err).NotTo(HaveOccurred())
				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
			})
		})
	})
})
