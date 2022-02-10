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
	"context"
	dockTypes "github.com/docker/docker/api/types"
	dockClient "github.com/docker/docker/client"
	"github.com/mudler/go-pluggable"
	"github.com/mudler/luet/pkg/api/core/bus"
	"github.com/spf13/afero"

	luetTypes "github.com/mudler/luet/pkg/api/core/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/sirupsen/logrus"
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
		Describe("Luet config", Label("config"), func() {
			It("Create empty config if there is no luet.yaml", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				fs := afero.NewMemMapFs()
				v1.NewLuet(v1.WithLuetLogger(log), v1.WithLuetFs(fs))
				Expect(memLog.String()).To(ContainSubstring("Creating empty luet config"))
			})
			It("Fail to parse wrong luet.yaml", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/etc/luet/luet.yaml", []byte("not valid I think? Maybe yes, who knows, only the yaml gods"), os.ModePerm)
				v1.NewLuet(v1.WithLuetLogger(log), v1.WithLuetFs(fs))
				Expect(memLog.String()).To(ContainSubstring("Loading luet config from /etc/luet/luet.yaml"))
				Expect(memLog.String()).To(ContainSubstring("Error unmarshalling luet.yaml"))
			})
			It("Loads default luet.yaml", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/etc/luet/luet.yaml", []byte("general:\n  debug: false\n  enable_emoji: false"), os.ModePerm)
				v1.NewLuet(v1.WithLuetLogger(log), v1.WithLuetFs(fs))
				Expect(memLog.String()).To(ContainSubstring("Loading luet config from /etc/luet/luet.yaml"))
			})
			It("Fails to init with broken paths", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/etc/luet/luet.yaml", []byte("system:\n  rootfs: /naranjas"), os.ModePerm)
				v1.NewLuet(v1.WithLuetLogger(log), v1.WithLuetFs(fs))
				Expect(memLog.String()).To(ContainSubstring("Loading luet config from /etc/luet/luet.yaml"))
				Expect(memLog.String()).To(ContainSubstring("Error running init on luet config"))
			})

		})
		Describe("Luet options", Label("options"), func() {
			It("Sets plugins correctly", func() {
				v1.NewLuet(v1.WithLuetPlugins("mkdir"))
				p := pluggable.Plugin{
					Name:       "mkdir",
					Executable: "/usr/bin/mkdir",
				}
				Expect(bus.Manager.Plugins).To(ContainElement(p))
			})
			It("Sets plugins correctly with log", func() {
				v1.NewLuet(v1.WithLuetLogger(v1.NewNullLogger()), v1.WithLuetPlugins("cat"))
				p := pluggable.Plugin{
					Name:       "cat",
					Executable: "/usr/bin/cat",
				}
				Expect(bus.Manager.Plugins).To(ContainElement(p))
			})
			It("Sets logger correctly", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				v1.NewLuet(v1.WithLuetLogger(log))
				// Check if the debug stuff was logged to the buffer
				Expect(memLog.String()).To(ContainSubstring("Creating empty luet config"))
			})
			It("Sets config", func() {
				cfg := luetTypes.LuetConfig{}
				v1.NewLuet(v1.WithLuetConfig(&cfg))
			})
			It("Sets Auth", func() {
				auth := dockTypes.AuthConfig{}
				v1.NewLuet(v1.WithLuetAuth(&auth))
			})
			It("Sets FS", func() {
				fs := afero.NewMemMapFs()
				v1.NewLuet(v1.WithLuetFs(fs))
			})

		})
	})
})
