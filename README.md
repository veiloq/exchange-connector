# Cryptocurrency Exchange Connector

A robust, production-ready Go library for connecting to cryptocurrency exchanges. The library provides a unified interface for interacting with multiple exchanges, supporting both REST API and WebSocket connections.

## Features

- 🔄 Unified interface for multiple exchanges
- 📊 Real-time market data streaming
- 🔒 Thread-safe operations
- 🔁 Automatic reconnection and recovery
- ⚡ Rate limiting and backoff strategies
- 📝 Comprehensive logging
- ✅ Extensive test coverage
- 🔄 CI/CD with GitHub Actions

## Installation

```bash
go get github.com/veiloq/exchange-connector
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/veiloq/exchange-connector/pkg/exchanges/bybit"
    "github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
)

func main() {
    // Create a new exchange connector
    connector := bybit.NewConnector(nil) // Use default options

    // Connect to the exchange
    ctx := context.Background()
    if err := connector.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer connector.Close()

    // Get historical candles
    candles, err := connector.GetCandles(ctx, interfaces.CandleRequest{
        Symbol:    "BTCUSDT",
        Interval:  "1m",
        StartTime: time.Now().Add(-1 * time.Hour),
        EndTime:   time.Now(),
        Limit:     60,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Subscribe to real-time candle updates
    err = connector.SubscribeCandles(ctx, interfaces.CandleSubscription{
        Symbols:  []string{"BTCUSDT"},
        Interval: "1m",
    }, func(candle interfaces.Candle) {
        log.Printf("Received candle: %+v", candle)
    })
    if err != nil {
        log.Fatal(err)
    }

    // Keep the program running
    select {}
}
```

## Supported Exchanges

- Bybit
- Binance (coming soon)
- More exchanges planned...

## Architecture

The library follows a clean, modular architecture with the following key components:

- **Exchange Connectors**: Implement exchange-specific logic
- **WebSocket Manager**: Handles real-time data streaming
- **Rate Limiter**: Ensures API rate limit compliance
- **HTTP Client**: Manages REST API requests with retries
- **Logger**: Provides structured logging

## Configuration

Each exchange connector can be configured with custom options:

```go
options := &interfaces.ExchangeOptions{
    APIKey:    "your-api-key",
    APISecret: "your-api-secret",
    
    // Connection settings
    HTTPTimeout: 15 * time.Second,
    
    // Rate limiting
    MaxRequestsPerSecond: 10,
    
    // WebSocket settings
    WSReconnectInterval: 5 * time.Second,
    WSHeartbeatInterval: 20 * time.Second,
    
    // Logging
    LogLevel: "info",
}

connector := bybit.NewConnector(options)
```

## Development

### Prerequisites

- Go 1.21 or higher
- Make
- golangci-lint
- GitHub CLI (gh) for release management

### Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/veiloq/exchange-connector.git
   cd exchange-connector
   ```

2. Install dependencies:
   ```bash
   make deps
   ```

3. Install development tools:
   ```bash
   make install-tools
   ```

### CI/CD

This project uses GitHub Actions for Continuous Integration and Deployment. The workflow includes:

- Dependency installation
- Code linting
- Unit tests
- Race condition detection
- End-to-end tests
- Coverage reporting
- Cross-platform builds for Linux and macOS (including Apple Silicon)
- Automatic releases

The workflow is triggered on pushes to the main branch and on pull requests.

### Cross-Platform Builds

Build for all platforms at once:
```bash
make build-all
```

Or build for specific platforms:
```bash
make build-linux-amd64   # Linux (AMD64)
make build-linux-arm64   # Linux (ARM64)
make build-darwin-amd64  # macOS (AMD64)
make build-darwin-arm64  # macOS (ARM64/Apple Silicon)
```

All build artifacts are placed in the `build` directory.

### Releases

To create a new release:

1. Update the version number in the `VERSION` file
2. Run the release command:
   ```bash
   make release
   ```

This will:
- Build binaries for all supported platforms
- Create a new GitHub release with the version from the VERSION file
- Upload all binaries to the release

You can also use the GitHub CLI directly:
```bash
gh release create v0.1.0 --title "Exchange Connector v0.1.0" --notes "Release notes..." build/exchange-connector-*
```

### Testing

Run all tests:
```bash
make test
```

Run tests with race detection:
```bash
make test-race
```

Run end-to-end tests:
```bash
make e2e-test
```

Generate test coverage report:
```bash
make test-cover
```

### Code Quality

Format code:
```bash
make fmt
```

Run linter:
```bash
make lint
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Error Handling

The library uses Go 1.13+ error wrapping for better error context:

```go
if err != nil {
    return fmt.Errorf("failed to fetch market data: %w", err)
}
```

Retry mechanism for transient errors:

```go
err := retry.Do(
    func() error {
        return someOperation()
    },
    retry.Attempts(3),
    retry.Delay(time.Second),
)
```

## Logging

The library provides structured logging with different levels:

```go
logger := logging.NewLogger()
logger.Info("connecting to exchange",
    logging.String("exchange", "bybit"),
    logging.String("url", wsURL),
)
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Gorilla WebSocket](https://github.com/gorilla/websocket) for WebSocket functionality
- [retry-go](https://github.com/avast/retry-go) for retry mechanisms
