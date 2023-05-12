# Directory of Makefile
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# Elemental client version to use
ELEMENTAL_CLI?=HEAD

QCOW2=$(shell ls $(ROOT_DIR)/build/*.qcow2 2> /dev/null)
ISO?=$(shell ls $(ROOT_DIR)/build/*.iso 2> /dev/null)
FLAVOR?=green
ARCH?=x86_64
PLATFORM?=linux/$(ARCH)
IMAGE_SIZE?=20G
PACKER_TARGET?=qemu.elemental-${ARCH}
VERSION?=latest
REPO?=local/elemental-$(FLAVOR)
DOCKER?=docker

GIT_COMMIT ?= $(shell git rev-parse HEAD)
GIT_COMMIT_SHORT ?= $(shell git rev-parse --short HEAD)
GIT_TAG ?= $(shell git describe --abbrev=0 --tags 2>/dev/null || echo "v0.0.1" )

PKG        := ./cmd ./pkg
LDFLAGS    := -w -s
LDFLAGS += -X "github.com/rancher/elemental-cli/internal/version.version=${GIT_TAG}"
LDFLAGS += -X "github.com/rancher/elemental-cli/internal/version.gitCommit=${GIT_COMMIT}"

# default target
.PHONY: all
all: build build-cli

#----------------------- includes -----------------------

include make/Makefile.test

#----------------------- targets ------------------------

.PHONY: build
build:
	$(DOCKER) build --build-arg ELEMENTAL_VERSION=${GIT_TAG} --build-arg ELEMENTAL_COMMIT=${GIT_COMMIT} --target elemental -t local/elemental-toolkit .

.PHONY: build-cli
build-cli:
	go build -ldflags '$(LDFLAGS)' -o build/elemental

.PHONY: build-os
build-os: build
	$(DOCKER) build examples/$(FLAVOR) --build-arg VERSION=$(VERSION) --build-arg REPO=$(REPO) -t $(REPO):$(VERSION)

.PHONY: build-iso
build-iso: build-os
	mkdir -p $(ROOT_DIR)/build
	$(DOCKER) run --rm -v /var/run/docker.sock:/var/run/docker.sock -v $(ROOT_DIR)/build:/build \
		--entrypoint /usr/bin/elemental $(REPO):$(VERSION) --debug build-iso --bootloader-in-rootfs -n elemental-$(FLAVOR).$(ARCH) \
		--local --platform $(PLATFORM) --squash-no-compression -o /build $(REPO):$(VERSION)

.PHONY: clean-iso
clean-iso: build-example-os
	$(DOCKER) run --rm -v $(ROOT_DIR)/build:/build --entrypoint /bin/bash $(REPO):$(VERSION) -c "rm -v /build/*.iso /build/*.iso.sha256 || true"

.PHONY: build-disk
build-disk: build-os
	mkdir -p $(ROOT_DIR)/build
	qemu-img create -f raw build/elemental-$(FLAVOR).$(ARCH).img $(IMAGE_SIZE)
	- losetup -f --show build/elemental-$(FLAVOR).$(ARCH).img > .loop
	$(DOCKER) run --rm --privileged --device=$$(cat .loop):$$(cat .loop) -v /var/run/docker.sock:/var/run/docker.sock \
		$(REPO):$(VERSION) /bin/bash -c "mount -t devtmpfs none /dev && elemental --debug install --firmware efi \
		--system.uri $(REPO):$(VERSION) --local --disable-boot-entry --platform $(PLATFORM) $$(cat .loop)"
	losetup -d $$(cat .loop)
	rm .loop
	qemu-img convert -O qcow2 build/elemental-$(FLAVOR).$(ARCH).img build/elemental-$(FLAVOR).$(ARCH).qcow2
	rm build/elemental-$(FLAVOR).$(ARCH).img

.PHONY: clean-disk
clean-disk:
	losetup -d $$(cat .loop)
	rm build/*.img
	rm .loop

.PHONY: vet
vet:
	go vet ${PKG}

.PHONY: fmt
fmt:
ifneq ($(shell go fmt ${PKG}),)
	@echo "Please commit the changes from 'make fmt'"
	@exit 1
else
	@echo "All files formatted"
	@exit 0
endif

.PHONY: license-check
license-check:
	@.github/license_check.sh

.PHONY: build-docs
build-docs:
	cd docs && go run generate_docs.go

.PHONY: lint
lint: fmt vet
