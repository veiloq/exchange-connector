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
- 🚨 Standardized error handling

## Simple Examples

### Example 1: Basic Exchange Connection and Data Retrieval

The following example shows how to connect to an exchange and retrieve historical candle data:

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"
    "time"

    "github.com/veiloq/exchange-connector/pkg/exchanges/bybit"
    "github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
)

func main() {
    // 1. Create exchange options with authentication credentials
    options := interfaces.NewExchangeOptions()
    options.APIKey = "your-api-key"        // Replace with your API key
    options.APISecret = "your-api-secret"  // Replace with your API secret
    
    // 2. Initialize the exchange-specific connector
    connector := bybit.NewConnector(options)
    
    // 3. Create a cancelable context for connection control
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel() // Ensure resources are properly released
    
    // 4. Connect to the exchange API
    if err := connector.Connect(ctx); err != nil {
        // Handle specific connection errors
        if errors.Is(err, interfaces.ErrInvalidCredentials) {
            log.Fatalf("Authentication failed: please check your API credentials")
        } else if errors.Is(err, interfaces.ErrExchangeUnavailable) {
            log.Fatalf("Exchange is currently unavailable, try again later")
        } else if errors.Is(err, context.DeadlineExceeded) {
            log.Fatalf("Connection timed out after %v", 30*time.Second)
        } else {
            log.Fatalf("Failed to connect to exchange: %v", err)
        }
    }
    defer connector.Close() // Ensure connection is closed when done
    
    // 5. Define the parameters for historical candle data
    request := interfaces.CandleRequest{
        Symbol:    "BTCUSDT",
        Interval:  "1h",
        StartTime: time.Now().Add(-24 * time.Hour), // Last 24 hours
        EndTime:   time.Now(),
        Limit:     24, // One candle per hour
    }
    
    // 6. Retrieve the historical candle data
    candles, err := connector.GetCandles(ctx, request)
    if err != nil {
        // Handle specific data retrieval errors
        if errors.Is(err, interfaces.ErrInvalidSymbol) {
            log.Fatalf("Invalid trading pair symbol: %s", request.Symbol)
        } else if errors.Is(err, interfaces.ErrInvalidInterval) {
            log.Fatalf("Invalid time interval: %s", request.Interval)
        } else if errors.Is(err, interfaces.ErrInvalidTimeRange) {
            log.Fatalf("Invalid time range specified")
        } else if errors.Is(err, interfaces.ErrRateLimitExceeded) {
            log.Fatalf("Rate limit exceeded, try again later")
        } else {
            // Check if it's a market-specific error
            var marketErr *interfaces.MarketError
            if errors.As(err, &marketErr) {
                log.Fatalf("Market error for %s: %s", marketErr.Symbol, marketErr.Message)
            } else {
                log.Fatalf("Failed to get candles: %v", err)
            }
        }
    }
    
    // 7. Process and display the results
    fmt.Printf("Retrieved %d candles for %s\n", len(candles), request.Symbol)
    for _, candle := range candles {
        fmt.Printf("%s | Open: %.2f, High: %.2f, Low: %.2f, Close: %.2f, Volume: %.2f\n",
            candle.StartTime.Format("2006-01-02 15:04:05"),
            candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)
    }
}
```

### Example 2: Real-time WebSocket Data Subscription

This example demonstrates how to subscribe to real-time order book updates via WebSocket:

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/veiloq/exchange-connector/pkg/exchanges/bybit"
    "github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
)

func main() {
    // 1. Initialize the exchange connector with default options
    connector := bybit.NewConnector(nil) // nil uses default options
    
    // 2. Create a long-lived context for the WebSocket connection
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel() // Ensure context is canceled when we're done
    
    // 3. Connect to the exchange
    if err := connector.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect to exchange: %v", err)
    }
    defer connector.Close() // Ensure connection is properly closed
    
    log.Println("Connected successfully to exchange")
    
    // 4. Define the trading pairs to monitor
    symbols := []string{"BTCUSDT"}
    
    // 5. Subscribe to real-time order book updates
    err := connector.SubscribeOrderBook(ctx, symbols, func(book interfaces.OrderBook) {
        // This handler function is called for each order book update
        
        // Extract best bid and ask prices (top of the book)
        if len(book.Bids) == 0 || len(book.Asks) == 0 {
            return // Skip incomplete order books
        }
        
        bestBid := book.Bids[0]
        bestAsk := book.Asks[0]
        spread := bestAsk.Price - bestBid.Price
        
        // Display formatted market data
        fmt.Printf("[%s] %s | Bid: $%.2f (%.6f) | Ask: $%.2f (%.6f) | Spread: $%.2f (%.2f%%)\n",
            time.Now().Format("15:04:05"),
            book.Symbol,
            bestBid.Price, bestBid.Quantity,
            bestAsk.Price, bestAsk.Quantity,
            spread, (spread/bestBid.Price)*100)
    })
    
    if err != nil {
        // Handle subscription-specific errors
        if errors.Is(err, interfaces.ErrNotConnected) {
            log.Fatalf("Not connected to exchange, call Connect() first")
        } else if errors.Is(err, interfaces.ErrInvalidSymbol) {
            log.Fatalf("Invalid trading pair symbol provided")
        } else if errors.Is(err, interfaces.ErrSubscriptionFailed) {
            log.Fatalf("Failed to establish subscription")
        } else {
            log.Fatalf("Failed to subscribe to order book: %v", err)
        }
    }
    
    log.Printf("Subscribed to order book updates for: %s", symbols[0])
    log.Println("Press Ctrl+C to exit")
    
    // 6. Wait for interrupt signal to gracefully shut down
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    
    log.Println("Shutdown signal received, closing connection...")
}
```

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
- **Error Handling**: Standardized error types for consistent handling

