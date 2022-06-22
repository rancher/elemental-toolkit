package config

type GrowPart struct {
	Mode    string   `yaml:"mode,omitempty"`
	Devices []string `yaml:"devices"`
}
