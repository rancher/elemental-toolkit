#
# Directory of Makefile
#
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

PACKER?=$(shell which packer 2> /dev/null)
ifeq ("$(PACKER)","")
PACKER="/usr/bin/packer"
endif

QCOW2=$(shell ls $(ROOT_DIR)/packer/build/*.qcow2 2> /dev/null)
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

.PHONY: build
build:
	docker build toolkit -t local/elemental-toolkit

# TODO requires local/elemental-cli to be built, we should find
# a way to build it here, I guess elemetal-toolkit and elemental-cli repos
# should be merged.
.PHONY: build-example-os
build-example-os: build
	docker build examples/$(FLAVOR) --build-arg VERSION=$(VERSION) --build-arg REPO=$(REPO) -t $(REPO):$(VERSION)

# TODO requires local/elemental-cli to be built, we should find
# a way to build it here, I guess elemetal-toolkit and elemental-cli repos
# should be merged.
.PHONY: build-example-iso
build-example-iso: build-example-os
	docker run --rm -ti -v /var/run/docker.sock:/var/run/docker.sock -v $(ROOT_DIR)/build:/build local/elemental-cli --debug build-iso --bootloader-in-rootfs -n elemental-$(FLAVOR) --date --local --squash-no-compression -o /build $(REPO):$(VERSION)

.PHONY: clean-iso
clean-iso:
	docker run --rm -ti -v $(ROOT_DIR)/build:/build --entrypoint /bin/bash local/elemental-cli -c "rm -v /build/*.iso /build/*.iso.sha256 || true"

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

.PHONY: packer-clean
packer-clean:
	rm -rf $(ROOT_DIR)/packer/build

.PHONY: prepare-test
prepare-test:
ifeq ("$(QCOW2)","")
	@echo "No qcow2 disk found, run make packer first"
	@exit 1
endif
	@scripts/run_vm.sh start $(QCOW2)
	@echo "VM started from $(QCOW2)"

# TODO this target is leaving behind the machine base disk in libvirt storage, we should either delete it
# or make user it gets overwritten on every 'vagrant box add --force cos'
test-clean:
	@scripts/run_vm.sh stop
	@scripts/run_vm.sh clean

test-smoke: prepare-test
	cd tests && go run github.com/onsi/ginkgo/v2/ginkgo $(GINKGO_ARGS) ./smoke

