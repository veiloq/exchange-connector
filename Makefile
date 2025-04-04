.PHONY: all build test test-race test-cover lint fmt clean e2e-test

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=crypto-exchange-connector

# Build parameters
BUILD_DIR=build
MAIN_PACKAGE=./cmd/examples

all: deps test build

build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

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
