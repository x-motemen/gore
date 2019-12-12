BIN := gore
CURRENT_REVISION := $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS := "-s -w -X github.com/motemen/$(BIN)/cli.revision=$(CURRENT_REVISION)"
GOBIN ?= $(shell go env GOPATH)/bin
export GO111MODULE=on

.PHONY: all
all: clean build

.PHONY: build
build:
	go build -ldflags=$(BUILD_LDFLAGS) -o build/$(BIN) ./cmd/...

.PHONY: install
install:
	go install -ldflags=$(BUILD_LDFLAGS) ./cmd/...

.PHONY: test
test: build
	go test -v ./...

.PHONY: lint
lint: $(GOBIN)/golint
	go vet ./...
	golint -set_exit_status ./...

$(GOBIN)/golint:
	cd && go get golang.org/x/lint/golint

.PHONY: clean
clean:
	rm -rf build
	go clean
