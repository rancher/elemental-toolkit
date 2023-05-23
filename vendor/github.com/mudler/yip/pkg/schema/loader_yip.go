package schema

import (
	"github.com/twpayne/go-vfs"
	"gopkg.in/yaml.v2"
)

type yipYAML struct{}

// LoadFromYaml loads a yip config from bytes
func (yipYAML) Load(b []byte, fs vfs.FS) (*YipConfig, error) {
	var yamlConfig YipConfig
	err := yaml.Unmarshal(b, &yamlConfig)
	if err != nil {
		return nil, err
	}

	return &yamlConfig, nil
}
