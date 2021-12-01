package utils

import (
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"os"
	"testing"
)

func setupTest(t *testing.T) {
	viper.Reset()
	RegisterTestingT(t)
}


func TestConfigRun(t *testing.T) {
	setupTest(t)
	// Load only main config
	err := ReadConfigRun("config/")
	Expect(err).To(BeNil())
	source := viper.GetString("file")
	// check that the final value comes from the main file
	Expect(source).To(Equal("main"))
}

func TestConfigRunCustomNotValidPath(t *testing.T) {
	setupTest(t)
	// Load only main config
	err := ReadConfigRun("/none/")
	Expect(err).ToNot(BeNil())
	source := viper.GetString("file")
	Expect(source).To(Equal(""))
}

func TestConfigRunCustomEmptyPath(t *testing.T) {
	setupTest(t)
	// Load only main config
	err := ReadConfigRun()
	Expect(err).ToNot(BeNil())
	source := viper.GetString("file")
	Expect(source).To(Equal(""))
}

func TestConfigRunOverride(t *testing.T) {
	setupTest(t)
	err := ReadConfigRun("config/", "config/config.d/")
	Expect(err).To(BeNil())
	source := viper.GetString("file")
	// check that the final value comes from the extra file
	Expect(source).To(Equal("extra"))
}

func TestConfigRunOverrideEnv(t *testing.T) {
	setupTest(t)
	err := ReadConfigRun("config/", "config/config.d/")
	Expect(err).To(BeNil())
	_= os.Setenv("ELEMENTAL_FILE", "environment")
	source := viper.GetString("file")
	// check that the final value comes from the env var
	Expect(source).To(Equal("environment"))
}

func TestConfigBuildCustomNotValidPath(t *testing.T)  {
	setupTest(t)
	err := ReadConfigBuild("/none/")
	Expect(err).ToNot(BeNil())
	Expect(viper.GetString("label")).To(Equal(""))
}

func TestConfigBuildCustomPath(t *testing.T)  {
	setupTest(t)
	err := ReadConfigBuild("config/")
	Expect(err).To(BeNil())
	Expect(viper.GetString("label")).To(Equal("COS_LIVE"))
}

func TestConfigBuildCustomEmptyPath(t *testing.T)  {
	setupTest(t)
	// Point to nothing, it will search the current path, should not find the config
	err := ReadConfigBuild()
	Expect(err).ToNot(BeNil())
	Expect(viper.GetString("label")).To(Equal(""))
}

func TestConfigBuildOverrideEnv(t *testing.T) {
	setupTest(t)
	err := ReadConfigBuild("config/")
	Expect(err).To(BeNil())
	_= os.Setenv("ELEMENTAL_LABEL", "environment")
	source := viper.GetString("label")
	// check that the final value comes from the env var
	Expect(source).To(Equal("environment"))
}