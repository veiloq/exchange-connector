# Exchange Connector

A production-ready Go library for connecting to cryptocurrency exchanges with a unified interface for both REST API and WebSocket connections.

## Features

- **Unified API**: Consistent interface for multiple cryptocurrency exchanges
- **Real-time Data**: WebSocket subscriptions for live market data
- **Market Data Operations**: Fetch candles, tickers, and order books
- **Automatic Reconnection**: Built-in connection management
- **Rate Limiting**: Protection against API rate limits
- **Standardized Error Handling**: Consistent error types across implementations
- **Thread Safety**: Safe for concurrent use

## Installation

Requires Go 1.24 or later:

```bash
go get github.com/veiloq/exchange-connector
```

## Quick Example

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
	// Create a connector with default options
	connector := bybit.NewConnector(nil)
	
	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := connector.Connect(ctx); err != nil {
		if errors.Is(err, interfaces.ErrExchangeUnavailable) {
			log.Fatalf("Exchange is currently unavailable, try again later")
		}
		log.Fatalf("Connection failed: %v", err)
	}
	defer connector.Close()
	
	// Get BTC/USDT candles for the last hour
	candles, err := connector.GetCandles(ctx, interfaces.CandleRequest{
		Symbol:    "BTCUSDT",
		Interval:  "1m",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Limit:     60,
	})
	
	if err != nil {
		switch {
		case errors.Is(err, interfaces.ErrInvalidSymbol):
			log.Fatalf("Invalid trading pair symbol: %s", "BTCUSDT")
		case errors.Is(err, interfaces.ErrInvalidInterval):
			log.Fatalf("Invalid time interval: %s", "1m")
		case errors.Is(err, interfaces.ErrInvalidTimeRange):
			log.Fatalf("Invalid time range specified")
		default:
			log.Fatalf("Failed to get candles: %v", err)
		}
	}
	
	fmt.Printf("Retrieved %d candles for BTCUSDT\n", len(candles))
	for i, candle := range candles[:3] {
		fmt.Printf("%d. %s | Open: %.2f, Close: %.2f\n",
			i+1,
			candle.StartTime.Format("15:04:05"),
			candle.Open,
			candle.Close)
	}
}
```

## Real-time WebSocket Example

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
	// Create and connect
	connector := bybit.NewConnector(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	if err := connector.Connect(ctx); err != nil {
		if errors.Is(err, interfaces.ErrExchangeUnavailable) {
			log.Fatalf("Exchange is currently unavailable, try again later")
		}
		log.Fatalf("Connection failed: %v", err)
	}
	defer connector.Close()
	
	// Subscribe to real-time candle updates
	symbols := []string{"BTCUSDT"}
	subscription := interfaces.CandleSubscription{
		Symbols:  symbols,
		Interval: "1m",
	}
	
	err := connector.SubscribeCandles(ctx, subscription, func(candle interfaces.Candle) {
		fmt.Printf("[%s] BTCUSDT | Open: $%.2f | High: $%.2f | Low: $%.2f | Close: $%.2f | Volume: %.2f\n",
			candle.StartTime.Format("15:04:05"),
			candle.Open,
			candle.High,
			candle.Low,
			candle.Close,
			candle.Volume)
	})
	
	if err != nil {
		switch {
		case errors.Is(err, interfaces.ErrInvalidSymbol):
			log.Fatalf("Invalid trading pair symbol provided")
		case errors.Is(err, interfaces.ErrInvalidInterval):
			log.Fatalf("Invalid candle interval specified")
		case errors.Is(err, interfaces.ErrSubscriptionFailed):
			log.Fatalf("Failed to establish subscription")
		default:
			log.Fatalf("Subscription failed: %v", err)
		}
	}
	
	fmt.Printf("Subscribed to %s candle updates for %s\n", subscription.Interval, symbols[0])
	fmt.Println("Press Ctrl+C to exit")
	
	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
```

## Authentication Example

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/veiloq/exchange-connector/pkg/exchanges/bybit"
	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
)

