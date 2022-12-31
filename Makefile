BIN := gore
VERSION := $$(make -s show-version)
VERSION_PATH := .
CURRENT_REVISION = $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS = "-s -w -X github.com/x-motemen/$(BIN)/cli.revision=$(CURRENT_REVISION)"
GOBIN ?= $(shell go env GOPATH)/bin

.PHONY: all
all: build

.PHONY: build
build:
	go build -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

.PHONY: install
install:
	go install -ldflags=$(BUILD_LDFLAGS) ./cmd/$(BIN)

.PHONY: show-version
show-version: $(GOBIN)/gobump
	@gobump show -r "$(VERSION_PATH)"

$(GOBIN)/gobump:
	@go install github.com/x-motemen/gobump/cmd/gobump@latest

.PHONY: test
test: build
	go test -v ./... # we don't use -race which increases much duration

.PHONY: lint
lint: $(GOBIN)/staticcheck
	go vet ./...
	staticcheck -checks all,-ST1000 ./...

$(GOBIN)/staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: clean
clean:
	rm -f $(BIN)
	go clean

.PHONY: bump
bump: $(GOBIN)/gobump
	test -z "$$(git status --porcelain || echo .)"
	test "$$(git branch --show-current)" = "main"
	@gobump up -w "$(VERSION_PATH)"
	git commit -am "bump up version to $(VERSION)"
	git tag "v$(VERSION)"
	git push --atomic origin main tag "v$(VERSION)"
