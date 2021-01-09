BIN := gore
VERSION := $$(make -s show-version)
CURRENT_REVISION := $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS := "-s -w -X github.com/motemen/$(BIN)/cli.revision=$(CURRENT_REVISION)"
GOBIN ?= $(shell go env GOPATH)/bin
export GO111MODULE=on

.PHONY: all
all: build

.PHONY: build
build:
	go build -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/...

.PHONY: install
install:
	go install -ldflags=$(BUILD_LDFLAGS) ./cmd/...

.PHONY: show-version
show-version: $(GOBIN)/gobump
	@gobump show -r $(VERSION_PATH)

$(GOBIN)/gobump:
	@cd && go get github.com/x-motemen/gobump/cmd/gobump

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
	rm -f $(BIN)
	go clean

.PHONY: bump
bump: $(GOBIN)/gobump
ifneq ($(shell git status --porcelain),)
	$(error git workspace is dirty)
endif
ifneq ($(shell git rev-parse --abbrev-ref HEAD),master)
	$(error current branch is not master)
endif
	@gobump up -w
	git checkout -b "bump-version-$(VERSION)"
	git commit -am "bump up version to $(VERSION)"
	git push origin "bump-version-$(VERSION)"
