SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.DEFAULT_GOAL := build

################################################################################
# Configurable variables
-include .env

GOOS        ?= ""
GOARCH      ?= ""
CGO_ENABLED ?= 0

################################################################################
# Functions
define INFO
	@echo -e "\e[32m----- $(1)\e[0m"
endef

################################################################################
# Linters (and checks)
tools/golangci-lint:
	@$(call INFO,"Installing tool $(shell basename $@)")
	GOBIN=$$(realpath $$(dirname $@)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0

lint: tools/golangci-lint
	@$(call INFO,"Linting Go")
	./tools/golangci-lint run -v

################################################################################
# Builds
build: bin/fleeting-plugin-proxmox
.PHONY: build

bin/fleeting-plugin-proxmox: $(shell find cmd -name *.go)
	@$(call INFO,"Building $@")
	@mkdir -p $(shell dirname $@)
	GOOS="${GOOS}" GOARCH="${GOARCH}" CGO_ENABLED="${CGO_ENABLED}" go build -ldflags "-w -extldflags '-static'" -o $@ ./cmd/fleeting-plugin-proxmox

################################################################################
# Tests
test: unit-test integration-test
.PHONY: test

unit-test:
	@$(call INFO,"Running unit tests")
	CGO_ENABLED=1 go test -v -race ./cmd/...
.PHONY: unit-test

integration-test: bin/fleeting-plugin-proxmox
	@$(call INFO,"Running integration tests")
	go test -v $(shell go list ./test/integration) \
		-timeout 30m \
		-plugin-binary-path="$(PWD)/bin/fleeting-plugin-proxmox" \
		-config-path="$(PWD)/config.json"
.PHONY: integration-test

################################################################################
# Cleanup
clean:
	@$(call INFO,"Cleaning")
	rm -rfv bin tools
.PHONY: clean
