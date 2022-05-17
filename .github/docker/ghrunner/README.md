# GHRUNNER

This is a small Dockerfile providing a github runner.
The images can be found `quay.io/costoolkit/ghrunner:latest`. 
Supported arches are:
  - aarch64
  - amd64

## Environment variables
```terminal
| Variable   | Values                                 | default        |
|------------+----------------------------------------+----------------|
| `TOKEN`    | token for runner from github           | <unset>        |
| `ARCH`     | arm64, x64                             | x64            |
| `OS`       | linux,osx                              | linux          |
| `ORG`      | name of the github org                 | <unset>        |
| `REPO`     | name of the github repo                | <unset>        |
| `VERSION`  | version of the github runner to user   | 2.280.3        |
| `CHECKSUM` | checksum for the github runner version | valid checksum |
```

## Setup

On the machines running the github the following are required:

- docker (`zypper in -y docker`)
- a time sync daemon

## Summary

For example, the following steps works for openSUSE:

```bash
$ zypper in -y docker
$ systemctl enable --now docker
$ systemctl enable --now systemd-timesyncd
```

To run the action runner with docker, for example it is necessary just to specify all the settings with environment variables (and share the docker socket):

```bash
docker run -e TOKEN=<TOKEN> -e ARCH=<ARCH> -e ORG=<ORG> -e REPO=<REPO> -e VERSION=<VERSION> -e CHECKSUM=<CHECKSUM> -v /var/run:/var/run -d --rm quay.io/costoolkit/ghrunner:latest
```
