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
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"
	"os"
	"testing"
)

func setupTest(t *testing.T) {
	viper.Reset()
	RegisterTestingT(t)
}

var logger = logrus.New()
var mounter = mount.FakeMounter{}

func TestConfigRunCustomNotValidPath(t *testing.T) {
	setupTest(t)
	// Load only main config
	cfg, err := ReadConfigRun("/none/", logger, &mounter)
	Expect(err).To(BeNil())
	source := viper.GetString("file")
	Expect(source).To(Equal(""))
	Expect(cfg.Source).To(Equal(""))
}

func TestConfigRunCustomEmptyPath(t *testing.T) {
	setupTest(t)
	// Load only main config
	cfg, err := ReadConfigRun("", logger, &mounter)
	Expect(err).To(BeNil())
	source := viper.GetString("file")
	Expect(source).To(Equal(""))
	Expect(cfg.Source).To(Equal(""))
}

func TestConfigRunOverride(t *testing.T) {
	setupTest(t)
	cfg, err := ReadConfigRun("config/", logger, &mounter)
	Expect(err).To(BeNil())
	source := viper.GetString("target")
	// check that the final value comes from the extra file
	Expect(source).To(Equal("extra"))
	Expect(cfg.Target).To(Equal("extra"))
}

func TestConfigRunOverrideEnv(t *testing.T) {
	setupTest(t)
	_ = os.Setenv("ELEMENTAL_TARGET", "environment")
	cfg, err := ReadConfigRun("config/", logger, &mounter)
	Expect(err).To(BeNil())
	source := viper.GetString("target")
	// check that the final value comes from the env var
	Expect(source).To(Equal("environment"))
	Expect(cfg.Target).To(Equal("environment"))
}

func TestConfigRunDebugFlag(t *testing.T) {
	setupTest(t)
	// Default value
	_, err := ReadConfigRun("config/", logger, &mounter)
	Expect(err).To(BeNil())
	debug := viper.GetBool("debug")
	Expect(logger.Level).ToNot(Equal(logrus.DebugLevel))
	Expect(debug).To(BeFalse())

	// Set it via viper, like the flag
	viper.Set("debug", true)
	_, err = ReadConfigRun("config/", logger, &mounter)
	Expect(err).To(BeNil())
	debug = viper.GetBool("debug")
	Expect(debug).To(BeTrue())
	Expect(logger.Level).To(Equal(logrus.DebugLevel))

}

func TestConfigBuildCustomNotValidPath(t *testing.T) {
	setupTest(t)
	cfg, err := ReadConfigBuild("/none/")
	Expect(err).To(BeNil())
	Expect(viper.GetString("label")).To(Equal(""))
	Expect(cfg.Label).To(Equal(""))
}

func TestConfigBuildCustomPath(t *testing.T) {
	setupTest(t)
	cfg, err := ReadConfigBuild("config/")
	Expect(err).To(BeNil())
	Expect(viper.GetString("label")).To(Equal("COS_LIVE"))
	Expect(cfg.Label).To(Equal("COS_LIVE"))
}

func TestConfigBuildOverrideEnv(t *testing.T) {
	setupTest(t)
	_ = os.Setenv("ELEMENTAL_LABEL", "environment")
	cfg, err := ReadConfigBuild("config/")
	Expect(err).To(BeNil())
	source := viper.GetString("label")
	// check that the final value comes from the env var
	Expect(source).To(Equal("environment"))
	Expect(cfg.Label).To(Equal("environment"))
}
