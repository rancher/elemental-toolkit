package v1_test

import (
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/sirupsen/logrus"
	"reflect"
	"testing"
)

// Test logger is same type as a logrus.Logger
func TestNewLogger(t *testing.T) {
	RegisterTestingT(t)
	l1 := v1.NewLogger()
	l2 := logrus.New()
	Expect(reflect.TypeOf(l1).Kind()).To(Equal(reflect.TypeOf(l2).Kind()))
}

// Test logger is same type as a logrus.Logger
func TestNewNullLogger(t *testing.T) {
	RegisterTestingT(t)
	l1 := v1.NewNullLogger()
	l2 := logrus.New()
	Expect(reflect.TypeOf(l1).Kind()).To(Equal(reflect.TypeOf(l2).Kind()))
}
