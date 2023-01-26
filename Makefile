build:
	docker build toolkit -t local/elemental-toolkit


build-green: build
	docker build examples/green -t local/elemental-green

#build-green-iso: build-green
#	
