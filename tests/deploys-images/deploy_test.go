package cos_test

import (
	"fmt"

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
		if CurrentGinkgoTestDescription().Failed == false {
			s.Reset()
		}
	})

	Context("From recovery", func() {
		When("deploying again", func() {
			It("deploys the system", func() {
				err := s.ChangeBootOnce(sut.Recovery)
				Expect(err).ToNot(HaveOccurred())
				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Recovery))

				out := s.GetOSRelease("VERSION")
				Expect(out).ToNot(Equal(""))

				version := out

				_, err = s.Command(fmt.Sprintf("elemental reset --docker-image %s:cos-system-%s", s.ArtifactsRepo, s.TestVersion))
				Expect(err).NotTo(HaveOccurred())
				s.Reboot()
				ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))

				out = s.GetOSRelease("VERSION")
				Expect(out).ToNot(Equal(""))

				Expect(out).ToNot(Equal(version))
				Expect(out).To(Equal(s.TestVersion))
			})
		})
	})
})
