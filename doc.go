// Package exchange-connector provides a unified interface for interacting with cryptocurrency exchanges.
//
// The library offers a consistent API that abstracts away exchange-specific implementation details,
// allowing applications to work with multiple exchange platforms through a standardized interface.
//
// Core Features:
//
//   - Unified API for multiple cryptocurrency exchanges
//   - Market data operations (candles, tickers, order books)
//   - WebSocket subscriptions for real-time data
//   - Automatic connection management and reconnection
//   - Rate limiting protection based on exchange requirements
//
// The library is built around the ExchangeConnector interface which defines the methods
// for interacting with exchanges, including REST API for historical data and WebSocket
// connections for real-time streaming.
//
// # Standard Errors
//
// The library defines standardized errors to provide consistent error handling across different
// exchange implementations:
//
//   - ErrNotConnected: Returned when an operation is attempted on a connector that hasn't been connected
//     yet or has lost connection
//
//   - ErrInvalidSymbol: Returned when an invalid trading pair symbol is provided
//
//   - ErrInvalidInterval: Returned when an unsupported time interval is provided
//
//   - ErrInvalidTimeRange: Returned when an invalid time range is provided (e.g., end time before start time)
//
//   - ErrRateLimitExceeded: Returned when the exchange rate limit is exceeded
//
//   - ErrAuthenticationRequired: Returned when attempting an operation that requires authentication
//     without providing credentials
//
//   - ErrInvalidCredentials: Returned when the provided API credentials are invalid
//
//   - ErrSubscriptionFailed: Returned when a WebSocket subscription cannot be established
//
//   - ErrSubscriptionNotFound: Returned when trying to unsubscribe from a non-existent subscription
//
//   - ErrExchangeUnavailable: Returned when the exchange API is unavailable
//
// Additionally, the library provides a MarketError type for market-specific error conditions,
// which can be created using NewMarketError(symbol, message, err).
//
// # Examples
//
// Basic usage:
//
// Option 1: Using method chaining for credential setup:
//
//	// Create exchange-specific connector with options and credentials
//	options := interfaces.NewExchangeOptions().WithCredentials("your-api-key", "your-api-secret")
//	connector := bybit.NewConnector(options)
//
// Option 2: Direct credential setup:
//
//	// Create exchange-specific connector with inline credential setup
//	connector := bybit.NewConnector(interfaces.NewExchangeOptions().WithCredentials("your-api-key", "your-api-secret"))
//
//	// Connect to the exchange
//	ctx := context.Background()
//	if err := connector.Connect(ctx); err != nil {
//	    log.Fatalf("Failed to connect: %v", err)
//	}
//	defer connector.Close()
//
// # Candle Examples
//
// Getting historical candle data:
//
//	// Get historical candle data for the last hour with 1-minute intervals
//	candles, err := connector.GetCandles(ctx, interfaces.CandleRequest{
//	    Symbol:    "BTCUSDT",
//	    Interval:  "1m",
//	    StartTime: time.Now().Add(-1 * time.Hour),
//	    EndTime:   time.Now(),
//	    Limit:     60,
//	})
//
//	if err != nil {
//	    switch {
//	    case errors.Is(err, interfaces.ErrInvalidSymbol):
//	        log.Fatalf("Invalid trading pair symbol: %s", "BTCUSDT")
//	    case errors.Is(err, interfaces.ErrInvalidInterval):
//	        log.Fatalf("Invalid time interval: %s", "1m")
//	    case errors.Is(err, interfaces.ErrInvalidTimeRange):
//	        log.Fatalf("Invalid time range specified")
//	    default:
//	        log.Fatalf("Failed to get candles: %v", err)
//	    }
//	}
//
//	fmt.Printf("Retrieved %d candles for BTCUSDT\n", len(candles))
//	for i, candle := range candles[:3] {
//	    fmt.Printf("%d. %s | Open: %.2f, Close: %.2f\n",
//	        i+1,
//	        candle.StartTime.Format("15:04:05"),
//	        candle.Open,
//	        candle.Close)
//	}
//
// Getting daily candles for longer-term analysis:
//
//	// Get daily candles for the last month
//	candles, err := connector.GetCandles(ctx, interfaces.CandleRequest{
//	    Symbol:    "BTCUSDT",
//	    Interval:  "1d",  // Daily candles
//	    StartTime: time.Now().Add(-30 * 24 * time.Hour), // 30 days ago
//	    EndTime:   time.Now(),
//	    Limit:     30,
//	})
//
//	// Process the daily candles
//	var totalVolume float64
//	for _, candle := range candles {
//	    totalVolume += candle.Volume
//
//	    // Calculate price change percentage
//	    priceChange := (candle.Close - candle.Open) / candle.Open * 100
//	    fmt.Printf("%s: Change: %.2f%%, Volume: %.2f\n",
//	        candle.StartTime.Format("2006-01-02"),
//	        priceChange,
//	        candle.Volume)
//	}
//
// Subscribing to real-time candle updates:
//
//	// Subscribe to real-time 1-minute candle updates for BTCUSDT
//	symbols := []string{"BTCUSDT"}
//	subscription := interfaces.CandleSubscription{
//	    Symbols:  symbols,
//	    Interval: "1m",
//	}
//
//	err := connector.SubscribeCandles(ctx, subscription, func(candle interfaces.Candle) {
//	    fmt.Printf("[%s] BTCUSDT | Open: $%.2f | High: $%.2f | Low: $%.2f | Close: $%.2f | Volume: %.2f\n",
//	        candle.StartTime.Format("15:04:05"),
//	        candle.Open,
//	        candle.High,
//	        candle.Low,
//	        candle.Close,
//	        candle.Volume)
//	})
//
//	if err != nil {
//	    switch {
//	    case errors.Is(err, interfaces.ErrInvalidSymbol):
//	        log.Fatalf("Invalid trading pair symbol provided")
//	    case errors.Is(err, interfaces.ErrInvalidInterval):
//	        log.Fatalf("Invalid candle interval specified")
//	    case errors.Is(err, interfaces.ErrSubscriptionFailed):
//	        log.Fatalf("Failed to establish subscription")
//	    default:
//	        log.Fatalf("Subscription failed: %v", err)
//	    }
//	}
//
// # Other examples
//
// Order book subscription:
//
//	// Subscribe to real-time order book updates
//	err = connector.SubscribeOrderBook(ctx, []string{"BTCUSDT"}, func(book interfaces.OrderBook) {
//	    // Process order book update
//	    fmt.Printf("Best bid: %f, Best ask: %f\n", book.Bids[0].Price, book.Asks[0].Price)
//	})
//
// The library provides concrete implementations for various exchanges while maintaining
// a consistent interface, enabling applications to easily switch between or support
// multiple exchanges simultaneously.
package exchangeconnector
