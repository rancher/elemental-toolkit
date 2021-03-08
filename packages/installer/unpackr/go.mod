module github.com/mudler/cOS/packages/installer/unpackr

go 1.14

require (
	github.com/Microsoft/go-winio v0.4.15-0.20200113171025-3fe6c5262873 // indirect
	github.com/containerd/containerd v1.4.1-0.20201117152358-0edc412565dc
	github.com/containerd/continuity v0.0.0-20200228182428-0f16d7a0959c // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.0-beta1.0.20201110211921-af34b94a78a1+incompatible
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-units v0.4.0
	github.com/genuinetools/img v0.5.11
	github.com/gofrs/flock v0.7.1 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/google/go-cmp v0.4.1 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/moby/buildkit v0.7.2
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.0.8 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1 // indirect
	go.etcd.io/bbolt v1.3.5
	go.opencensus.io v0.22.3 // indirect
	golang.org/x/crypto v0.0.0-20201112155050-0c6587e931a9 // indirect
	golang.org/x/net v0.0.0-20201006153459-a7d1128ccaa0 // indirect
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208 // indirect
	golang.org/x/sys v0.0.0-20201113135734-0a15ea8d9b02 // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20200527145253-8367513e4ece // indirect
	google.golang.org/grpc v1.29.1 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect

)

replace github.com/docker/docker => github.com/Luet-lab/moby v17.12.0-ce-rc1.0.20200605210607-749178b8f80d+incompatible

replace github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200227195959-4d242818bf55

replace github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.0-rc9.0.20200221051241-688cf6d43cc4
