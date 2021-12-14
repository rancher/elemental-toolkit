package v1_test

import (
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"testing"
)

func TestRealRunner_Run(t *testing.T) {
	RegisterTestingT(t)
	r := v1.RealRunner{}
	_, err := r.Run("pwd")
	Expect(err).To(BeNil())
}
