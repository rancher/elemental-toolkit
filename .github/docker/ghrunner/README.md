# GHRUNNER

This is a small Dockerfile providing a github runner.
The images can be found `ghcr.io/dragonchaser/ghrunner:latest`. 
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

