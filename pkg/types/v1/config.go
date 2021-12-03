package v1

type RunConfig struct {
	Device string `yaml:"device,omitempty" mapstructure:"device"`
	Target string `yaml:"target,omitempty" mapstructure:"target"`
	Source string `yaml:"source,omitempty" mapstructure:"source"`
	CloudInit string `yaml:"cloud-init,omitempty" mapstructure:"cloud-init"`
	ForceEfi bool `yaml:"force-efi,omitempty" mapstructure:"force-efi"`
	ForceGpt bool `yaml:"force-gpt,omitempty" mapstructure:"force-gpt"`
	PartTable string
	BootFlag string
}

type BuildConfig struct {
	Label string `yaml:"label,omitempty" mapstructure:"label"`
}