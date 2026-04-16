MODULE      = $(shell go list -m)
BINARY_NAME = go-root-ceremony
BUILD_DIR   = bin
VERSION    ?= $(shell git describe --tags --always --dirty --match=v* 2>/dev/null || echo "dev")
BUILD_TIME  = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell grep -E '^go [0-9]+\.[0-9]+' go.mod | sed 's/go //')
GOBIN      ?= $$(go env GOPATH)/bin

LDFLAGS = -ldflags "-X main.Version=$(VERSION) -s -w"

.PHONY: all build build-all install test lint fmt vet clean \
        generate example run help check-go-version

## ── Default ────────────────────────────────────────────────────────────────

all: check-go-version fmt vet test build ## Run all checks and build (CI pipeline)

default: build

## ── Build ──────────────────────────────────────────────────────────────────

build: ## Build the binary to bin/
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME) ($(VERSION))"

build-all: ## Build for Linux, macOS (amd64 + arm64) and Windows
	@mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64   .
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64   .
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64  .
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64  .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Built all targets in $(BUILD_DIR)/"

install: build ## Install binary to GOPATH/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOBIN)/$(BINARY_NAME)
	@echo "Installed to $(GOBIN)/$(BINARY_NAME)"

## ── Development ────────────────────────────────────────────────────────────

run: build ## Build and run interactively
	./$(BUILD_DIR)/$(BINARY_NAME) generate -interactive -output ceremony-script.html

generate: build ## Generate a script from example.yaml (writes example-script.html)
	@[ -f example.yaml ] || $(MAKE) example-config
	./$(BUILD_DIR)/$(BINARY_NAME) generate -config example.yaml -output example-script.html
	@echo "Script written to example-script.html — open in browser or print"

example-config: build ## Write example.yaml config file
	./$(BUILD_DIR)/$(BINARY_NAME) init -output example.yaml

## ── Quality ────────────────────────────────────────────────────────────────

test: ## Run tests with race detection
	go test -v -race ./...

test-coverage: ## Run tests and show coverage report
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

fmt: ## Run gofmt and goimports
	gofmt -w .
	@which goimports > /dev/null && goimports -w . || true

vet: ## Run go vet
	go vet ./...

check-go-version: ## Verify installed Go version matches go.mod
	@go version | grep -q "go$(GO_VERSION)" || \
		(echo "Error: Go $(GO_VERSION) required, got $$(go version | awk '{print $$3}')"; exit 1)
	@echo "Go $(GO_VERSION) ✓"

## ── Dependency management ──────────────────────────────────────────────────

tidy: ## Tidy and verify go.mod / go.sum
	go mod tidy
	go mod verify

## ── Cleanup ────────────────────────────────────────────────────────────────

clean: ## Remove build artifacts and generated scripts
	rm -rf $(BUILD_DIR)/
	rm -f coverage.out coverage.html
	rm -f example-script.html
	@echo "Cleaned"

## ── Help ───────────────────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'
