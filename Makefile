#
# Directory of Makefile
#
export ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

build:
	docker build toolkit -t local/elemental-toolkit

# TODO requires local/elemental-cli to be built, we should find
# a way to build it here, I guess elemetal-toolkit and elemental-cli repos
# should be merged.
build-green: build
	docker build examples/green -t local/elemental-green

# TODO requires local/elemental-cli to be built, we should find
# a way to build it here, I guess elemetal-toolkit and elemental-cli repos
# should be merged.
build-green-iso: build-green
	docker run --rm -ti -v /var/run/docker.sock:/var/run/docker.sock -v $(ROOT_DIR)/build:/build local/elemental-cli --debug build-iso --bootloader-in-rootfs -n elemental-green --date --local --squash-no-compression -o /build local/elemental-green 
