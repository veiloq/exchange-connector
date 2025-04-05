# Exchange Connector

A production-ready Go library for connecting to cryptocurrency exchanges with a unified interface for both REST API and WebSocket connections.

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
	// Create options with credentials
	options := interfaces.NewExchangeOptions()
	options.APIKey = "your-api-key"
	options.APISecret = "your-api-secret"
	
	// Create authenticated connector
	connector := bybit.NewConnector(options)
	
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

## Supported Exchanges

- Bybit
- Binance (coming soon)

## Error Handling

The library provides standardized error types for consistent error handling:

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

## License

This project is licensed under the MIT License - see the LICENSE file for details.
