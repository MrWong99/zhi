# Copyright 2025 Lukas Schmidt
# SPDX-License-Identifier: MIT

# ──────────────────────────────────────────────────────────────────────────────
# Project metadata
# ──────────────────────────────────────────────────────────────────────────────
MODULE   := github.com/MrWong99/zhi
BIN_NAME := zhi
BIN_DIR  := bin
CMD_DIR  := ./cmd

# Version info (injected via ldflags)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Go tool flags
GOFLAGS  ?=
GOTESTFLAGS ?= -race -count=1
GOPATH ?= $(shell go env GOPATH)

# Proto
PROTO_DIR     := api/proto
PROTO_FILES   := $(shell find $(PROTO_DIR) -name '*.proto' 2>/dev/null)
PROTO_VERSION := 33.5
PROTO_INSTALL_DIR := $(BIN_DIR)/protoc
PROTOC        := $(PROTO_INSTALL_DIR)/bin/protoc

# Detect OS and architecture for protoc download
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Linux)
    PROTOC_OS := linux
endif
ifeq ($(UNAME_S),Darwin)
    PROTOC_OS := osx
endif

ifeq ($(UNAME_M),x86_64)
    PROTOC_ARCH := x86_64
endif
ifeq ($(UNAME_M),aarch64)
    PROTOC_ARCH := aarch_64
endif
ifeq ($(UNAME_M),arm64)
    PROTOC_ARCH := aarch_64
endif

PROTOC_ZIP := protoc-$(PROTO_VERSION)-$(PROTOC_OS)-$(PROTOC_ARCH).zip
PROTOC_URL := https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTO_VERSION)/$(PROTOC_ZIP)

# ──────────────────────────────────────────────────────────────────────────────
# Default target
# ──────────────────────────────────────────────────────────────────────────────
.DEFAULT_GOAL := help

# ──────────────────────────────────────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build the cmd/ binaries
	@mkdir -p $(BIN_DIR)
	@for dir in $(CMD_DIR)/*/; do \
		name=$$(basename "$$dir"); \
		echo "Building cmd: $$name"; \
	    go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$$name ./$$dir; \
	done

.PHONY: build-examples
build-examples: ## Build all Go example plugins
	@mkdir -p $(BIN_DIR)/examples
	@for dir in examples/*/; do \
		name=$$(basename "$$dir"); \
		if ls "$$dir"*.go 1>/dev/null 2>&1; then \
			echo "Building example: $$name"; \
			go build $(GOFLAGS) -o $(BIN_DIR)/examples/$$name ./$$dir; \
		else \
			echo "Skipping non-Go example: $$name (build separately)"; \
		fi; \
	done

.PHONY: build-all
build-all: build build-examples ## Build all binaries: zhi, marketplace, mirror, and examples

