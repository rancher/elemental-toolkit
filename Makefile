# Directory of Makefile
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# Elemental client version to use
ELEMENTAL_CLI?=HEAD

PACKER?=$(shell which packer 2> /dev/null)
ifeq ("$(PACKER)","")
PACKER="/usr/bin/packer"
endif

QCOW2=$(shell ls $(ROOT_DIR)/build/*.qcow2 2> /dev/null)
ISO?=$(shell ls $(ROOT_DIR)/build/*.iso 2> /dev/null)
FLAVOR?=green
ARCH?=x86_64
PLATFORM?=linux/$(ARCH)
IMAGE_SIZE?=32G
PACKER_TARGET?=qemu.elemental-${ARCH}
GINKGO_ARGS?=-v --fail-fast -r --timeout=3h
VERSION?=$(shell git describe --tags)
ifeq ("$(PACKER)","")
VERSION="latest"
endif
REPO?=local/elemental-$(FLAVOR)
DOCKER?=docker

ifeq ("$(ARCH)","arm64")
PACKER_ACCELERATOR?=none
PACKER_CPU?=
endif
PACKER_ACCELERATOR?=

PACKER_CPU?=host
PACKER_FIRMWARE?=/usr/share/qemu/ovmf-x86_64-ms-code.bin

# default target
.PHONY: all
all: build

#----------------------- includes -----------------------

include make/Makefile.test

#----------------------- targets ------------------------

.PHONY: build
build:
	$(DOCKER) build toolkit --platform $(ARCH) --build-arg=ELEMENTAL_REVISION=$(ELEMENTAL_CLI) -t local/elemental-toolkit

.PHONY: build-example-os
build-example-os: build
	mkdir -p $(ROOT_DIR)/build
	$(DOCKER) build examples/$(FLAVOR) --platform $(ARCH) --build-arg VERSION=$(VERSION) --build-arg REPO=$(REPO) -t $(REPO):$(VERSION)

.PHONY: build-example-iso
build-example-iso: build-example-os
	$(DOCKER) run --rm -v /var/run/docker.sock:/var/run/docker.sock -v $(ROOT_DIR)/build:/build \
		--entrypoint /usr/bin/elemental $(REPO):$(VERSION) --debug build-iso --bootloader-in-rootfs -n elemental-$(FLAVOR).$(ARCH) \
		--local --platform $(PLATFORM) --squash-no-compression -o /build $(REPO):$(VERSION)

.PHONY: clean-iso
clean-iso: build-example-os
	$(DOCKER) run --rm -v $(ROOT_DIR)/build:/build --entrypoint /bin/bash $(REPO):$(VERSION) -c "rm -v /build/*.iso /build/*.iso.sha256 || true"

.PHONY: image
image:
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

.PHONY: packer
packer:
ifeq ("$(PACKER)","/usr/sbin/packer")
	@echo "The 'packer' binary at $(PACKER) might be from cracklib"
	@echo "Please set PACKER to the correct binary before calling make"
	@exit 1
endif
ifeq ("$(ISO)","")
	@echo "No ISO image found"
	@exit 1
endif
	cd $(ROOT_DIR)/packer && PKR_VAR_accelerator=$(PACKER_ACCELERATOR) PKR_VAR_cpu_model=$(PACKER_CPU) PKR_VAR_iso=$(ISO) PKR_VAR_iso_checksum=file:$(ISO).sha256 PKR_VAR_flavor=$(FLAVOR) PKR_VAR_firmware=$(PACKER_FIRMWARE) PACKER_LOG=1 $(PACKER) build -only $(PACKER_TARGET) .
	mv -f $(ROOT_DIR)/packer/build/*.qcow2 $(ROOT_DIR)/build && rm -rf $(ROOT_DIR)/packer/build

.PHONY: packer-clean
packer-clean:
	rm -rf $(ROOT_DIR)/packer/build
	rm -f $(ROOT_DIR)/build/.*qcow2
