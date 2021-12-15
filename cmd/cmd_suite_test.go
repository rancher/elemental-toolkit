package cmd

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWhitebox(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI whitebox test suite")
}
