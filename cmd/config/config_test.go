package config

import (
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"testing"
)

func setupTest(t *testing.T) {
	viper.Reset()
	RegisterTestingT(t)
}

var logger = logrus.New()

func TestConfigRunCustomNotValidPath(t *testing.T) {
	setupTest(t)
	// Load only main config
	cfg, err := ReadConfigRun("/none/", logger)
	Expect(err).To(BeNil())
	source := viper.GetString("file")
	Expect(source).To(Equal(""))
	Expect(cfg.Source).To(Equal(""))
}

func TestConfigRunCustomEmptyPath(t *testing.T) {
	setupTest(t)
	// Load only main config
	cfg, err := ReadConfigRun("", logger)
	Expect(err).To(BeNil())
	source := viper.GetString("file")
	Expect(source).To(Equal(""))
	Expect(cfg.Source).To(Equal(""))
}

func TestConfigRunOverride(t *testing.T) {
	setupTest(t)
	cfg, err := ReadConfigRun("config/", logger)
	Expect(err).To(BeNil())
	source := viper.GetString("target")
	// check that the final value comes from the extra file
	Expect(source).To(Equal("extra"))
	Expect(cfg.Target).To(Equal("extra"))
}

func TestConfigRunOverrideEnv(t *testing.T) {
	setupTest(t)
	_ = os.Setenv("ELEMENTAL_TARGET", "environment")
	cfg, err := ReadConfigRun("config/", logger)
	Expect(err).To(BeNil())
	source := viper.GetString("target")
	// check that the final value comes from the env var
	Expect(source).To(Equal("environment"))
	Expect(cfg.Target).To(Equal("environment"))
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
