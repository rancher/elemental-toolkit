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

package cmd

import (
	"bytes"
	. "github.com/onsi/gomega"
	"testing"
)


func TestInstallNoParams(t *testing.T) {
	RegisterTestingT(t)
	// Silence cobra output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	_, _, err := executeCommandC(rootCmd, "install")
	// Restore cobra output
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	Expect(err).ToNot(BeNil())
}

func TestInstall(t *testing.T) {
	RegisterTestingT(t)
	_, _, err := executeCommandC(rootCmd, "install", "/dev/null")
	//Check output from command here once we add stuff that prints?
	Expect(err).To(BeNil())
}

