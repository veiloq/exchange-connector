# Include .env file if it exists
-include .env

.PHONY: all build test test-race test-cover lint fmt clean e2e-test build-all build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 help install-tools generate-mocks run-example release get-openapi install-openapi-generator

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

# TARGET: all
#
# DESCRIPTION:
#   Main target to run dependencies, tests, and build the binary ✅
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make all
#
# EXPLANATION:
#   This is the default target that gets executed when running make without arguments
all: deps test build

# TARGET: build
#
# DESCRIPTION:
#   Builds the binary for the current platform ✅
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make build
#
# EXPLANATION:
#   Creates the build directory and compiles the main package
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

# TARGET: build-all
#
# DESCRIPTION:
#   Builds binaries for all supported platforms ✅
#
# PREREQUISITES:
#   - Go toolchain with cross-compilation support
#
# USAGE EXAMPLES:
#   - make build-all
#
# EXPLANATION:
#   Runs build targets for Linux (AMD64/ARM64) and macOS (AMD64/ARM64)
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

# TARGET: build-linux-amd64
#
# DESCRIPTION:
#   Builds the binary for Linux AMD64 ✅
#
# PREREQUISITES:
#   - Go toolchain with cross-compilation support
#
# USAGE EXAMPLES:
#   - make build-linux-amd64
build-linux-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

# TARGET: build-linux-arm64
#
# DESCRIPTION:
#   Builds the binary for Linux ARM64 ✅
#
# PREREQUISITES:
#   - Go toolchain with cross-compilation support
#
# USAGE EXAMPLES:
#   - make build-linux-arm64
build-linux-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)

# TARGET: build-darwin-amd64
#
# DESCRIPTION:
#   Builds the binary for macOS AMD64 (Intel) ✅
#
# PREREQUISITES:
#   - Go toolchain with cross-compilation support
#
# USAGE EXAMPLES:
#   - make build-darwin-amd64
build-darwin-amd64:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)

# TARGET: build-darwin-arm64
#
# DESCRIPTION:
#   Builds the binary for macOS ARM64 (Apple Silicon) ✅
#
# PREREQUISITES:
#   - Go toolchain with cross-compilation support
#
# USAGE EXAMPLES:
#   - make build-darwin-arm64
build-darwin-arm64:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

# TARGET: test
#
# DESCRIPTION:
#   Runs all tests in the project ✅
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make test
#
# EXPLANATION:
#   Executes Go tests with verbose output for all packages
test:
	$(GOTEST) -v ./...

# TARGET: test-race
#
# DESCRIPTION:
#   Runs all tests with race condition detection ⚠️
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make test-race
#
# EXPLANATION:
#   Executes Go tests with race detection enabled to find concurrency issues
test-race:
	$(GOTEST) -v -race ./...

# TARGET: test-cover
#
# DESCRIPTION:
#   Runs tests with coverage reporting ✅
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make test-cover
#
# EXPLANATION:
#   Generates coverage profile and HTML report for code coverage visualization
test-cover:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# TARGET: e2e-test
#
# DESCRIPTION:
#   Runs end-to-end tests ✅
#
# PREREQUISITES:
#   - Go toolchain
#   - External services may be required depending on test setup
#
# USAGE EXAMPLES:
#   - make e2e-test
#
# EXPLANATION:
#   Executes end-to-end tests that validate the full system behavior
e2e-test:
	$(GOTEST) -v -tags=e2e ./test/e2e/...

# TARGET: lint
#
# DESCRIPTION:
#   Runs the linter to check code quality ⚠️
#
# PREREQUISITES:
#   - golangci-lint installed (run make install-tools first)
#
# USAGE EXAMPLES:
#   - make lint
#
# EXPLANATION:
#   Uses golangci-lint to perform static code analysis and enforce coding standards
lint:
	golangci-lint run ./...

# TARGET: fmt
#
# DESCRIPTION:
#   Formats Go code according to standard style ✅
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make fmt
fmt:
	$(GOCMD) fmt ./...

# TARGET: deps
#
# DESCRIPTION:
#   Downloads and tidies dependencies ✅
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# TARGET: clean
#
# DESCRIPTION:
#   Removes build artifacts and temporary files ✅
#
# USAGE EXAMPLES:
#   - make clean
#
# EXPLANATION:
#   Cleans build directory, coverage files, and other generated artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# TARGET: install-tools
#
# DESCRIPTION:
#   Installs development tools required for the project ✅
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make install-tools
install-tools:
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# TARGET: install-openapi-generator
#
# DESCRIPTION:
#   Installs OpenAPI Generator CLI ✅
#
# PREREQUISITES:
#   - Java Runtime Environment (JRE)
#   - npm (for the npm installation method)
#
# USAGE EXAMPLES:
#   - make install-openapi-generator
#
# EXPLANATION:
#   Installs OpenAPI Generator CLI. This target uses npm, but you can
#   modify it to use other installation methods if preferred.
install-openapi-generator:
	@echo "Installing OpenAPI Generator CLI..."
	@if command -v npm > /dev/null; then \
		npm install -g @openapitools/openapi-generator-cli; \
	else \
		echo "Error: npm not found. Please install Node.js and npm first."; \
		exit 1; \
	fi
	@echo "✅ OpenAPI Generator installed successfully"

