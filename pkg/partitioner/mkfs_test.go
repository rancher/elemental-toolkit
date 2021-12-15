/*
Copyright © 2021 SUSE LLC

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
