BIN := gore
CURRENT_REVISION := $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS := "-s -w -X github.com/motemen/gore.revision=$(CURRENT_REVISION)"

.PHONY: all
all: clean build

.PHONY: build
build: deps
	go build -ldflags=$(BUILD_LDFLAGS) -o build/$(BIN) ./cmd/...

.PHONY: install
install: deps
	go install -ldflags=$(BUILD_LDFLAGS) ./cmd/...

.PHONY: deps
deps:
	go get -d -v ./...

.PHONY: test
test: build testdeps
	go test -v ./...

.PHONY: testdeps
testdeps:
	go get -d -v -t ./...

.PHONY: lint
lint: lintdeps build
	golint -set_exit_status ./...

.PHONY: lintdeps
lintdeps:
	command -v golint >/dev/null || go get -u golang.org/x/lint/golint

.PHONY: clean
clean:
	rm -rf build
	go clean
