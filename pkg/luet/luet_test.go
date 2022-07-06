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

package luet_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/utils"

	dockTypes "github.com/docker/docker/api/types"
	dockClient "github.com/docker/docker/client"
	"github.com/mudler/go-pluggable"
	"github.com/mudler/luet/pkg/api/core/bus"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"

	"io"
	"io/ioutil"
	"os"

	luetTypes "github.com/mudler/luet/pkg/api/core/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/rancher/elemental-cli/pkg/luet"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
)

func TestElementalSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Luet test suite")
}

var _ = Describe("Types", Label("luet", "types"), func() {
	var l *luet.Luet
	var target string
	var fs vfs.FS
	var cleanup func()

	BeforeEach(func() {
		var err error
		fs, cleanup, _ = vfst.NewTestFS(nil)
		fs.Mkdir("/etc", os.ModePerm)
		fs.Mkdir("/etc/luet", os.ModePerm)
		target, err = os.MkdirTemp("", "elemental")
		Expect(err).To(BeNil())
		l = luet.NewLuet(
			luet.WithLogger(v1.NewNullLogger()),
			luet.WithArch(constants.Archx86),
		)
	})
	AfterEach(func() {
		Expect(os.RemoveAll(target)).To(BeNil())
		cleanup()
	})
	Describe("Luet", func() {
		It("Fails to unpack without root privileges", Label("unpack"), func() {
			image := "quay.io/costoolkit/releases-green:cloud-config-system-0.11-1"
			_, err := l.Unpack(target, image, false)
			Expect(err).NotTo(BeNil())
		})
		It("Check that luet can unpack the remote image", Label("unpack", "root"), func() {
			image := "registry.opensuse.org/opensuse/redis"
			// Check that luet can unpack the remote image
			_, err := l.Unpack(target, image, false)
			Expect(err).To(BeNil())
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
			_, err = l.Unpack(target, image, true)
			Expect(err).To(BeNil())
		})
		Describe("UnpackFromChannel", Label("unpack", "channel"), func() {
			It("Check that luet can unpack from channel", Label("root"), func() {
				repo := v1.Repository{URI: "quay.io/costoolkit/releases-teal", Arch: constants.Archx86}
				_, err := l.UnpackFromChannel(target, "utils/gomplate", repo)
				Expect(err).To(BeNil())
			})
			It("Fails to unpack with a repository with no URI", func() {
				repo := v1.Repository{Arch: constants.Archx86}
				_, err := l.UnpackFromChannel(target, "utils/gomplate", repo)
				Expect(err.Error()).To(ContainSubstring("no URI is provided"))
				Expect(err).NotTo(BeNil())
			})
			It("Fails to unpack with a dir repository that doesnt have anything", func() {
				dir, _ := utils.TempDir(fs, "", "")
				repo := v1.Repository{URI: dir}
				_, err := l.UnpackFromChannel(target, "utils/gomplate", repo)
				Expect(err).NotTo(BeNil())
			})
			It("Fails to unpack with a http repository that doesnt exists", func() {
				repo := v1.Repository{URI: "http://jojo.bizarre.adventure"}
				_, err := l.UnpackFromChannel(target, "utils/gomplate", repo)
				Expect(err).NotTo(BeNil())
			})
			It("Fails to unpack with a strange repository that cant get the type for", func() {
				repo := v1.Repository{URI: "is:this:real:life", Arch: constants.Archx86}
				_, err := l.UnpackFromChannel(target, "utils/gomplate", repo)
				Expect(err.Error()).To(ContainSubstring("Invalid Luet repository URI"))
				Expect(err).NotTo(BeNil())
			})
			It("Fails to unpack from channel without root privileges", func() {
				repo := v1.Repository{URI: "quay.io/costoolkit/releases-teal"}
				_, err := l.UnpackFromChannel(target, "utils/gomplate", repo)
				Expect(err).ToNot(BeNil())
			})
			It("Fails to unpack from channel without matching arch", func() {
				repo := v1.Repository{URI: "quay.io/costoolkit/releases-teal-arm-64", Arch: constants.ArchArm64}
				_, err := l.UnpackFromChannel(target, "utils/gomplate", repo)
				Expect(err.Error()).To(ContainSubstring("package 'utils/gomplate->=0' not found"))
				Expect(err).NotTo(BeNil())
			})
		})

		Describe("Luet config", Label("config"), func() {
			It("Create empty config if there is no luet.yaml", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				luet.NewLuet(luet.WithLogger(log), luet.WithFs(fs))
				Expect(memLog.String()).To(ContainSubstring("Creating empty luet config"))
			})
			It("Fail to parse wrong luet.yaml", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				Expect(fs.WriteFile("/etc/luet/luet.yaml", []byte("not valid I think? Maybe yes, who knows, only the yaml gods"), os.ModePerm)).ShouldNot(HaveOccurred())
				luet.NewLuet(luet.WithLogger(log), luet.WithFs(fs))
				Expect(memLog.String()).To(ContainSubstring("Loading luet config from /etc/luet/luet.yaml"))
				Expect(memLog.String()).To(ContainSubstring("Error unmarshalling luet.yaml"))
			})
			It("Loads default luet.yaml", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				_ = fs.WriteFile("/etc/luet/luet.yaml", []byte("general:\n  debug: false\n  enable_emoji: false"), os.ModePerm)
				luet.NewLuet(luet.WithLogger(log), luet.WithFs(fs))
				Expect(memLog.String()).To(ContainSubstring("Loading luet config from /etc/luet/luet.yaml"))
			})
			It("Fails to init with broken paths", func() {
				memLog := bytes.Buffer{}
				log := v1.NewBufferLogger(&memLog)
				log.SetLevel(logrus.DebugLevel)
				_ = fs.WriteFile("/etc/luet/luet.yaml", []byte("system:\n  rootfs: /naranjas"), os.ModePerm)
				luet.NewLuet(luet.WithLogger(log), luet.WithFs(fs))
				Expect(memLog.String()).To(ContainSubstring("Loading luet config from /etc/luet/luet.yaml"))
				Expect(memLog.String()).To(ContainSubstring("Error running init on luet config"))
			})

		})
		Describe("Luet options", Label("options"), func() {
			It("Sets plugins correctly", func() {
				l = luet.NewLuet(luet.WithPlugins("mkdir"))
				l.InitPlugins()
				p := pluggable.Plugin{
					Name:       "mkdir",
					Executable: "/usr/bin/mkdir",
				}
				Expect(bus.Manager.Plugins).To(ContainElement(p))
			})
			It("Sets plugins correctly with log", func() {
				l = luet.NewLuet(luet.WithLogger(v1.NewNullLogger()), luet.WithPlugins("cat"))
				l.InitPlugins()
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
				luet.NewLuet(luet.WithFs(fs), luet.WithLogger(log))
				// Check if the debug stuff was logged to the buffer
				Expect(memLog.String()).To(ContainSubstring("Creating empty luet config"))
			})
			It("Sets config", func() {
				cfg := luetTypes.LuetConfig{}
				luet.NewLuet(luet.WithConfig(&cfg))
			})
			It("Sets Auth", func() {
				auth := dockTypes.AuthConfig{}
				luet.NewLuet(luet.WithAuth(&auth))
			})
			It("Sets FS", func() {
				luet.NewLuet(luet.WithFs(fs))
			})

		})
	})
})
