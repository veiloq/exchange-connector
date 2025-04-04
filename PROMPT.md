# Cryptocurrency Exchange Connector Library Development

## Overview
Create a robust, production-ready Go library for connecting to cryptocurrency exchanges, based on the Bybit connector example. The library should follow best practices for Go development, implement proper error handling with retries, and be designed for easy maintenance and extension to other exchanges.

## Core Requirements

### 1. Library Structure
- Implement a clean, modular architecture with clear separation of concerns
- Create well-defined interfaces for exchange connectors
- Support both REST API and WebSocket connections
- Ensure thread-safety for concurrent operations
- Include comprehensive documentation with examples

### 2. Features
- Historical data retrieval (candles, trades, orderbook)
- Real-time data streaming via WebSockets
- Automatic reconnection and recovery logic
- Rate limiting compliance
- Comprehensive error handling with appropriate retries

### 3. Implementation Details
- Follow Go best practices and idiomatic patterns
- Use modern error handling with wrapping: `fmt.Errorf("%w, %w", err1, err2)`
- Implement retry mechanisms using github.com/avast/retry-go
- Ensure proper context propagation throughout the codebase
- Include comprehensive logging for debugging and monitoring

### 4. Testing
- Write comprehensive unit tests with high coverage
- Implement integration tests for API endpoints
- Create mock implementations for testing without actual API calls
- Use test fixtures for consistent test scenarios
- Implement e2e tests using `make e2e-test` for eventual consistency verification

## Publication Standards

### 1. Package Organization
- Follow standard Go project layout
- Organize by feature, not by type
- Keep public API surface minimal and well-documented
- Use internal packages for implementation details

### 2. Documentation
- Write godoc-compliant documentation for all exported types and functions
- Include usage examples in documentation
- Create a comprehensive README with installation and usage instructions
- Document common error scenarios and how to handle them

### 3. Version Management
- Follow semantic versioning (SemVer)
- Maintain a detailed CHANGELOG.md
- Use proper Go module versioning
- Document breaking changes clearly

## Gitflow Integration

### 1. Branch Structure
- main: stable releases only
- develop: integration branch for features
- feature/*: new features and enhancements
- bugfix/*: bug fixes
- release/*: release candidates
- hotfix/*: urgent production fixes

### 2. Development Workflow
1. Create feature branch from develop
2. Implement and test the feature
3. Submit PR to develop branch
4. Review, approve, and merge
5. Create release branch when ready
6. Test on release branch
7. Merge to main and tag with version
8. Merge main back to develop

### 3. CI/CD Pipeline
- Run tests on every PR
- Perform code quality checks (linting, static analysis)
- Generate and publish documentation
- Create release artifacts
- Automate version bumping
- Use GitHub Actions for automation

## Implementation Example

Based on the provided Bybit connector code, extend and refactor to:

1. Support multiple exchanges through a common interface
2. Improve error handling with proper retry mechanisms
3. Enhance the WebSocket reconnection logic
4. Add comprehensive logging
5. Implement rate limiting
6. Create a more robust test suite

## Delivery Expectations

- Clean, well-documented code
- Comprehensive test suite
- Example applications demonstrating usage
- CI/CD pipeline configuration
- Complete documentation including:
  - API reference
  - Usage guides
  - Troubleshooting information
- GitHub repository with proper branch structure

## Implementation Process

1. Start by refactoring the existing Bybit connector into the new architecture
2. Add comprehensive testing
3. Implement proper error handling with retries
4. Enhance WebSocket handling
5. Document the API and usage examples
6. Set up CI/CD pipeline
7. Create release process documentation

The final deliverable should be a production-ready library that can be easily maintained, extended, and integrated into other projects. 