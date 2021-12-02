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
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

type MockBody struct {}

func (m MockBody) Read(p []byte) (n int, err error){
	return 1024, io.EOF
}

func (m MockBody) Close() error {
	return nil
}

type MockClient struct {
	Calls []string
}

func (m MockClient) GetCalls() []string {
	return m.Calls
}

func (m *MockClient) Get(url string) (*http.Response, error) {
	// Store calls to the mock client, so we can verify them
	m.Calls = append(m.Calls, url)
	return &http.Response{Body: &MockBody{}}, nil
}


func TestGetUrlNet(t *testing.T) {
	RegisterTestingT(t)
	Client = &MockClient{}
	url := "http://fake.com"
	tmpDir, _ := os.MkdirTemp("", "elemental")
	destination := fmt.Sprintf("%s/test", tmpDir)
	defer os.RemoveAll(tmpDir)
	err := GetUrl(url, destination)
	Expect(err).To(BeNil())
	f, err := os.Stat(destination)
	Expect(err).To(BeNil())
	Expect(f.Size()).To(Equal(int64(1024)))  // We create a 1024 bytes size file on the mock
}

func TestGetUrlFile(t *testing.T) {
	RegisterTestingT(t)
	testString := "In a galaxy far far away..."

	tmpDir, _ := os.MkdirTemp("", "elemental")
	defer os.RemoveAll(tmpDir)

	destination := fmt.Sprintf("%s/test", tmpDir)
	sourceName := fmt.Sprintf("%s/source", tmpDir)

	// Create source file
	s, err := os.Create(sourceName)
	_, err = s.WriteString(testString)
	defer s.Close()

	err = GetUrl(sourceName, destination)
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