# TARGET: get-openapi
#
# DESCRIPTION:
#   Downloads OpenAPI specifications from official site and places them in docs folder ✅
#
# PREREQUISITES:
#   - curl command
#
# USAGE EXAMPLES:
#   - make get-openapi
#
# EXPLANATION:
#   Creates docs directory if it doesn't exist and downloads the latest OpenAPI specifications
get-openapi:
	@echo "Downloading OpenAPI specifications..."
	@mkdir -p docs
	@curl -s https://api.bybit.com/v5/portal/specs/v5-open-api.json -o docs/openapi.json
	@curl -s https://api.bybit.com/v5/portal/specs/v5-websocket.json -o docs/websocket-api.json
	@echo "✅ OpenAPI specifications downloaded to docs/ directory"

# TARGET: generate-mocks
#
# DESCRIPTION:
#   Generates mock implementations for testing ✅
#
# PREREQUISITES:
#   - mockgen tool (install with go install github.com/golang/mock/mockgen@latest)
#
# USAGE EXAMPLES:
#   - make generate-mocks
generate-mocks:
	mockgen -source=pkg/exchanges/interfaces/connector.go -destination=test/mocks/mock_connector.go -package=mocks
	mockgen -source=pkg/websocket/connector.go -destination=test/mocks/mock_websocket.go -package=mocks
	mockgen -source=pkg/common/http.go -destination=test/mocks/mock_http.go -package=mocks

# TARGET: run-example
#
# DESCRIPTION:
#   Builds and runs the example application ✅
#
# PREREQUISITES:
#   - Go toolchain
#
# USAGE EXAMPLES:
#   - make run-example
run-example:
	$(GOBUILD) -o $(BUILD_DIR)/example $(MAIN_PACKAGE)
	$(BUILD_DIR)/example

# TARGET: release
#
# DESCRIPTION:
#   Creates a new GitHub release with cross-platform binaries ✅
#
# PREREQUISITES:
#   - GitHub CLI (gh) installed
#   - Authenticated GitHub account with repository access
#   - VERSION file containing the release version
#
# USAGE EXAMPLES:
#   - make release
#   - VERSION=v1.2.3 make release
#
# EXPLANATION:
#   Builds all platform binaries and creates a GitHub release with the specified version
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

# TARGET: help
#
# DESCRIPTION:
#   Displays help information about available targets ℹ️
#
# USAGE EXAMPLES:
#   - make help
#   - make
help:
	@echo "Solution: veiloq/exchange-connector"
	@echo ""
	@echo "Description: A robust, production-ready Go library for connecting to cryptocurrency exchanges"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@echo "  all              - Run dependencies, tests, and build the binary"
	@echo "  build            - Build the binary for the current platform"
	@echo "  build-all        - Build binaries for all supported platforms"
	@echo "  build-linux-amd64 - Build binary for Linux AMD64"
	@echo "  build-linux-arm64 - Build binary for Linux ARM64" 
	@echo "  build-darwin-amd64 - Build binary for macOS AMD64 (Intel)"
	@echo "  build-darwin-arm64 - Build binary for macOS ARM64 (Apple Silicon)"
	@echo "  test             - Run all tests in the project"
	@echo "  test-race        - Run tests with race condition detection"
	@echo "  test-cover       - Run tests with coverage reporting"
	@echo "  e2e-test         - Run end-to-end tests"
	@echo "  lint             - Run linter to check code quality"
	@echo "  fmt              - Format Go code according to standard style"
	@echo "  deps             - Download and tidy dependencies"
	@echo "  clean            - Remove build artifacts and temporary files"
	@echo "  install-tools    - Install development tools"
	@echo "  install-openapi-generator - Install OpenAPI Generator CLI"
	@echo "  get-openapi      - Download OpenAPI specifications"
	@echo "  generate-mocks   - Generate mock implementations for testing"
	@echo "  run-example      - Build and run the example application"
	@echo "  release          - Create a new GitHub release with binaries"
	@echo "  help             - Display this help information"
	@echo ""
	@echo "Status Indicators:"
	@echo "  ✅ - Success/OK      - Operation completed successfully"
	@echo "  ❌ - Error/Failure   - Operation failed or critical issue detected"
	@echo "  ⚠️ - Warning         - Potential issue that requires attention"
