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
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/afero"
	"io/ioutil"
	"os"
	"testing"
)


func TestGetUrlNet(t *testing.T) {
	RegisterTestingT(t)
	client := &MockClient{}

	url := "http://fake.com"
	tmpDir, _ := os.MkdirTemp("", "elemental")
	destination := fmt.Sprintf("%s/test", tmpDir)
	defer os.RemoveAll(tmpDir)
	err := GetUrl(client, url, destination)
	Expect(err).To(BeNil())
	Expect(MockClientCalls).To(ContainElement("http://fake.com"))
	f, err := os.Stat(destination)
	Expect(err).To(BeNil())
	Expect(f.Size()).To(Equal(int64(1024)))  // We create a 1024 bytes size file on the mock

	ftp := "ftp://fake"
	err = GetUrl(client, ftp, destination)
	Expect(err).To(BeNil())
	Expect(MockClientCalls).To(ContainElement("ftp://fake"))
	f, err = os.Stat(destination)
	Expect(err).To(BeNil())
	Expect(f.Size()).To(Equal(int64(1024)))  // We create a 1024 bytes size file on the mock

	tftp := "tftp://fake"
	err = GetUrl(client, tftp, destination)
	Expect(err).To(BeNil())
	Expect(MockClientCalls).To(ContainElement("tftp://fake"))
	f, err = os.Stat(destination)
	Expect(err).To(BeNil())
	Expect(f.Size()).To(Equal(int64(1024)))  // We create a 1024 bytes size file on the mock
}


func TestGetUrlFile(t *testing.T) {
	RegisterTestingT(t)
	client := &MockClient{}
	testString := "In a galaxy far far away..."

	tmpDir, _ := os.MkdirTemp("", "elemental")
	defer os.RemoveAll(tmpDir)

	destination := fmt.Sprintf("%s/test", tmpDir)
	sourceName := fmt.Sprintf("%s/source", tmpDir)

	// Create source file
	s, err := os.Create(sourceName)
	_, err = s.WriteString(testString)
	defer s.Close()

	err = GetUrl(client, sourceName, destination)
	Expect(err).To(BeNil())

	// check they are the same size
	f, err := os.Stat(destination)
	source, err := os.Stat(sourceName)
	Expect(err).To(BeNil())
	Expect(f.Size()).To(Equal(source.Size()))

	// Cehck destination contains what we had on the source
	dest, err := ioutil.ReadFile(destination)
	Expect(dest).To(ContainSubstring(testString))
}

func TestBootedFrom(t *testing.T) {
	RegisterTestingT(t)
	Expect(BootedFrom("I_EXPECT_THIS_LABEL_TO_NOT_EXIST")).To(BeFalse())
	Expect(BootedFrom("")).To(BeTrue()) // Empty label should match everything!
}

func TestSetupStyleDefault(t *testing.T) {
	RegisterTestingT(t)
	fs := afero.NewMemMapFs()
	c := v1.RunConfig{}
	setupStyle(&c, fs)
	Expect(c.PartTable).To(Equal(MSDOS))
	Expect(c.BootFlag).To(Equal(BOOT))
	c = v1.RunConfig{
		ForceEfi: true,
	}
	setupStyle(&c, fs)
	Expect(c.PartTable).To(Equal(GPT))
	Expect(c.BootFlag).To(Equal(ESP))
	c = v1.RunConfig{
		ForceGpt: true,
	}
	setupStyle(&c, fs)
	Expect(c.PartTable).To(Equal(GPT))
	Expect(c.BootFlag).To(Equal(BIOS))
}