func main() {
	// Create authenticated connector using WithCredentials method
	connector := bybit.NewConnector(
		interfaces.NewExchangeOptions().WithCredentials("your-api-key", "your-api-secret"),
	)
	
	ctx := context.Background()
	if err := connector.Connect(ctx); err != nil {
		switch {
		case errors.Is(err, interfaces.ErrInvalidCredentials):
			log.Fatalf("Authentication failed: check your API credentials")
		case errors.Is(err, interfaces.ErrExchangeUnavailable):
			log.Fatalf("Exchange is currently unavailable, try again later")
		default:
			log.Fatalf("Connection failed: %v", err)
		}
	}
	defer connector.Close()
	
	// Get account balances (requires authentication)
	balances, err := connector.GetBalances(ctx)
	if err != nil {
		switch {
		case errors.Is(err, interfaces.ErrNotConnected):
			log.Fatalf("Not connected to exchange, call Connect() first")
		case errors.Is(err, interfaces.ErrAuthenticationRequired):
			log.Fatalf("Authentication required for this operation")
		case errors.Is(err, interfaces.ErrRateLimitExceeded):
			log.Fatalf("Rate limit exceeded, try again later")
		default:
			log.Fatalf("Failed to get balances: %v", err)
		}
	}
	
	fmt.Println("Account Balances:")
	for asset, balance := range balances {
		fmt.Printf("%s: Available: %.8f, Total: %.8f\n", 
			asset, balance.Available, balance.Total)
	}
}
```

## Candle Analysis Example

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
	connector := bybit.NewConnector(nil)
	ctx := context.Background()
	
	if err := connector.Connect(ctx); err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer connector.Close()
	
	// Get daily candles for the last month
	candles, err := connector.GetCandles(ctx, interfaces.CandleRequest{
		Symbol:    "BTCUSDT",
		Interval:  "1d",  // Daily candles
		StartTime: time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
		EndTime:   time.Now(),
		Limit:     30,
	})
	
	if err != nil {
		log.Fatalf("Failed to get candles: %v", err)
	}
	
	// Calculate simple moving average
	calculateSMA := func(period int) float64 {
		if len(candles) < period {
			return 0
		}
		
		sum := 0.0
		for i := len(candles) - 1; i >= len(candles) - period; i-- {
			sum += candles[i].Close
		}
		return sum / float64(period)
	}
	
	// Calculate and print moving averages
	sma7 := calculateSMA(7)
	sma14 := calculateSMA(14)
	
	fmt.Printf("7-day SMA: %.2f\n", sma7)
	fmt.Printf("14-day SMA: %.2f\n", sma14)
	
	// Determine trend
	if sma7 > sma14 {
		fmt.Println("Current trend: Bullish (Short-term MA above Long-term MA)")
	} else {
		fmt.Println("Current trend: Bearish (Short-term MA below Long-term MA)")
	}
}
```

## Order Book Example

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/veiloq/exchange-connector/pkg/exchanges/bybit"
	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
)

func main() {
	connector := bybit.NewConnector(nil)
	ctx := context.Background()
	
	if err := connector.Connect(ctx); err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer connector.Close()
	
	// Get order book with 10 levels of depth
	orderBook, err := connector.GetOrderBook(ctx, "BTCUSDT", 10)
	if err != nil {
		switch {
		case errors.Is(err, interfaces.ErrInvalidSymbol):
			log.Fatalf("Invalid trading pair symbol provided")
		default:
			log.Fatalf("Failed to get order book: %v", err)
		}
	}
	
	// Print best bid and ask
	fmt.Printf("Best bid: %.2f (%.6f BTC)\n", orderBook.Bids[0].Price, orderBook.Bids[0].Quantity)
	fmt.Printf("Best ask: %.2f (%.6f BTC)\n", orderBook.Asks[0].Price, orderBook.Asks[0].Quantity)
	
	// Calculate bid-ask spread
	spread := orderBook.Asks[0].Price - orderBook.Bids[0].Price
	spreadPercentage := spread / orderBook.Bids[0].Price * 100
	
	fmt.Printf("Bid-ask spread: %.2f (%.4f%%)\n", spread, spreadPercentage)
	
	// Calculate total liquidity within 1% of best bid/ask
	threshold := orderBook.Bids[0].Price * 0.01
	bidLiquidity := 0.0
	askLiquidity := 0.0
	
	for _, bid := range orderBook.Bids {
		if orderBook.Bids[0].Price - bid.Price <= threshold {
			bidLiquidity += bid.Quantity
		}
	}
	
	for _, ask := range orderBook.Asks {
		if ask.Price - orderBook.Asks[0].Price <= threshold {
			askLiquidity += ask.Quantity
		}
	}
	
	fmt.Printf("Bid liquidity within 1%%: %.6f BTC\n", bidLiquidity)
	fmt.Printf("Ask liquidity within 1%%: %.6f BTC\n", askLiquidity)
}
```

## Supported Exchanges

- Bybit
- Binance (coming soon)

## Comprehensive Error Handling

The library provides standardized error types for consistent error handling across different exchange implementations. These errors can be checked using `errors.Is()` to identify specific conditions:

| Error | Description |
|-------|-------------|
| `ErrNotConnected` | Returned when an operation is attempted on a connector that hasn't been connected yet or has lost connection |
| `ErrInvalidSymbol` | Returned when an invalid trading pair symbol is provided |
| `ErrInvalidInterval` | Returned when an unsupported time interval is provided |
| `ErrInvalidTimeRange` | Returned when an invalid time range is provided (e.g., end time before start time) |
| `ErrRateLimitExceeded` | Returned when the exchange rate limit is exceeded |
| `ErrAuthenticationRequired` | Returned when attempting an operation that requires authentication without providing credentials |
| `ErrInvalidCredentials` | Returned when the provided API credentials are invalid |
| `ErrSubscriptionFailed` | Returned when a WebSocket subscription cannot be established |
| `ErrSubscriptionNotFound` | Returned when trying to unsubscribe from a non-existent subscription |
| `ErrExchangeUnavailable` | Returned when the exchange API is unavailable |

Additionally, the library provides `MarketError` type for market-specific error conditions, which can be created using `NewMarketError(symbol, message, err)`.

### Example Error Handling

```go
if err := connector.Connect(ctx); err != nil {
	switch {
	case errors.Is(err, interfaces.ErrInvalidCredentials):
		log.Fatal("Authentication failed: check your API credentials")
	case errors.Is(err, interfaces.ErrRateLimitExceeded):
		log.Fatal("Rate limit exceeded, try again later")
	case errors.Is(err, interfaces.ErrExchangeUnavailable):
		log.Fatal("Exchange is currently unavailable")
	default:
		log.Fatalf("Connection error: %v", err)
	}
}
```

### Error Handling with Retries

Using the `retry-go` package for exponential backoff with certain error types:

```go
import (
	"github.com/avast/retry-go"
	"github.com/veiloq/exchange-connector/pkg/exchanges/bybit"
	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
)

