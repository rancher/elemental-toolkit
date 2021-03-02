BACKEND?=docker
CONCURRENCY?=1


export LUET?=$(shell which luet)
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
DESTINATION?=$(ROOT_DIR)/build
COMPRESSION?=zstd
ISO_SPEC?=$(ROOT_DIR)/iso/cOS-local.yaml
CLEAN?=false
export TREE?=$(ROOT_DIR)/packages

BUILD_ARGS?=--pull --no-spinner --only-target-package

VALIDATE_OPTIONS?=-s
ARCH?=amd64
REPO_CACHE?=raccos/$(ARCH)
FINAL_REPO?=raccos/releases-$(ARCH)

PACKAGES?=$(shell yq r -j $(ISO_SPEC) 'packages.[*]' | jq -r '.[]' | sort -u)
HAS_LUET := $(shell command -v luet 2> /dev/null)
QEMU?=qemu-kvm
QEMU_ARGS?=-bios /usr/share/qemu/ovmf-x86_64.bin
QEMU_MEMORY?=2048

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
	 rm -rf $(ROOT_DIR)/build/ $(ROOT_DIR)/.qemu

.PHONY: build
build:
	$(LUET) build $(BUILD_ARGS) \
	--values $(ROOT_DIR)/values/$(ARCH).yaml \
	--tree=$(TREE) $(PACKAGES) \
	--backend $(BACKEND) \
	--concurrency $(CONCURRENCY) \
	--compression $(COMPRESSION)

create-repo:
	$(LUET) create-repo --tree "$(TREE)" \
    --output $(DESTINATION) \
    --packages $(DESTINATION) \
    --name "cOS" \
    --descr "cOS $(ARCH)" \
    --urls "" \
    --tree-compression $(COMPRESSION) \
    --tree-filename tree.tar \
    --meta-compression $(COMPRESSION) \
    --type http

publish-repo:
	$(LUET) create-repo --tree "$(TREE)" \
    --output $(FINAL_REPO) \
    --packages $(DESTINATION) \
    --name "cOS" \
    --descr "cOS $(ARCH)" \
    --urls "" \
    --tree-compression $(COMPRESSION) \
    --tree-filename tree.tar \
    --meta-compression $(COMPRESSION) \
    --push-images \
    --type docker

serve-repo:
	LUET_NOLOCK=true $(LUET) serve-repo --port 8000 --dir $(ROOT_DIR)/build

autobump:
	TREE_DIR=$(ROOT_DIR) $(LUET) autobump-github

validate:
	$(LUET) tree validate --tree $(TREE) $(VALIDATE_OPTIONS)

$(ROOT_DIR)/build/conf.yaml:
	touch $(ROOT_DIR)/build/conf.yaml
	yq w -i $(ROOT_DIR)/build/conf.yaml 'repositories[0].name' 'local'
	yq w -i $(ROOT_DIR)/build/conf.yaml 'repositories[0].type' 'disk'
	yq w -i $(ROOT_DIR)/build/conf.yaml 'repositories[0].enable' true
	yq w -i $(ROOT_DIR)/build/conf.yaml 'repositories[0].urls[0]' $(DESTINATION)

local-iso: create-repo $(ROOT_DIR)/build/conf.yaml
	 $(LUET) geniso-isospec $(ISO_SPEC)

$(ROOT_DIR)/.qemu:
	mkdir -p $(ROOT_DIR)/.qemu

$(ROOT_DIR)/.qemu/drive.img: $(ROOT_DIR)/.qemu
	qemu-img create -f qcow2 $(ROOT_DIR)/.qemu/drive.img 16g

run-qemu: .qemu/drive.img
	$(QEMU) \
	-m $(QEMU_MEMORY) \
	-cdrom $(ISO) \
	-nographic \
	-serial mon:stdio \
	-rtc base=utc,clock=rt \
	-chardev socket,path=$(ROOT_DIR)/.qemu/qga.sock,server,nowait,id=qga0 \
	-device virtio-serial \
	-hda $(ROOT_DIR)/.qemu/drive.img $(QEMU_ARGS)