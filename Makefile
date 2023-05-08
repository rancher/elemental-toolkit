# Directory of Makefile
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# Elemental client version to use
ELEMENTAL_CLI?=HEAD

QCOW2=$(shell ls $(ROOT_DIR)/build/*.qcow2 2> /dev/null)
ISO?=$(shell ls $(ROOT_DIR)/build/*.iso 2> /dev/null)
FLAVOR?=green
ARCH?=x86_64
PLATFORM?=linux/$(ARCH)
IMAGE_SIZE?=32G
PACKER_TARGET?=qemu.elemental-${ARCH}
GINKGO_ARGS?=-v --fail-fast -r --timeout=3h
VERSION?=latest
REPO?=local/elemental-$(FLAVOR)
DOCKER?=docker

# default target
.PHONY: all
all: build

#----------------------- includes -----------------------

include make/Makefile.test

#----------------------- targets ------------------------

.PHONY: build
build:
	$(DOCKER) build toolkit --build-arg=ELEMENTAL_REVISION=$(ELEMENTAL_CLI) -t local/elemental-toolkit

.PHONY: build-os
build-os: build
	mkdir -p $(ROOT_DIR)/build
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
	$(DOCKER) run --privileged --device=$$(cat .loop):$$(cat .loop) -v /var/run/docker.sock:/var/run/docker.sock \
		$(REPO):$(VERSION) /bin/bash -c "mount -t devtmpfs none /dev && elemental --debug install \
		--system.uri $(REPO):$(VERSION) --local --disable-boot-entry --platform $(PLATFORM) $$(cat .loop)"
	losetup -d $$(cat .loop)
	rm .loop
	qemu-img convert -O qcow2 build/elemental-$(FLAVOR).$(ARCH).img build/elemental-$(FLAVOR).$(ARCH).qcow2
	rm build/elemental-$(FLAVOR).$(ARCH).img

.PHONY: clean-image
clean-image:
	losetup -d $$(cat .loop)
	rm build/*.img
	rm .loop
