BIN := gore

.PHONY: all
all: clean build

.PHONY: build
build: deps
	go build -o build/$(BIN) .

.PHONY: install
install: deps
	go install .

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

.PHONY: testdeps
lintdeps:
	command -v golint >/dev/null || go get -u golang.org/x/lint/golint

.PHONY: clean
clean:
	rm -rf build
	go clean
