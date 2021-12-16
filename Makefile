#
# cOS-toolkit Makefile
#
#

#----------------------- global variables -----------------------
#
# Path to luet binary
#
export LUET?=$(shell which luet 2> /dev/null)
ifeq ("$(LUET)","")
LUET="/usr/bin/luet"
endif

#
# Path to jq binary
#
export JQ?=$(shell which jq 2> /dev/null)
ifeq ("$(JQ)","")
JQ="/usr/bin/jq"
endif

#
# Path to yq binary
#
export YQ?=$(shell which yq 2> /dev/null)
ifeq ("$(YQ)","")
YQ="/usr/bin/yq"
endif

#
# Path to luet-makeiso binary
#
export MAKEISO?=$(shell which luet-makeiso 2> /dev/null)
ifeq ("$(MAKEISO)","")
MAKEISO="/usr/bin/luet-makeiso"
endif

#
# Path to luet-mtree binary
#
export MTREE?=$(shell which luet-mtree 2> /dev/null)
ifeq ("$(MTREE)","")
MTREE="/usr/bin/luet-mtree"
endif

#
# Path to luet-cosign binary
#
export COSIGN?=$(shell which luet-cosign 2> /dev/null)
ifeq ("$(COSIGN)","")
COSIGN="/usr/bin/luet-cosign"
endif



#
# Location of package tree
#
TREE?=$(ROOT_DIR)/packages

#
# Directory of Makefile
#
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

#
# OS flavor to build
#
FLAVOR?=green

#
# Arch to build for
#

ARCH?=x86_64

#
# Output for "make publish-repo" and base for "make iso"
#
ifneq ($(strip $(ARCH)), x86_64)
	FINAL_REPO?=quay.io/costoolkit/releases-$(FLAVOR)-$(ARCH)
else
	FINAL_REPO?=quay.io/costoolkit/releases-$(FLAVOR)
endif

#
# folder for build artefacts
#
DESTINATION?=$(ROOT_DIR)/build

#
# yaml specification of build targets
#
export MANIFEST?=$(ROOT_DIR)/manifest.yaml

#
# cos config environment file
#
export COS_CONFIG?=$(ROOT_DIR)/packages/cos-config/cos-config

#
# Packer target to build
#
PACKER_TARGET?=virtualbox-iso.cos

#
# Used by .iso, .test, and .run
#
ISO?=$(shell ls $(ROOT_DIR)/*.iso 2> /dev/null)


#----------------------- end global variables -----------------------


#----------------------- default target -----------------------

all: deps build

#----------------------- includes -----------------------

include make/Makefile.build
include make/Makefile.iso
include make/Makefile.run
include make/Makefile.test
include make/Makefile.images

#----------------------- targets -----------------------

deps: $(LUET) $(YQ) $(JQ) $(MAKEISO) $(MTREE) $(COSIGN)

as_root:
ifneq ($(shell id -u), 0)
	@echo "Please run 'make $@' as root"
	@exit 1
endif


$(LUET):
ifneq ($(shell id -u), 0)
	@echo "'$@' is missing and you must be root to install it."
	@exit 1
else
	$(ROOT_DIR)/scripts/get_luet.sh
endif

$(YQ):
ifneq ($(shell id -u), 0)
	@echo "'$@' is missing and you must be root to install it."
	@exit 1
else
	$(LUET) install -y --relax toolchain/yq
endif

$(JQ):
ifneq ($(shell id -u), 0)
	@echo "'$@' is missing and you must be root to install it."
	@exit 1
else
	$(LUET) install -y --relax utils/jq
endif

$(MAKEISO):
ifneq ($(shell id -u), 0)
	@echo "'$@' is missing and you must be root to install it."
	@exit 1
else
	$(LUET) install -y --relax toolchain/luet-makeiso
endif

$(MTREE):
ifneq ($(shell id -u), 0)
	@echo "'$@' is missing and you must be root to install it."
	@exit 1
else
	$(LUET) install -y --relax toolchain/luet-mtree
endif

$(COSIGN):
ifneq ($(shell id -u), 0)
	@echo "'$@' is missing and you must be root to install it."
	@exit 1
else
	$(LUET) install -y --relax toolchain/luet-cosign  toolchain/cosign@1.3.1
endif

clean: clean_build clean_iso clean_run clean_test
	rm -rf $(ROOT_DIR)/*.sha256
