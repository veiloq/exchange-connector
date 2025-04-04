.PHONY: all build test test-race test-cover lint fmt clean e2e-test build-all build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=exchange-connector

# Build parameters
BUILD_DIR=build
MAIN_PACKAGE=./cmd/examples
VERSION=$(shell cat VERSION 2>/dev/null || echo "v0.1.0")

all: deps test build

build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

# Cross-platform builds
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

build-linux-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

build-linux-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)

build-darwin-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)

build-darwin-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

test:
	$(GOTEST) -v ./...

test-race:
	$(GOTEST) -v -race ./...

test-cover:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

e2e-test:
	$(GOTEST) -v -tags=e2e ./test/e2e/...

lint:
	golangci-lint run ./...

fmt:
	$(GOCMD) fmt ./...

deps:
	$(GOMOD) download
	$(GOMOD) tidy

clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install development tools
.PHONY: install-tools
install-tools:
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Generate mocks for testing
.PHONY: generate-mocks
generate-mocks:
	mockgen -source=pkg/exchanges/interfaces/connector.go -destination=test/mocks/mock_connector.go -package=mocks
	mockgen -source=pkg/websocket/connector.go -destination=test/mocks/mock_websocket.go -package=mocks
	mockgen -source=pkg/common/http.go -destination=test/mocks/mock_http.go -package=mocks

# Run example
.PHONY: run-example
run-example:
	$(GOBUILD) -o $(BUILD_DIR)/example $(MAIN_PACKAGE)
	$(BUILD_DIR)/example

# Create a new release 
.PHONY: release
release: build-all
	@echo "Creating release $(VERSION)"
	@if [ -z "$(shell which gh)" ]; then \
		echo "GitHub CLI not found. Please install it first."; \
		exit 1; \
	fi
	@gh release create $(VERSION) \
		--title "Exchange Connector $(VERSION)" \
		--notes "Release $(VERSION)" \
		$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 \
		$(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 \
		$(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 \
		$(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64
