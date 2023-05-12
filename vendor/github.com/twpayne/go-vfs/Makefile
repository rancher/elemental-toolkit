GOLANGCI_LINT_VERSION=1.34.1

.PHONY: smoketest
smoketest: test lint

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint: ensure-golangci-lint
	./bin/golangci-lint run

.PHONY: format
format: ensure-gofumports
	find . -name \*.go | xargs ./bin/gofumports -local github.com/twpayne/chezmoi -w

.PHONY: ensure-tools
ensure-tools: \
	ensure-gofumports \
	ensure-golangci-lint

.PHONY: ensure-gofumports
ensure-gofumports:
	if [ ! -x bin/gofumports ] ; then \
		mkdir -p bin ; \
		( cd $$(mktemp -d) && go mod init tmp && GOBIN=${PWD}/bin go get mvdan.cc/gofumpt/gofumports ) ; \
	fi

.PHONY: ensure-golangci-lint
ensure-golangci-lint:
	if [ ! -x bin/golangci-lint ] || ( ./bin/golangci-lint --version | grep -Fqv "version ${GOLANGCI_LINT_VERSION}" ) ; then \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- v${GOLANGCI_LINT_VERSION} ; \
	fi