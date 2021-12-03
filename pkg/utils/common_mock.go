package utils

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