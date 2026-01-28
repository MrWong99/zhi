# Copyright 2025 Lukas Schmidt
# SPDX-License-Identifier: MIT

# ──────────────────────────────────────────────────────────────────────────────
# Project metadata
# ──────────────────────────────────────────────────────────────────────────────
MODULE   := github.com/MrWong99/zhi
BIN_NAME := zhi
BIN_DIR  := bin
CMD_DIR  := ./cmd/zhi

# Go tool flags
GOFLAGS  ?=
GOTESTFLAGS ?= -race -count=1

# Proto
PROTO_DIR     := api/proto
PROTO_FILES   := $(shell find $(PROTO_DIR) -name '*.proto' 2>/dev/null)

# ──────────────────────────────────────────────────────────────────────────────
# Default target
# ──────────────────────────────────────────────────────────────────────────────
.DEFAULT_GOAL := help

# ──────────────────────────────────────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build the zhi binary
	@mkdir -p $(BIN_DIR)
	go build $(GOFLAGS) -o $(BIN_DIR)/$(BIN_NAME) $(CMD_DIR)

.PHONY: build-examples
build-examples: ## Build all example plugins
	@mkdir -p $(BIN_DIR)/examples
	@for dir in examples/*/; do \
		name=$$(basename "$$dir"); \
		echo "Building example: $$name"; \
		go build $(GOFLAGS) -o $(BIN_DIR)/examples/$$name ./$$dir; \
	done

.PHONY: build-all
build-all: build build-examples ## Build the main binary and all examples

.PHONY: install
install: ## Install the zhi binary into GOPATH/bin
	go install $(GOFLAGS) $(CMD_DIR)

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

.PHONY: proto
proto: ## Generate Go code from Protocol Buffer definitions
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		$(PROTO_FILES)

.PHONY: proto-check
proto-check: proto ## Verify generated proto code is up-to-date
	@if [ -n "$$(git diff --name-only -- '*.pb.go')" ]; then \
		echo "error: generated protobuf files are out of date; run 'make proto' and commit the changes"; \
		git diff --stat -- '*.pb.go'; \
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
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ──────────────────────────────────────────────────────────────────────────────
# Clean
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
	go clean -cache -testcache

# ──────────────────────────────────────────────────────────────────────────────
# Help
# ──────────────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
