#
# Directory of Makefile
#
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

.PHONY: build
build:
	docker build toolkit -t local/elemental-toolkit

# TODO requires local/elemental-cli to be built, we should find
# a way to build it here, I guess elemetal-toolkit and elemental-cli repos
# should be merged.
.PHONY: build-green
build-green: build
	docker build examples/green -t local/elemental-green

# TODO requires local/elemental-cli to be built, we should find
# a way to build it here, I guess elemetal-toolkit and elemental-cli repos
# should be merged.
.PHONY: build-green-iso
build-green-iso: build-green
	docker run --rm -ti -v /var/run/docker.sock:/var/run/docker.sock -v $(ROOT_DIR)/build:/build local/elemental-cli --debug build-iso --bootloader-in-rootfs -n elemental-green --date --local --squash-no-compression -o /build local/elemental-green 


VAGRANT?=$(shell which vagrant 2> /dev/null)
ifeq ("$(VAGRANT)","")
VAGRANT="/usr/bin/vagrant"
endif

VBOXMANAGE?=$(shell which VBoxManage 2> /dev/null)
ifeq ("$(VBOXMANAGE)","")
VBOXMANAGE="/usr/bin/VBoxManage"
endif

PACKER?=$(shell which packer 2> /dev/null)
ifeq ("$(PACKER)","")
PACKER="/usr/bin/packer"
endif

BOXFILE=$(shell ls $(ROOT_DIR)/packer/*$(ARCH).box 2> /dev/null)
ifeq ("$(BOXFILE)","")
BOXFILE="$(ROOT_DIR)/packer/cOS.box"
endif

ISO?=$(shell ls $(ROOT_DIR)/build/*.iso 2> /dev/null)

PACKER_TARGET?=qemu.cos

#
# target 'packer' creates a compressed tarball with an 'ova' file
#
.PHONY: packer
packer: $(BOXFILE)

.PHONY: packer-clean
packer-clean:
	rm -rf $(BOXFILE)

$(BOXFILE): $(PACKER)
ifeq ("$(PACKER)","/usr/sbin/packer")
	@echo "The 'packer' binary at $(PACKER) might be from cracklib"
	@echo "Please set PACKER to the correct binary before calling make"
	@exit 1
endif
	export PKR_VAR_iso=$(ISO) && cd $(ROOT_DIR)/packer && $(PACKER) build -only $(PACKER_TARGET) .

vagrantfile: $(ROOT_DIR)/tests/Vagrantfile $(VAGRANT)

$(ROOT_DIR)/tests/Vagrantfile: $(VAGRANT)
	cd $(ROOT_DIR)/tests && vagrant init cos

prepare-test: $(VAGRANT) $(BOXFILE)
	vagrant box add --force cos $(BOXFILE)
	cd $(ROOT_DIR)/tests && vagrant up $(VMNAME) || true

test-clean:
	(cd $(ROOT_DIR)/tests && vagrant destroy) 2> /dev/null || true
	(vagrant box remove cos) 2> /dev/null || true
