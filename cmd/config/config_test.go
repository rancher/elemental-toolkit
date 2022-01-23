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

package config

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/rancher-sandbox/elemental/pkg/types/v1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"
	"os"
	"testing"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI config test suite")
}

var _ = Describe("Config", func() {
	Context("Build config", func() {
		It("values empty if config path not valid", func() {
			cfg, err := ReadConfigBuild("/none/")
			Expect(err).To(BeNil())
			Expect(viper.GetString("label")).To(Equal(""))
			Expect(cfg.Label).To(Equal(""))
		})
		It("values filled if config path valid", func() {
			cfg, err := ReadConfigBuild("config/")
			Expect(err).To(BeNil())
			Expect(viper.GetString("label")).To(Equal("COS_LIVE"))
			Expect(cfg.Label).To(Equal("COS_LIVE"))
		})
		It("overrides values with env values", func() {
			_ = os.Setenv("ELEMENTAL_LABEL", "environment")
			cfg, err := ReadConfigBuild("config/")
			Expect(err).To(BeNil())
			source := viper.GetString("label")
			// check that the final value comes from the env var
			Expect(source).To(Equal("environment"))
			Expect(cfg.Label).To(Equal("environment"))
		})

	})
	Context("Run config", func() {
		var logger v1.Logger
		var mounter mount.Interface

		BeforeEach(func() {
			logger = logrus.New()
			mounter = &mount.FakeMounter{}
		})

		It("values empty if config does not exist", func() {
			cfg, err := ReadConfigRun("/none/", logger, mounter)
			Expect(err).To(BeNil())
			source := viper.GetString("file")
			Expect(source).To(Equal(""))
			Expect(cfg.Source).To(Equal(""))
		})
		It("values empty if config value is empty", func() {
			cfg, err := ReadConfigRun("", logger, mounter)
			Expect(err).To(BeNil())
			source := viper.GetString("file")
			Expect(source).To(Equal(""))
			Expect(cfg.Source).To(Equal(""))
		})
		It("overrides values with config files", func() {
			cfg, err := ReadConfigRun("config/", logger, mounter)
			Expect(err).To(BeNil())
			source := viper.GetString("target")
			// check that the final value comes from the extra file
			Expect(source).To(Equal("extra"))
			Expect(cfg.Target).To(Equal("extra"))
		})
		It("overrides values with env values", func() {
			_ = os.Setenv("ELEMENTAL_TARGET", "environment")
			cfg, err := ReadConfigRun("config/", logger, mounter)
			Expect(err).To(BeNil())
			source := viper.GetString("target")
			// check that the final value comes from the env var
			Expect(source).To(Equal("environment"))
			Expect(cfg.Target).To(Equal("environment"))
		})
		It("sets log level debug based on debug flag", func() {
			// Default value
			_, err := ReadConfigRun("config/", logger, mounter)
			Expect(err).To(BeNil())
			debug := viper.GetBool("debug")
			Expect(logger.GetLevel()).ToNot(Equal(logrus.DebugLevel))
			Expect(debug).To(BeFalse())

			// Set it via viper, like the flag
			viper.Set("debug", true)
			_, err = ReadConfigRun("config/", logger, mounter)
			Expect(err).To(BeNil())
			debug = viper.GetBool("debug")
			Expect(debug).To(BeTrue())
			Expect(logger.GetLevel()).To(Equal(logrus.DebugLevel))
		})
	})
})