func main() {
	connector := bybit.NewConnector(nil)
	ctx := context.Background()
	
	// Connect with retry for transient errors
	err := retry.Do(
		func() error {
			err := connector.Connect(ctx)
			if err != nil {
				if errors.Is(err, interfaces.ErrExchangeUnavailable) ||
				   errors.Is(err, interfaces.ErrRateLimitExceeded) {
					// These errors are retriable
					return err
				}
				// Other errors should not be retried
				return retry.Unrecoverable(err)
			}
			return nil
		},
		retry.Attempts(3),
		retry.Delay(time.Second),
		retry.DelayType(retry.BackOffDelay),
	)
	
	if err != nil {
		log.Fatalf("Failed to connect after retries: %v", err)
	}
	defer connector.Close()
	
	// Continue with exchange operations...
}
```

## Configuration Options

The `ExchangeOptions` struct provides various configuration options:

```go
// Option 1: Using method chaining for credential setup
options := interfaces.NewExchangeOptions().WithCredentials("your-api-key", "your-api-secret")

// Option 2: Direct setup of individual properties
options := interfaces.NewExchangeOptions()
options.APIKey = "your-api-key"
options.APISecret = "your-api-secret"

// Additional configuration
options.BaseURL = "https://custom-endpoint.com"  // Optional custom API endpoint
options.HTTPTimeout = 30 * time.Second           // Increase HTTP timeout
options.MaxRequestsPerSecond = 5                 // Conservative rate limiting
options.WSReconnectInterval = 3 * time.Second    // Faster reconnection
options.WSHeartbeatInterval = 15 * time.Second   // More frequent heartbeats
options.LogLevel = "debug"                       // More verbose logging

// Create connector with options
connector := bybit.NewConnector(options)

// Shorthand for authenticated connector (inline)
connector := bybit.NewConnector(interfaces.NewExchangeOptions().WithCredentials("your-api-key", "your-api-secret"))
```

## Development

For contributing to the project:

1. Clone the repository:
```bash
git clone https://github.com/veiloq/exchange-connector.git
```

2. Run tests:
```bash
make test
```

3. Run end-to-end tests (requires exchange API credentials):
```bash
make e2e-test
```

4. Lint the code:
```bash
make lint
```

5. Build the documentation:
```bash
make docs
```

## Best Practices

- Always call `Close()` when you're done with a connector to release resources
- Use contexts with timeouts for all operations to prevent hanging requests
- Handle rate limit errors with exponential backoff using the retry-go package
- Use the standard error types for consistent error handling
- Consider using WebSocket subscriptions for high-frequency data needs
- For production, customize the rate limiting parameters based on your exchange tier

## License

This project is licensed under the MIT License - see the LICENSE file for details.
