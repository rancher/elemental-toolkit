export HUGO_VERSION?=0.85.0
export HUGO_PLATFORM?=Linux-64bit

export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
export BASE_URL?=https://rancher-sandbox.github.io/cOS-toolkit/

.DEFAULT_GOAL := build

.PHONY: build
build:
	scripts/build.sh

.PHONY: serve
serve:
	scripts/serve.sh

generate-docs:
	scripts/generate_packages_docs.sh $(ROOT_DIR)/../packages
