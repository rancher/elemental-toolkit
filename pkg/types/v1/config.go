package v1

type RunConfig struct {
	Device string `yaml:"device,omitempty" mapstructure:"device"`
	Target string `yaml:"target,omitempty" mapstructure:"target"`
	Source string `yaml:"source,omitempty" mapstructure:"source"`
	CloudInit string `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
}

type BuildConfig struct {
	Label string `yaml:"label,omitempty" mapstructure:"label"`
}