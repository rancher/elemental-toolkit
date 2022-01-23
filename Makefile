GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_TAG = $(shell git describe --tags 2>/dev/null || echo "v0.0.1" )

PKG        := ./...
LDFLAGS    := -w -s
LDFLAGS += -X "github.com/rancher-sandbox/elemental/internal/version.version=${GIT_TAG}"
LDFLAGS += -X "github.com/rancher-sandbox/elemental/internal/version.gitCommit=${GIT_COMMIT}"



build:
	go build -ldflags '$(LDFLAGS)' -o bin/

vet:
	go vet ${PKG}

fmt:
	go fmt ${PKG}

test:
	go test -v ${PKG} -race -coverprofile=coverage.txt -covermode=atomic

license-check:
	@.github/license_check.sh

lint: fmt vet

all: lint test build