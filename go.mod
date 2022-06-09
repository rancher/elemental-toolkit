module github.com/rancher/elemental-cli

go 1.16

// until https://github.com/zloylos/grsync/pull/20 is merged we need to use our fork
replace github.com/zloylos/grsync v1.6.1 => github.com/rancher-sandbox/grsync v1.6.2-0.20220526080038-4032e9b0e97c

require (
	github.com/Masterminds/sprig/v3 v3.2.2 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20220113124808-70ae35bab23f // indirect
	github.com/cavaliergopher/grab/v3 v3.0.1
	github.com/distribution/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-units v0.4.0
	github.com/hashicorp/go-getter v1.5.11
	github.com/hashicorp/go-multierror v1.1.1
	github.com/ishidawataru/sctp v0.0.0-20210707070123-9a39160e9062 // indirect
	github.com/itchyny/gojq v0.12.6 // indirect
	github.com/jaypipes/ghw v0.9.1-0.20220404152016-2ea05cb6c17c
	github.com/joho/godotenv v1.4.0
	github.com/kevinburke/ssh_config v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3
	github.com/mudler/go-pluggable v0.0.0-20211206135551-9263b05c562e
	github.com/mudler/luet v0.0.0-20220526130937-264bf53fe7ab
	github.com/mudler/yip v0.0.0-20220321143540-2617d71ea02a
	github.com/onsi/ginkgo/v2 v2.1.4
	github.com/onsi/gomega v1.19.0
	github.com/packethost/packngo v0.21.0 // indirect
	github.com/sanity-io/litter v1.5.5
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.0
	github.com/twpayne/go-vfs v1.7.2
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	github.com/xanzy/ssh-agent v0.3.1 // indirect
	github.com/zcalusic/sysinfo v0.0.0-20210905121133-6fa2f969a900 // indirect
	github.com/zloylos/grsync v1.6.1
	golang.org/x/crypto v0.0.0-20220126234351-aa10faf2a1f8 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220106191415-9b9b3d81d5e3 // indirect
	golang.org/x/net v0.0.0-20220607020251-c690dde0001d // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	gopkg.in/ini.v1 v1.66.3 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/mount-utils v0.23.0
	pault.ag/go/topsort v0.1.1 // indirect
)
