/*
Copyright © 2022 - 2026 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package elemental_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sut "github.com/rancher/elemental-toolkit/v2/tests/vm"
)

var _ = Describe("Elemental Recovery deploy tests", func() {
	var s *sut.SUT

	BeforeEach(func() {
		s = sut.NewSUT()
		s.EventuallyConnects(sut.TimeoutRawDiskTest)
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			s.GatherAllLogs()
		}
	})

	Context("after running recovery from the raw_disk image", func() {
		It("uses cos-deploy to install", func() {
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Recovery))

			_, err := s.Command("elemental reset")
			Expect(err).ToNot(HaveOccurred())

			s.Reboot(sut.TimeoutRawDiskTest)
			ExpectWithOffset(1, s.BootFrom()).To(Equal(sut.Active))
		})
	})
})
