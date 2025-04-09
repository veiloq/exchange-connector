# Include .env file if it exists
-include .env

.PHONY: all test test-race test-cover lint fmt clean e2e-test help install-tools generate-mocks get-openapi install-openapi-generator

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
# MAIN_PACKAGE removed - not needed for library
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
all: deps test

# Build targets removed - Replaced by goreleaser in CI/CD pipeline
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
#   DISABLED: Currently disabled due to Go 1.24 compatibility issues with golangci-lint
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
	@echo "Linting is currently disabled due to compatibility issues between Go 1.24 and golangci-lint"
	@echo "To re-enable, update the Makefile when a compatible golangci-lint version is available"

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

# run-example target removed - examples should be run directly with 'go run'

# Release target removed - Handled by goreleaser in CI/CD pipeline

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
	@echo "  all              - Run dependencies and tests"
	@echo "  test             - Run all tests in the project"
	@echo "  test-race        - Run tests with race condition detection"
	@echo "  test-cover       - Run tests with coverage reporting"
	@echo "  lint             - Run linter to check code quality"
	@echo "  fmt              - Format Go code according to standard style"
	@echo "  deps             - Download and tidy dependencies"
	@echo "  clean            - Remove build artifacts and temporary files"
	@echo "  install-tools    - Install development tools"
	@echo "  install-openapi-generator - Install OpenAPI Generator CLI"
	@echo "  get-openapi      - Download OpenAPI specifications"
	@echo "  generate-mocks   - Generate mock implementations for testing"
	@echo "  run-example      - (Removed - run examples directly with 'go run')"
	@echo "  release          - (Removed - Handled by goreleaser in CI/CD)"
	@echo "  help             - Display this help information"
	@echo ""
	@echo "Status Indicators:"
	@echo "  ✅ - Success/OK      - Operation completed successfully"
	@echo "  ❌ - Error/Failure   - Operation failed or critical issue detected"
	@echo "  ⚠️ - Warning         - Potential issue that requires attention"
