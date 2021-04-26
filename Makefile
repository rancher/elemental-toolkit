BACKEND?=docker
CONCURRENCY?=1


export LUET?=$(shell which luet)
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
COMPRESSION?=zstd
ISO_SPEC?=$(ROOT_DIR)/iso/cOS.yaml
CLEAN?=false
export TREE?=$(ROOT_DIR)/packages

BUILD_ARGS?=--pull --no-spinner --only-target-package --live-output

VALIDATE_OPTIONS?=-s
FLAVOR?=opensuse
DESTINATION?=$(ROOT_DIR)/build
REPO_CACHE?=raccos/$(FLAVOR)
FINAL_REPO?=raccos/releases-$(FLAVOR)

PACKAGES?=$(shell yq r -j $(ISO_SPEC) 'packages.[*]' | jq -r '.[]' | sort -u)
HAS_LUET := $(shell command -v luet 2> /dev/null)
QEMU?=qemu-kvm
QEMU_ARGS?=-bios /usr/share/qemu/ovmf-x86_64.bin
QEMU_MEMORY?=2048
PACKER_ARGS?=
PUBLISH_ARGS?=
GINKGO_ARGS?=-progress -v --failFast -flakeAttempts 3 -r
ISO?=$(ROOT_DIR)/$(shell ls *.iso)

export REPO_CACHE
ifneq ($(strip $(REPO_CACHE)),)
	BUILD_ARGS+=--image-repository $(REPO_CACHE)
endif

all: deps build

deps:
ifndef HAS_LUET
ifneq ($(shell id -u), 0)
	@echo "You must be root to perform this action."
	exit 1
endif
	curl https://get.mocaccino.org/luet/get_luet_root.sh |  sh
	luet install -y repository/mocaccino-extra-stable
	luet install -y utils/jq utils/yq system/luet-devkit
endif

clean:
	 rm -rf $(DESTINATION) $(ROOT_DIR)/.qemu $(ROOT_DIR)/*.iso $(ROOT_DIR)/*.sha256

.PHONY: build
build:
	$(LUET) build $(BUILD_ARGS) \
	--values $(ROOT_DIR)/values/$(FLAVOR).yaml \
	--tree=$(TREE) $(PACKAGES) \
	--backend $(BACKEND) \
	--concurrency $(CONCURRENCY) \
	--compression $(COMPRESSION) \
	--destination $(DESTINATION)

create-repo:
	$(LUET) create-repo --tree "$(TREE)" \
    --output $(DESTINATION) \
    --packages $(DESTINATION) \
    --name "cOS" \
    --descr "cOS $(FLAVOR)" \
    --urls "" \
    --tree-compression $(COMPRESSION) \
    --tree-filename tree.tar \
    --meta-compression $(COMPRESSION) \
    --type http

publish-repo:
	$(LUET) create-repo $(PUBLISH_ARGS) --tree "$(TREE)" \
    --output $(FINAL_REPO) \
    --packages $(DESTINATION) \
    --name "cOS" \
    --descr "cOS $(FLAVOR)" \
    --urls "" \
    --tree-compression $(COMPRESSION) \
    --tree-filename tree.tar \
    --meta-compression $(COMPRESSION) \
    --push-images \
    --type docker

serve-repo:
	LUET_NOLOCK=true $(LUET) serve-repo --port 8000 --dir $(DESTINATION)

autobump:
	TREE_DIR=$(ROOT_DIR) $(LUET) autobump-github

validate:
	$(LUET) tree validate --tree $(TREE) $(VALIDATE_OPTIONS)

# ISO

$(DESTINATION):
	mkdir $(DESTINATION)

$(DESTINATION)/conf.yaml: $(DESTINATION)
	touch $(ROOT_DIR)/build/conf.yaml
	yq w -i $(ROOT_DIR)/build/conf.yaml 'repositories[0].name' 'cOS'
	yq w -i $(ROOT_DIR)/build/conf.yaml 'repositories[0].enable' true

local-iso: create-repo $(DESTINATION)/conf.yaml
	yq w -i $(DESTINATION)/conf.yaml 'repositories[0].urls[0]' $(DESTINATION)
	yq w -i $(DESTINATION)/conf.yaml 'repositories[0].type' 'disk'
	$(LUET) geniso-isospec $(ISO_SPEC)

iso: $(DESTINATION)/conf.yaml
	yq w -i $(DESTINATION)/conf.yaml 'repositories[0].type' 'docker'
	yq w -i $(DESTINATION)/conf.yaml 'repositories[0].urls[0]' $(FINAL_REPO)
	$(LUET) geniso-isospec $(ISO_SPEC)

# QEMU

$(ROOT_DIR)/.qemu:
	mkdir -p $(ROOT_DIR)/.qemu

$(ROOT_DIR)/.qemu/drive.img: $(ROOT_DIR)/.qemu
	qemu-img create -f qcow2 $(ROOT_DIR)/.qemu/drive.img 16g

run-qemu: $(ROOT_DIR)/.qemu/drive.img
	$(QEMU) \
	-m $(QEMU_MEMORY) \
	-cdrom $(ISO) \
	-nographic \
	-serial mon:stdio \
	-rtc base=utc,clock=rt \
	-chardev socket,path=$(ROOT_DIR)/.qemu/qga.sock,server,nowait,id=qga0 \
	-device virtio-serial \
	-hda $(ROOT_DIR)/.qemu/drive.img $(QEMU_ARGS)

# Packer

.PHONY: packer
packer:
	cd $(ROOT_DIR)/packer && packer build -var "iso=$(ISO)" $(PACKER_ARGS) images.json

# Tests

prepare-test:
	vagrant box add cos packer/*.box
	cd $(ROOT_DIR)/tests && vagrant up || true

tests/Vagrantfile:
	cd $(ROOT_DIR)/tests && vagrant init cos

test-clean:
	cd $(ROOT_DIR)/tests && vagrant destroy || true
	vagrant box remove cos || true

test-fallback:
	cd $(ROOT_DIR)/tests && ginkgo $(GINKGO_ARGS) ./fallback

test-features:
	cd $(ROOT_DIR)/tests && ginkgo $(GINKGO_ARGS) ./features

test-upgrades-images-signed:
	cd $(ROOT_DIR)/tests && ginkgo $(GINKGO_ARGS) ./upgrades-images-signed

test-upgrades-images-unsigned:
	cd $(ROOT_DIR)/tests && ginkgo $(GINKGO_ARGS) ./upgrades-images-unsigned

test-smoke:
	cd $(ROOT_DIR)/tests && ginkgo $(GINKGO_ARGS) ./smoke

test-recovery:
	cd $(ROOT_DIR)/tests && ginkgo $(GINKGO_ARGS) ./recovery

test: test-clean tests/Vagrantfile prepare-test test-smoke test-upgrades-images-signed test-upgrades-images-unsigned test-features test-fallback test-recovery
