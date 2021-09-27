module github.com/rancher-sandbox/cOS-toolkit/github

go 1.14

require (
	github.com/google/go-containerregistry v0.5.1
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/mudler/luet v0.0.0-20210922074617-2970d8e52e6f
)

replace github.com/docker/docker => github.com/Luet-lab/moby v17.12.0-ce-rc1.0.20200605210607-749178b8f80d+incompatible

replace github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200227195959-4d242818bf55

replace github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.0-rc9.0.20200221051241-688cf6d43cc4

replace github.com/googleapis/gnostic => github.com/google/gnostic v0.3.1
