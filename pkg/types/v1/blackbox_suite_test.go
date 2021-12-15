
package v1_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBlackbox(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "v1 Blackbox test suite")
}