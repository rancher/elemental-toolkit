package cos_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sut "github.com/rancher-sandbox/ele-testhelpers/vm"
)

var _ = Describe("cOS Feature tests", func() {
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

	Context("After install", func() {
		It("can enable a persistent k3s install", func() {

			err := s.SendFile("../assets/features-config.yaml", "/usr/local/cloud-config/features-config.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			_, err = s.Command("mkdir /usr/local/features")
			Expect(err).ToNot(HaveOccurred())

			_, err = s.Command("cp -rf /system/features/vagrant.yaml /usr/local/features")
			Expect(err).ToNot(HaveOccurred())

			err = s.SendFile("../assets/k3s.yaml", "/usr/local/features/k3s.yaml", "0770")
			Expect(err).ToNot(HaveOccurred())

			s.Reboot()

			out, err := s.Command("cos-feature enable k3s")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("k3s enabled"))

			s.Reboot()

			out, err = s.Command("cos-feature list")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("k3s (enabled)"))

			Eventually(func() string {
				out, _ := s.Command("k3s --data-dir /usr/local/rancher/k3s/ kubectl get pods -A")
				return out

			}, time.Duration(time.Duration(400)*time.Second), time.Duration(5*time.Second)).Should(ContainSubstring("local-path-provisioner"))

			_, err = s.Command("k3s --data-dir /usr/local/rancher/k3s/ kubectl create deployment test-sut-deployment --image nginx")
			Expect(err).ToNot(HaveOccurred())

			s.Reboot()
			Eventually(func() string {
				out, _ := s.Command("k3s --data-dir /usr/local/rancher/k3s/ kubectl get pods -A")
				return out

			}, time.Duration(time.Duration(900)*time.Second), time.Duration(5*time.Second)).Should(ContainSubstring("test-sut-deployment"))

			out, err = s.Command("cos-feature disable k3s")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("k3s disabled"))
			s.Reboot()

			out, err = s.Command("cos-feature disable k3s")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).Should(ContainSubstring("Feature k3s not enabled"))
		})
	})
})
