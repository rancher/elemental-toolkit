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

package utils_test

import (
	"fmt"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental-cli/pkg/utils"
	"github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"os"
	"testing"
)

func TestBootedFrom(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.FakeRunner{}
	Expect(utils.BootedFrom(&runner, "I_EXPECT_THIS_LABEL_TO_NOT_EXIST")).To(BeFalse())
	Expect(utils.BootedFrom(&runner, "I_EXPECT_THIS_LABEL_TO_EXIST")).To(BeTrue())
}

func TestFindLabelNonExisting(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.FakeRunner{}
	out, err := utils.FindLabel(&runner, "DOESNT_EXIST")
	Expect(err).To(BeNil())
	Expect(out).To(BeEmpty())
}

func TestFindLabel(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.FakeRunner{}
	out, err := utils.FindLabel(&runner, "EXISTS")
	Expect(err).To(BeNil())
	Expect(out).To(Equal("/dev/fake"))
}

// TestFindLabelError tests that even with an error we return an empty string, so it would be as if the partition is not there
func TestFindLabelError(t *testing.T) {
	RegisterTestingT(t)
	runner := mocks.FakeRunner{ErrorOnCommand: true}
	out, err := utils.FindLabel(&runner, "WHATEVS")
	Expect(err).ToNot(BeNil())
	Expect(out).To(Equal(""))
}

// TestHelperFindLabel will be called by the FakeRunner when running the TestFindLabel func as it
// Matches the command + args and return the proper output we want for testing
func TestHelperFindLabel(*testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)
	fmt.Println("/dev/fake") // Mock the device returned for that label
}

// TestHelperBootedFrom will be called by the FakeRunner when running the BootedFrom func as it
// Matches the command + args and return the proper output we want for testing
func TestHelperBootedFrom(*testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)
	fmt.Println("I_EXPECT_THIS_LABEL_TO_EXIST") // Mock that label
}
