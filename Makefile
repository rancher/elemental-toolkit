# Directory of Makefile
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

QCOW2?=$(shell ls $(ROOT_DIR)/build/*.qcow2 2> /dev/null)
ISO?=$(shell ls $(ROOT_DIR)/build/*.iso 2> /dev/null)
FLAVOR?=green
ARCH?=x86_64
PLATFORM?=linux/$(ARCH)
IMAGE_SIZE?=20G
PACKER_TARGET?=qemu.elemental-${ARCH}
REPO?=local/elemental-$(FLAVOR)
TOOLKIT_REPO?=local/elemental-toolkit
DOCKER?=docker

GIT_COMMIT ?= $(shell git rev-parse HEAD)
GIT_COMMIT_SHORT ?= $(shell git rev-parse --short HEAD)
GIT_TAG ?= $(shell git describe --candidates=50 --abbrev=0 --tags 2>/dev/null || echo "v0.0.1" )
VERSION ?= ${GIT_TAG}-g${GIT_COMMIT_SHORT}

PKG        := ./cmd ./pkg/...
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
	$(DOCKER) build --build-arg ELEMENTAL_VERSION=${GIT_TAG} --platform ${PLATFORM} --build-arg ELEMENTAL_COMMIT=${GIT_COMMIT} --target elemental -t ${TOOLKIT_REPO}:${VERSION} .

.PHONY: push-toolkit
push-toolkit:
	$(DOCKER) push $(TOOLKIT_REPO):$(VERSION)

.PHONY: build-cli
build-cli:
	go build -ldflags '$(LDFLAGS)' -o build/elemental

.PHONY: build-os
build-os: build
	$(DOCKER) build examples/$(FLAVOR) --platform ${PLATFORM} --build-arg TOOLKIT_REPO=$(TOOLKIT_REPO) --build-arg VERSION=$(VERSION) --build-arg REPO=$(REPO) -t $(REPO):$(VERSION)

.PHONY: push-os
push-os:
	$(DOCKER) push $(REPO):$(VERSION)

.PHONY: build-iso
build-iso: build-os
	@echo Building ${ARCH} ISO
	mkdir -p $(ROOT_DIR)/build
	$(DOCKER) run --rm -v /var/run/docker.sock:/var/run/docker.sock -v $(ROOT_DIR)/build:/build \
		--entrypoint /usr/bin/elemental $(REPO):$(VERSION) --debug build-iso --bootloader-in-rootfs -n elemental-$(FLAVOR).$(ARCH) \
		--local --platform $(PLATFORM) --squash-no-compression -o /build $(REPO):$(VERSION)

.PHONY: clean-iso
clean-iso: build-example-os
	$(DOCKER) run --rm -v $(ROOT_DIR)/build:/build --entrypoint /bin/bash $(REPO):$(VERSION) -c "rm -v /build/*.iso /build/*.iso.sha256 || true"

.PHONY: build-disk
build-disk: build-os
	@echo Building ${ARCH} disk
	mkdir -p $(ROOT_DIR)/build
	qemu-img create -f raw build/elemental-$(FLAVOR).$(ARCH).img $(IMAGE_SIZE)
	- losetup -f --show build/elemental-$(FLAVOR).$(ARCH).img > .loop
	$(DOCKER) run --rm --privileged --device=$$(cat .loop):$$(cat .loop) -v /var/run/docker.sock:/var/run/docker.sock \
		--entrypoint=/bin/bash $(TOOLKIT_REPO):$(VERSION) -c "mount -t devtmpfs none /dev && \
		elemental --debug install --firmware efi --system.uri $(REPO):$(VERSION) --local --disable-boot-entry --platform $(PLATFORM) $$(cat .loop)"
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

.PHONY: lint
lint: fmt vet

.PHONY: build-docs
build-docs:
	@./scripts/docs-build.sh
	cd docs && go run generate_docs.go