.PHONY: install
install: ## Install the cmd/ binaries into GOPATH/bin
	@for dir in $(CMD_DIR)/*/; do \
		name=$$(basename "$$dir"); \
		echo "Installing cmd: $$name"; \
	    go install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./$$dir; \
	done

# ──────────────────────────────────────────────────────────────────────────────
# Test
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all tests
	go test $(GOTESTFLAGS) ./...

.PHONY: test-short
test-short: ## Run tests in short mode (skip long-running tests)
	go test $(GOTESTFLAGS) -short ./...

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	go test $(GOTESTFLAGS) -v ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	@mkdir -p $(BIN_DIR)
	go test $(GOTESTFLAGS) -coverprofile=$(BIN_DIR)/coverage.out ./...
	go tool cover -func=$(BIN_DIR)/coverage.out
	@echo ""
	@echo "HTML report: go tool cover -html=$(BIN_DIR)/coverage.out"

.PHONY: test-cover-html
test-cover-html: test-cover ## Run tests and open HTML coverage report
	go tool cover -html=$(BIN_DIR)/coverage.out -o $(BIN_DIR)/coverage.html
	@echo "Coverage report written to $(BIN_DIR)/coverage.html"

.PHONY: bench
bench: ## Run benchmarks
	go test -bench=. -benchmem ./...

# ──────────────────────────────────────────────────────────────────────────────
# Code generation
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: generate
generate: proto ## Run all code generation (proto + go generate)
	go generate ./...

.PHONY: install-protoc
install-protoc: ## Download and install the required protoc version locally
	@if [ -f "$(PROTOC)" ]; then \
		INSTALLED_VERSION=$$($(PROTOC) --version | grep -oE '[0-9]+\.[0-9]+' | head -1); \
		if [ "$$INSTALLED_VERSION" = "$(PROTO_VERSION)" ]; then \
			echo "protoc $(PROTO_VERSION) already installed in $(PROTO_INSTALL_DIR)"; \
			exit 0; \
		fi; \
	fi; \
	echo "Installing protoc $(PROTO_VERSION)..."; \
	mkdir -p $(PROTO_INSTALL_DIR); \
	curl -fsSL -o /tmp/$(PROTOC_ZIP) $(PROTOC_URL); \
	unzip -q -o /tmp/$(PROTOC_ZIP) -d $(PROTO_INSTALL_DIR); \
	rm /tmp/$(PROTOC_ZIP); \
	chmod +x $(PROTOC); \
	$(PROTOC) --version

.PHONY: proto
proto: install-protoc ## Generate Go code from Protocol Buffer definitions
	$(PROTOC) \
		--proto_path=$(PROTO_DIR) \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		$(PROTO_FILES)

.PHONY: proto-check
proto-check: proto ## Verify generated proto code is up-to-date
	@if [ -n "$$(git diff --name-only -- '*.pb.go')" ]; then \
		echo "error: generated protobuf files are out of date; run 'make tools proto' and commit the changes"; \
		git diff -- '*.pb.go'; \
		$(PROTOC) --version; \
		exit 1; \
	fi

# ──────────────────────────────────────────────────────────────────────────────
# Lint & format
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: fmt
fmt: ## Format Go source files
	gofmt -s -w .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix
	golangci-lint run --fix ./...

.PHONY: modernize
modernize: ## Check for modern Go code
	go fix ./...

.PHONY: modernize-fix
modernize-fix: ## Write modern Go code
	modernize -fix -test ./...

.PHONY: check
check: fmt vet lint test ## Run all checks: format, vet, lint, test

# ──────────────────────────────────────────────────────────────────────────────
# Dependencies & tools
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: deps
deps: ## Download module dependencies
	go mod download

.PHONY: tools
tools: ## Install required development tools
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/goreleaser/goreleaser/v2@latest
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(GOPATH)/bin v2.9.0

# ──────────────────────────────────────────────────────────────────────────────
# Release
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: snapshot
snapshot: ## Build a snapshot release locally (no publish)
	goreleaser release --snapshot --clean

.PHONY: release-dry-run
release-dry-run: ## Run GoReleaser without publishing
	goreleaser release --skip=publish --clean

# ──────────────────────────────────────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────────────────────────────────────

DOCKER_REGISTRY ?= ghcr.io/mrwong99
DOCKER_TAG ?= $(VERSION)

.PHONY: docker-build
docker-build: build ## Build Docker images for zhi-mirror and zhi-marketplace
	docker build -f zhi-mirror.dockerfile -t $(DOCKER_REGISTRY)/zhi-mirror:$(DOCKER_TAG) .
	docker build -f zhi-marketplace.dockerfile -t $(DOCKER_REGISTRY)/zhi-marketplace:$(DOCKER_TAG) .
	docker tag $(DOCKER_REGISTRY)/zhi-mirror:$(DOCKER_TAG) $(DOCKER_REGISTRY)/zhi-mirror:latest
	docker tag $(DOCKER_REGISTRY)/zhi-marketplace:$(DOCKER_TAG) $(DOCKER_REGISTRY)/zhi-marketplace:latest

.PHONY: docker-push
docker-push: ## Push Docker images to ghcr.io/mrwong99
	docker push $(DOCKER_REGISTRY)/zhi-mirror:$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/zhi-marketplace:$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/zhi-mirror:latest
	docker push $(DOCKER_REGISTRY)/zhi-marketplace:latest

# ──────────────────────────────────────────────────────────────────────────────
# Clean
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
	go clean -cache -testcache

.PHONY: clean-protoc
clean-protoc: ## Remove locally installed protoc
	rm -rf $(PROTO_INSTALL_DIR)

# ──────────────────────────────────────────────────────────────────────────────
# Help
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
