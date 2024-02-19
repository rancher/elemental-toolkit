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

package v1_test

import (
	"bytes"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	v2 "github.com/rancher/elemental-toolkit/v2/pkg/types/v2"
)

var _ = Describe("logger", Label("log", "logger", "types"), func() {
	It("TestNewLogger returns a logger interface", func() {
		l1 := v2.NewLogger()
		l2 := logrus.New()
		Expect(reflect.TypeOf(l1).Kind()).To(Equal(reflect.TypeOf(l2).Kind()))
	})
	It("TestNewNullLogger returns logger interface", func() {
		l1 := v2.NewNullLogger()
		l2 := logrus.New()
		Expect(reflect.TypeOf(l1).Kind()).To(Equal(reflect.TypeOf(l2).Kind()))
	})
	It("DebugLevel returns the proper log level for debug output", func() {
		Expect(v2.DebugLevel()).To(Equal(logrus.DebugLevel))
	})
	It("Returns true on IsDebugLevel when log level is set to debug", func() {
		l := v2.NewLogger()
		l.SetLevel(v2.DebugLevel())
		Expect(v2.IsDebugLevel(l)).To(BeTrue())
	})
	It("Returns false on IsDebugLevel when log level is not set to debug", func() {
		Expect(v2.IsDebugLevel(v2.NewLogger())).To(BeFalse())
	})
	It("NewBufferLogger stores content in a buffer", func() {
		b := &bytes.Buffer{}
		l1 := v2.NewBufferLogger(b)
		l1.Info("TEST")
		Expect(b).To(ContainSubstring("TEST"))
	})
})
