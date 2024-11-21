# Directory of Makefile
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

DISK?=$(shell ls $(ROOT_DIR)/build/elemental*.qcow2 2> /dev/null)
DISKSIZE?=20G
ISO?=$(shell ls $(ROOT_DIR)/build/elemental*.iso 2> /dev/null)
FLAVOR?=green
ifdef PLATFORM
ARCH=$(subst linux/,,$(PLATFORM))
else
ARCH?=$(shell uname -m)
endif
UPGRADE_DISK_URL?=https://github.com/rancher/elemental-toolkit/releases/download/v1.1.4/elemental-$(FLAVOR)-v1.1.4.$(ARCH).qcow2
UPGRADE_DISK?=upgrade-test-elemental-disk-$(FLAVOR).qcow2
UPGRADE_DISK_CHECK?=$(shell ls $(ROOT_DIR)/build/$(UPGRADE_DISK) 2> /dev/null)
PLATFORM?=linux/$(ARCH)
IMAGE_SIZE?=20G
REPO?=local/elemental-$(FLAVOR)
TOOLKIT_REPO?=local/elemental-toolkit
DOCKER?=docker
BASE_OS_IMAGE?=registry.opensuse.org/opensuse/tumbleweed
DOCKER_SOCK?=/var/run/docker.sock

GIT_COMMIT?=$(shell git rev-parse HEAD)
GIT_COMMIT_SHORT?=$(shell git rev-parse --short HEAD)
GIT_TAG?=$(shell git describe --candidates=50 --abbrev=0 --tags 2>/dev/null || echo "v0.0.1" )
VERSION?=$(GIT_TAG)-g$(GIT_COMMIT_SHORT)

PKG:=./cmd ./pkg/...
LDFLAGS:=-w -s
LDFLAGS+=-X "github.com/rancher/elemental-toolkit/v2/internal/version.version=$(GIT_TAG)"
LDFLAGS+=-X "github.com/rancher/elemental-toolkit/v2/internal/version.gitCommit=$(GIT_COMMIT)"

# default target
.PHONY: all
all: build build-cli

#----------------------- includes -----------------------

include make/Makefile.test

#----------------------- targets ------------------------

.PHONY: build
build:
	@echo Building $(ARCH) Image
	$(DOCKER) build --platform $(PLATFORM) ${DOCKER_ARGS} \
			--build-arg ELEMENTAL_VERSION=$(GIT_TAG) \
			--build-arg ELEMENTAL_COMMIT=$(GIT_COMMIT) \
			--build-arg BASE_OS_IMAGE=$(BASE_OS_IMAGE) \
			--target elemental-toolkit -t $(TOOLKIT_REPO):$(VERSION) .

.PHONY: build-save
build-save: build
	@echo Saving $(ARCH) Image
	mkdir -p $(ROOT_DIR)/build
	$(DOCKER) save --output build/elemental-toolkit-image-$(VERSION).tar \
			$(TOOLKIT_REPO):$(VERSION)

.PHONY: build-cli
build-cli:
	@echo Building $(ARCH) CLI
	go generate ./...
	go build -ldflags '$(LDFLAGS)' -o build/elemental

.PHONY: build-os
build-os:
	@echo Building $(ARCH) OS
	$(DOCKER) build --platform $(PLATFORM) ${DOCKER_ARGS} \
			--build-arg TOOLKIT_REPO=$(TOOLKIT_REPO) \
			--build-arg VERSION=$(VERSION) \
			--build-arg REPO=$(REPO) -t $(REPO):$(VERSION) \
			$(BUILD_OPTS) examples/$(FLAVOR)
.PHONY: build-os-save
build-os-save: build-os
	@echo Saving $(ARCH) OS
	mkdir -p $(ROOT_DIR)/build
	$(DOCKER) save --output build/elemental-$(FLAVOR)-image-$(VERSION).tar \
			$(REPO):$(VERSION)

.PHONY: build-iso
build-iso:
	@echo Building $(ARCH) ISO
	mkdir -p $(ROOT_DIR)/build
	$(DOCKER) run --rm -v $(DOCKER_SOCK):$(DOCKER_SOCK) -v $(ROOT_DIR)/build:/build \
		-v $(ROOT_DIR)/tests/assets/remote_login.yaml:/overlay-iso/iso-config/remote_login.yaml \
		--entrypoint /usr/bin/elemental $(TOOLKIT_REPO):$(VERSION) --debug build-iso \
		-n elemental-$(FLAVOR).$(ARCH) --overlay-iso /overlay-iso \
		--local --platform $(PLATFORM) -o /build $(REPO):$(VERSION)

.PHONY: build-disk
build-disk:
	@echo Building $(ARCH) disk
	mkdir -p $(ROOT_DIR)/build
	$(DOCKER) run --rm -v $(DOCKER_SOCK):$(DOCKER_SOCK) -v $(ROOT_DIR)/build:/build -v $(ROOT_DIR)/tests/assets:/assets \
		--entrypoint /usr/bin/elemental $(TOOLKIT_REPO):$(VERSION) --debug build-disk --platform $(PLATFORM) \
		--expandable -n elemental-$(FLAVOR).$(ARCH) --local --cloud-init /assets/remote_login.yaml -o /build --system $(REPO):$(VERSION)
	qemu-img convert -O qcow2 $(ROOT_DIR)/build/elemental-$(FLAVOR).$(ARCH).raw $(ROOT_DIR)/build/elemental-$(FLAVOR).$(ARCH).qcow2
	qemu-img resize $(ROOT_DIR)/build/elemental-$(FLAVOR).$(ARCH).qcow2 $(DISKSIZE) 

.PHONY: build-rpi-disk
build-rpi-disk:
ifneq ("$(PLATFORM)","linux/arm64")
	@echo "Cannot build Raspberry Pi disk for $(PLATFORM)"
	@exit 1
endif
	@echo Building $(ARCH) disk
	mkdir -p $(ROOT_DIR)/build
	$(DOCKER) run --rm -v $(DOCKER_SOCK):$(DOCKER_SOCK) -v $(ROOT_DIR)/examples:/examples -v $(ROOT_DIR)/build:/build \
		--entrypoint /usr/bin/elemental \
		$(TOOLKIT_REPO):$(VERSION) --debug build-disk --platform $(PLATFORM) --cloud-init-paths /examples/$(FLAVOR) --expandable -n elemental-$(FLAVOR).aarch64 --local \
		--squash-no-compression -o /build --system $(REPO):$(VERSION)

.PHONY: clean
clean:
	rm -fv $(ROOT_DIR)/build/elemental-$(FLAVOR).$(ARCH).{raw,img,qcow2,iso,iso.sha256}

.PHONY: vet
vet:
	go vet $(PKG)

.PHONY: fmt
fmt:
ifneq ($(shell go fmt $(PKG)),)
	@echo "Please commit the changes from 'make fmt'"
	@exit 1
else
	@echo "All files formatted"
	@exit 0
endif

.PHONY: license-check
license-check:
	@.github/license_check.sh

.PHONY: lint
lint: fmt vet

.PHONY: build-docs
build-docs:
	@./scripts/docs-build.sh
	go generate ./...
	cd docs && go run generate_docs.go
