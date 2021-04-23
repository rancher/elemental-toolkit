#
# cOS-toolkit Makefile
#
#

#----------------------- global variables -----------------------
#
# Path to luet binary
#
export LUET?=$(shell which luet)

#
# Backend to use for "luet build"
# Values "docker" or "podman"
#
BACKEND?=docker

#
# Concurrent downloads in luet
#
CONCURRENCY?=1

#
# Directory of Makefile
#
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

HAS_LUET := $(shell command -v luet 2> /dev/null)

#
# Compression scheme for build artefacts
#
COMPRESSION?=zstd

#
# OS flavor to build
#
FLAVOR?=opensuse

#
# folder for build artefacts
#
DESTINATION?=$(ROOT_DIR)/build

#
# yaml specification of build targets
#
export ISO_SPEC?=$(ROOT_DIR)/iso/cOS.yaml

all: deps build


#----------------------- includes -----------------------

include make/Makefile.build
include make/Makefile.iso
include make/Makefile.run
include make/Makefile.test

#----------------------- targets -----------------------

deps: luet

luet:
ifndef HAS_LUET
ifneq ($(shell id -u), 0)
	@echo "'luet' is missing and you must be root to install it."
	exit 1
endif
	curl https://get.mocaccino.org/luet/get_luet_root.sh |  sh
	luet install -y repository/mocaccino-extra-stable
	luet install -y utils/jq utils/yq system/luet-devkit
endif


clean: clean_build clean_iso clean_run clean_test
	rm -rf $(ROOT_DIR)/*.sha256
