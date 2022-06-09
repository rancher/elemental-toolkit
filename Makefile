GIT_COMMIT ?= $(shell git rev-parse HEAD)
GIT_COMMIT_SHORT ?= $(shell git rev-parse --short HEAD)
GIT_TAG ?= $(shell git describe --abbrev=0 --tags 2>/dev/null || echo "v0.0.1" )

PKG        := ./...
LDFLAGS    := -w -s
LDFLAGS += -X "github.com/rancher/elemental-cli/internal/version.version=${GIT_TAG}"
LDFLAGS += -X "github.com/rancher/elemental-cli/internal/version.gitCommit=${GIT_COMMIT}"


GINKGO?=$(shell which ginkgo 2> /dev/null)
ifeq ("$(GINKGO)","")
GINKGO="/usr/bin/ginkgo"
endif

$(GINKGO):
	@echo "'ginkgo' not found."
	@exit 1

build:
	go build -ldflags '$(LDFLAGS)' -o bin/elemental

docker_build:
	DOCKER_BUILDKIT=1 docker build --build-arg ELEMENTAL_VERSION=${GIT_TAG} --build-arg ELEMENTAL_COMMIT=${GIT_COMMIT} --target elemental -t elemental:${GIT_TAG}-${GIT_COMMIT_SHORT} .

vet:
	go vet ${PKG}

fmt:
ifneq ($(shell go fmt ${PKG}),)
	@echo "Please commit the changes from 'make fmt'"
	@exit 1
else
	@echo "All files formatted"
	@exit 0
endif

test_deps:
	go mod download
	go get github.com/onsi/gomega/...
	go get github.com/onsi/ginkgo/v2/ginkgo/internal@v2.1.4
	go get github.com/onsi/ginkgo/v2/ginkgo/generators@v2.1.4
	go get github.com/onsi/ginkgo/v2/ginkgo/labels@v2.1.4
	go install github.com/onsi/ginkgo/v2/ginkgo

test: $(GINKGO)
	ginkgo run --label-filter !root --fail-fast --slow-spec-threshold 30s --race --covermode=atomic --coverprofile=coverage.txt -p -r ${PKG}

test_root: $(GINKGO)
ifneq ($(shell id -u), 0)
	@echo "This tests require root/sudo to run."
	@exit 1
else
	ginkgo run --label-filter root --fail-fast --slow-spec-threshold 30s --race --covermode=atomic --coverprofile=coverage_root.txt -procs=1 -r ${PKG}
endif

# Useful test run for local dev. It does not run tests that require root and it does not run tests that require systemctl checks
# which results in a escalation prompt for privileges. This can block a run until a password or the prompt is cancelled
test_no_root_no_systemctl:
	ginkgo run --label-filter '!root && !systemctl' --fail-fast --slow-spec-threshold 30s --race --covermode=atomic --coverprofile=coverage.txt -p -r ${PKG}

license-check:
	@.github/license_check.sh

build_docs:
	cd docs && go run generate_docs.go

lint: fmt vet

all: build_docs lint test build