## Configuration

Each exchange connector can be configured with custom options:

```go
options := &interfaces.ExchangeOptions{
    APIKey:    "your-api-key",
    APISecret: "your-api-secret",
    
    // Optional custom base URL (useful for testing or proxies)
    BaseURL:   "wss://test.bybit.com/v5/public/spot",
    
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

## Error Handling

The library provides standardized error types for consistent error handling across different exchanges:

### Standard Error Types

```go
// Common error constants
var (
    ErrNotConnected           = errors.New("exchange connector not connected")
    ErrInvalidSymbol          = errors.New("invalid trading pair symbol")
    ErrInvalidInterval        = errors.New("invalid time interval")
    ErrInvalidTimeRange       = errors.New("invalid time range")
    ErrRateLimitExceeded      = errors.New("exchange rate limit exceeded")
    ErrAuthenticationRequired = errors.New("authentication required for this operation")
    ErrInvalidCredentials     = errors.New("invalid API credentials")
    ErrSubscriptionFailed     = errors.New("failed to establish subscription")
    ErrSubscriptionNotFound   = errors.New("subscription not found")
    ErrExchangeUnavailable    = errors.New("exchange API unavailable")
)
```

### Checking for Specific Errors

Use `errors.Is()` to check for specific error types:

```go
if err := connector.Connect(ctx); err != nil {
    if errors.Is(err, interfaces.ErrInvalidCredentials) {
        // Handle authentication error
    } else if errors.Is(err, context.DeadlineExceeded) {
        // Handle timeout
    } else {
        // Handle other errors
    }
}
```

### Market-Specific Errors

For symbol-specific errors, use `errors.As()` to extract detailed information:

```go
var marketErr *interfaces.MarketError
if errors.As(err, &marketErr) {
    log.Printf("Error for symbol %s: %s", marketErr.Symbol, marketErr.Message)
}
```

The library uses Go 1.13+ error wrapping for better error context:

```go
if err != nil {
    return fmt.Errorf("failed to fetch market data: %w", err)
}
```

### Retry mechanism for transient errors:

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

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Gorilla WebSocket](https://github.com/gorilla/websocket) for WebSocket functionality
- [retry-go](https://github.com/avast/retry-go) for retry mechanisms
