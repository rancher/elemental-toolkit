module github.com/rancher/elemental-cli

go 1.16

// until https://github.com/zloylos/grsync/pull/20 is merged we need to use our fork
replace github.com/zloylos/grsync v1.6.1 => github.com/rancher-sandbox/grsync v1.6.2-0.20220526080038-4032e9b0e97c

require (
	github.com/cavaliergopher/grab/v3 v3.0.1
	github.com/distribution/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.17+incompatible
	github.com/docker/go-units v0.4.0
	github.com/hashicorp/go-getter v1.5.11
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jaypipes/ghw v0.9.1-0.20220511134554-dac2f19e1c76
	github.com/joho/godotenv v1.4.0
	github.com/mitchellh/mapstructure v1.4.3
	github.com/mudler/go-pluggable v0.0.0-20211206135551-9263b05c562e
	github.com/mudler/luet v0.0.0-20220526130937-264bf53fe7ab
	github.com/mudler/yip v0.0.0-20220704150701-30d215fa4ab0
	github.com/onsi/ginkgo/v2 v2.1.4
	github.com/onsi/gomega v1.20.1
	github.com/sanity-io/litter v1.5.5
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.0
	github.com/twpayne/go-vfs v1.7.2
	github.com/zloylos/grsync v1.6.1
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/mount-utils v0.23.0
)
