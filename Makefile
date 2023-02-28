# Directory of Makefile
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# Elemental client version to use
ELEMENTAL_CLI?=v0.2.1

PACKER?=$(shell which packer 2> /dev/null)
ifeq ("$(PACKER)","")
PACKER="/usr/bin/packer"
endif

QCOW2=$(shell ls $(ROOT_DIR)/build/*.qcow2 2> /dev/null)
ISO?=$(shell ls $(ROOT_DIR)/build/*.iso 2> /dev/null)
PACKER_TARGET?=qemu.cos
FLAVOR?=green
ARCH?=x86_64
GINKGO_ARGS?=-progress -v --fail-fast -r --timeout=3h
VERSION?=$(shell git describe --tags)
ifeq ("$(PACKER)","")
VERSION="latest"
endif
REPO?=local/elemental-$(FLAVOR)


#----------------------- includes -----------------------

include make/Makefile.test


#----------------------- targets ------------------------

.PHONY: build
build:
	docker build toolkit --build-arg=$(ELEMENTAL_CLI) -t local/elemental-toolkit

.PHONY: build-example-os
build-example-os: build
	mkdir -p $(ROOT_DIR)/build
	docker build examples/$(FLAVOR) --build-arg VERSION=$(VERSION) --build-arg REPO=$(REPO) -t $(REPO):$(VERSION)

.PHONY: build-example-iso
build-example-iso: build-example-os
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v $(ROOT_DIR)/build:/build \
		--entrypoint /usr/bin/elemental $(REPO):$(VERSION) --debug build-iso --bootloader-in-rootfs -n elemental-$(FLAVOR) \
		--local --squash-no-compression -o /build $(REPO):$(VERSION)

.PHONY: clean-iso
clean-iso: build-example-os
	docker run --rm -v $(ROOT_DIR)/build:/build --entrypoint /bin/bash $(REPO):$(VERSION) -c "rm -v /build/*.iso /build/*.iso.sha256 || true"

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
	export PKR_VAR_iso=$(ISO) && export PKR_VAR_flavor=$(FLAVOR) && cd $(ROOT_DIR)/packer && $(PACKER) build -only $(PACKER_TARGET) .
	mv $(ROOT_DIR)/packer/build/*.qcow2 $(ROOT_DIR)/build && rm -rf $(ROOT_DIR)/packer/build

.PHONY: packer-clean
packer-clean:
	rm -rf $(ROOT_DIR)/packer/build
	rm -f $(ROOT_DIR)/build/.*qcow2
