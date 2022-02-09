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
	"context"
	dockTypes "github.com/docker/docker/api/types"
	dockClient "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"io"
	"io/ioutil"
	"os"
)

var _ = Describe("Types", Label("luet", "types"), func() {
	var luet *v1.Luet
	var target string
	BeforeEach(func() {
		var err error
		target, err = os.MkdirTemp("", "elemental")
		Expect(err).To(BeNil())
		luet = v1.NewLuet(v1.WithLuetLogger(v1.NewNullLogger()))
	})
	AfterEach(func() {
		Expect(os.RemoveAll(target)).To(BeNil())
	})
	Describe("Luet", func() {
		It("Fails to unpack without root privileges", Label("unpack"), func() {
			image := "quay.io/costoolkit/releases-green:cloud-config-system-0.11-1"
			Expect(luet.Unpack(target, image, false)).NotTo(BeNil())
		})
		It("Check that luet can unpack the local image", Label("unpack", "root"), func() {
			image := "docker.io/library/alpine"
			ctx := context.Background()
			cli, err := dockClient.NewClientWithOpts(dockClient.FromEnv, dockClient.WithAPIVersionNegotiation())
			Expect(err).ToNot(HaveOccurred())
			// Pull image
			reader, err := cli.ImagePull(ctx, image, dockTypes.ImagePullOptions{})
			defer reader.Close()
			_, _ = io.Copy(ioutil.Discard, reader)
			// Check that luet can unpack the local image
			Expect(luet.Unpack(target, image, true)).To(BeNil())
		})
	})
})
