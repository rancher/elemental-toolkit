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

package utils

import (
	"fmt"
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental-cli/tests/mocks"
	"io/ioutil"
	"os"
	"testing"
)

func TestChroot(t *testing.T) {
	RegisterTestingT(t)
	runner := v1.TestRunner{}
	tmpDir, _ := os.MkdirTemp("", "elemental-*")
	chroot := NewChroot(tmpDir)
	defer chroot.Close()
	// Not even sure how to check that this works...I tested with the RealRunner and it fails to find the command so
	// That is a confirmation that it works, but otherwise....
	_, err := chroot.Run(&runner, "ls")
	Expect(err).To(BeNil())
	dirs, _ := ioutil.ReadDir(tmpDir)
	for _, d := range dirs {
		fullDir := fmt.Sprintf("/%s", d.Name())
		// Expect the dirs to appear in the defaultMounts
		Expect(stringInSlice(fullDir, chroot.defaultMounts)).To(BeTrue())
	}
}

// I wonder how many times this code has been repeated in go history....
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}