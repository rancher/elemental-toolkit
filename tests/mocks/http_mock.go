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

package mocks

import (
	"io"
	"net/http"
)

var MockClientCalls []string

type MockBody struct {}

func (m MockBody) Read(p []byte) (n int, err error){
	return 1024, io.EOF
}

func (m MockBody) Close() error {
	return nil
}

type MockClient struct {}

func (m *MockClient) Get(url string) (*http.Response, error) {
	// Store calls to the mock client, so we can verify that we didnt mangled them or anything
	MockClientCalls = append(MockClientCalls, url)
	return &http.Response{Body: &MockBody{}}, nil
}
