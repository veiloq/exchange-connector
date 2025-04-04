package bybit

import (
	"context"
	"fmt"
	"time"

	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
	"github.com/veiloq/exchange-connector/pkg/logging"
	"github.com/veiloq/exchange-connector/pkg/websocket"
)

// Supported time intervals for Bybit candles
var supportedIntervals = map[string]bool{
	"1m":  true,
	"3m":  true,
	"5m":  true,
	"15m": true,
	"30m": true,
	"1h":  true,
	"2h":  true,
	"4h":  true,
	"6h":  true,
	"12h": true,
	"1d":  true,
	"1w":  true,
	"1M":  true,
}

// Connector implements the exchange connector for Bybit API
// It provides methods for connecting to Bybit's API endpoints
// and subscribing to various data streams.
type Connector struct {
	options *interfaces.ExchangeOptions
	ws      websocket.WSConnector
	logger  logging.Logger

	// Track if connected
	connected bool

	// Track active subscriptions by ID
	subscriptions map[string]struct{}
}

// NewConnector creates a new Bybit exchange connector with the given options.
//
// Parameters:
// - options: Configuration options for the connector. If nil, default options are used.
//
// Returns:
// - *Connector: A new Bybit connector instance (not yet connected)
//
// Example:
//
//	options := &interfaces.ExchangeOptions{
//		APIKey:    "your-api-key",
//		APISecret: "your-api-secret",
//		BaseURL:   "wss://stream.bybit.com/v5/public/spot", // Optional override
//	}
//	connector := bybit.NewConnector(options)
func NewConnector(options *interfaces.ExchangeOptions) *Connector {
	// Use default options if none provided
	if options == nil {
		options = interfaces.NewExchangeOptions()
	}

	// Set default WebSocket URL if not specified
	baseURL := "wss://stream.bybit.com/v5/public/spot"
	if options.BaseURL != "" {
		baseURL = options.BaseURL
	}

	logger := logging.NewLogger()
	if options.LogLevel != "" {
		switch options.LogLevel {
		case "debug":
			logger.SetLevel(logging.DEBUG)
		case "info":
			logger.SetLevel(logging.INFO)
		case "warn":
			logger.SetLevel(logging.WARN)
		case "error":
			logger.SetLevel(logging.ERROR)
		}
	}

	// Set default values for WebSocket config
	heartbeatInterval := 20 * time.Second
	if options.WSHeartbeatInterval > 0 {
		heartbeatInterval = options.WSHeartbeatInterval
	}

	reconnectInterval := 5 * time.Second
	if options.WSReconnectInterval > 0 {
		reconnectInterval = options.WSReconnectInterval
	}

	return &Connector{
		options: options,
		ws: websocket.NewConnector(websocket.Config{
			URL:               baseURL,
			HeartbeatInterval: heartbeatInterval,
			ReconnectInterval: reconnectInterval,
			MaxRetries:        3,
		}),
		logger:        logger,
		connected:     false,
		subscriptions: make(map[string]struct{}),
	}
}

// Connect establishes a connection to the Bybit WebSocket API.
// It should be called before using any other methods that require a connection.
//
// Parameters:
// - ctx: Context for controlling the connection timeout/cancellation
//
// Returns:
// - error: An error if the connection cannot be established
//
// Example:
//
//	ctx := context.Background()
//	if err := connector.Connect(ctx); err != nil {
//		log.Fatalf("Failed to connect: %v", err)
//	}
func (c *Connector) Connect(ctx context.Context) error {
	// Log connection attempt with the URL
	c.logger.Info("connecting to Bybit WebSocket API",
		logging.String("url", c.ws.GetConfig().URL))

	if err := c.ws.Connect(ctx); err != nil {
		c.logger.Error("failed to connect to Bybit", logging.Error(err))
		return fmt.Errorf("failed to connect to Bybit WebSocket API: %w", err)
	}

	c.connected = true
	c.logger.Info("successfully connected to Bybit WebSocket API")
	return nil
}

// Close properly terminates the connection to the Bybit WebSocket API.
// It should be called when the connector is no longer needed to free resources.
//
// Returns:
// - error: An error if the connection cannot be closed properly
//
// Example:
//
//	defer connector.Close()
func (c *Connector) Close() error {
	if !c.connected {
		return interfaces.ErrNotConnected
	}

	c.logger.Info("closing connection to Bybit WebSocket API")
	err := c.ws.Close()
	c.connected = false
	c.subscriptions = make(map[string]struct{})

	if err != nil {
		c.logger.Error("error closing WebSocket connection", logging.Error(err))
		return fmt.Errorf("error closing Bybit WebSocket connection: %w", err)
	}

	c.logger.Info("successfully closed connection to Bybit WebSocket API")
	return nil
}

// isValidSymbol checks if a trading pair symbol is valid
func (c *Connector) isValidSymbol(symbol string) bool {
	// TODO: Implement proper symbol validation against Bybit supported symbols
	// For now, just check if it's not empty and has a reasonable format
	return symbol != "" && len(symbol) >= 5
}

// isValidInterval checks if a time interval is supported by Bybit
func (c *Connector) isValidInterval(interval string) bool {
	_, ok := supportedIntervals[interval]
	return ok
}

// validateTimeRange checks if a time range is valid
func (c *Connector) validateTimeRange(startTime, endTime time.Time) error {
	if startTime.IsZero() || endTime.IsZero() {
		return interfaces.ErrInvalidTimeRange
	}

	if endTime.Before(startTime) {
		return interfaces.ErrInvalidTimeRange
	}

	// Bybit has a maximum time range of 200 candles by default
	// We could add additional validation here based on the interval
	return nil
}

// GetCandles retrieves historical candlestick data for a specified symbol and time interval.
// This is a synchronous request that returns historical candle data.
//
// Parameters:
// - ctx: Context for timeout/cancellation control
// - req: CandleRequest containing symbol, timeframe, and time range parameters
//
// Returns:
// - []interfaces.Candle: Array of candle data
// - error: An error if the request fails or parameters are invalid
//
// Example:
//
//	req := interfaces.CandleRequest{
//		Symbol:   "BTCUSDT",
//		Interval: "1h",
//		StartTime: time.Now().Add(-24 * time.Hour),
//		EndTime:   time.Now(),
//		Limit:    100,
//	}
//	candles, err := connector.GetCandles(ctx, req)
//	if err != nil {
//		log.Printf("Error getting candles: %v", err)
//		return
//	}
//	for _, candle := range candles {
//		fmt.Printf("Time: %v, Open: %.2f, Close: %.2f\n", candle.StartTime, candle.Open, candle.Close)
//	}
func (c *Connector) GetCandles(ctx context.Context, req interfaces.CandleRequest) ([]interfaces.Candle, error) {
	// Connection check
	if !c.connected {
		return nil, interfaces.ErrNotConnected
	}

	// Validate parameters
	if !c.isValidSymbol(req.Symbol) {
		return nil, interfaces.ErrInvalidSymbol
	}

	if !c.isValidInterval(req.Interval) {
		return nil, interfaces.ErrInvalidInterval
	}

	if err := c.validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Enforce reasonable limits
	if req.Limit <= 0 {
		req.Limit = 100 // Default limit
	} else if req.Limit > 1000 {
		req.Limit = 1000 // Maximum limit
	}

	// TODO: Implement actual candle fetching from Bybit API
	// For now, return mock data
	c.logger.Info("fetching candles",
		logging.String("symbol", req.Symbol),
		logging.String("interval", req.Interval),
		logging.Int("limit", req.Limit))

	// Mock data for the sample
	candles := make([]interfaces.Candle, 0, req.Limit)
	for i := 0; i < req.Limit; i++ {
		candle := interfaces.Candle{
			Symbol:    req.Symbol,
			StartTime: req.StartTime.Add(time.Duration(i) * getIntervalDuration(req.Interval)),
			Open:      50000 + float64(i),
			High:      51000 + float64(i),
			Low:       49000 + float64(i),
			Close:     50500 + float64(i),
			Volume:    100 + float64(i),
		}
		candles = append(candles, candle)
	}

	c.logger.Info("successfully fetched candles",
		logging.String("symbol", req.Symbol),
		logging.Int("count", len(candles)))
	return candles, nil
}

// Helper function to convert interval string to time.Duration
func getIntervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "2h":
		return 2 * time.Hour
	case "4h":
		return 4 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	case "1M":
		return 30 * 24 * time.Hour // Approximation
	default:
		return time.Hour // Default fallback
	}
}

// SubscribeCandles sets up a real-time subscription for candlestick updates.
// The provided handler function will be called whenever new candle data is available.
//
// Parameters:
// - ctx: Context for controlling the subscription lifetime
// - sub: CandleSubscription with symbols and interval to subscribe to
// - handler: Callback function that will be invoked with each candle update
//
// Returns:
// - error: An error if the subscription cannot be established
//
// Example:
//
//	sub := interfaces.CandleSubscription{
//		Symbols:  []string{"BTCUSDT"},
//		Interval: "1m",
//	}
//	err := connector.SubscribeCandles(ctx, sub, func(candle interfaces.Candle) {
//		fmt.Printf("New candle for %s: Open=%.2f, Close=%.2f, Volume=%.2f\n",
//			candle.Symbol, candle.Open, candle.Close, candle.Volume)
//	})
//	if err != nil {
//		log.Printf("Error subscribing to candles: %v", err)
//	}
func (c *Connector) SubscribeCandles(ctx context.Context, sub interfaces.CandleSubscription, handler interfaces.CandleHandler) error {
	// Connection check
	if !c.connected {
		return interfaces.ErrNotConnected
	}

	// Parameter validation
	if len(sub.Symbols) == 0 {
		return interfaces.ErrInvalidSymbol
	}

	// Check that all symbols are valid
	for _, symbol := range sub.Symbols {
		if !c.isValidSymbol(symbol) {
			return interfaces.NewMarketError(symbol, "invalid symbol", interfaces.ErrInvalidSymbol)
		}
	}

	if !c.isValidInterval(sub.Interval) {
		return interfaces.ErrInvalidInterval
	}

	// Subscribe to kline updates for each symbol
	for _, symbol := range sub.Symbols {
		topic := fmt.Sprintf("kline.%s.%s", sub.Interval, symbol)

		// Track the subscription
		c.subscriptions[topic] = struct{}{}

		c.logger.Info("subscribing to candle updates",
			logging.String("symbol", symbol),
			logging.String("interval", sub.Interval),
			logging.String("topic", topic))

		if err := c.ws.Subscribe(topic, func(message []byte) {
			// TODO: Implement actual message parsing from Bybit format
			// For now, just send a mock candle
			handler(interfaces.Candle{
				Symbol:    symbol,
				StartTime: time.Now().Truncate(getIntervalDuration(sub.Interval)),
				Open:      50000,
				High:      51000,
				Low:       49000,
				Close:     50500,
				Volume:    100,
			})
		}); err != nil {
			c.logger.Error("failed to subscribe to candle updates",
				logging.String("symbol", symbol),
				logging.String("interval", sub.Interval),
				logging.Error(err))
			return interfaces.NewMarketError(symbol, "failed to subscribe to candle updates", err)
		}
	}

	return nil
}

// GetTicker retrieves the current market ticker information for a specific symbol.
// This includes the latest price, volume, and other market statistics.
//
// Parameters:
// - ctx: Context for timeout/cancellation control
// - symbol: The trading pair symbol to get data for (e.g., "BTCUSDT")
//
// Returns:
// - *interfaces.Ticker: Ticker data for the requested symbol
// - error: An error if the request fails or parameters are invalid
//
// Example:
//
//	ticker, err := connector.GetTicker(ctx, "BTCUSDT")
//	if err != nil {
//		log.Printf("Error getting ticker: %v", err)
//		return
//	}
//	fmt.Printf("Current price of %s: %.2f, 24h volume: %.2f\n",
//		ticker.Symbol, ticker.LastPrice, ticker.Volume24h)
func (c *Connector) GetTicker(ctx context.Context, symbol string) (*interfaces.Ticker, error) {
	// Connection check
	if !c.connected {
		return nil, interfaces.ErrNotConnected
	}

	// Validate parameters
	if !c.isValidSymbol(symbol) {
		return nil, interfaces.ErrInvalidSymbol
	}

	// TODO: Implement actual ticker fetching from Bybit API
	// For now, return mock data
	c.logger.Info("fetching ticker", logging.String("symbol", symbol))

	ticker := &interfaces.Ticker{
		Symbol:    symbol,
		LastPrice: 50000,
		Volume24h: 1000,
	}

	return ticker, nil
}

// SubscribeTicker sets up a real-time subscription for ticker updates.
// The provided handler function will be called whenever the ticker data is updated.
//
// Parameters:
// - ctx: Context for controlling the subscription lifetime
// - symbols: List of trading pair symbols to subscribe to
// - handler: Callback function that will be invoked with each ticker update
//
// Returns:
// - error: An error if the subscription cannot be established
//
// Example:
//
//	symbols := []string{"BTCUSDT", "ETHUSDT"}
//	err := connector.SubscribeTicker(ctx, symbols, func(ticker interfaces.Ticker) {
//		fmt.Printf("Ticker update for %s: Price=%.2f, Volume=%.2f\n",
//			ticker.Symbol, ticker.LastPrice, ticker.Volume24h)
//	})
//	if err != nil {
//		log.Printf("Error subscribing to ticker: %v", err)
//	}
func (c *Connector) SubscribeTicker(ctx context.Context, symbols []string, handler interfaces.TickerHandler) error {
	// Connection check
	if !c.connected {
		return interfaces.ErrNotConnected
	}

	// Parameter validation
	if len(symbols) == 0 {
		return interfaces.ErrInvalidSymbol
	}

	// Check that all symbols are valid
	for _, symbol := range symbols {
		if !c.isValidSymbol(symbol) {
			return interfaces.NewMarketError(symbol, "invalid symbol", interfaces.ErrInvalidSymbol)
		}
	}

	// Subscribe to ticker updates for each symbol
	for _, symbol := range symbols {
		topic := fmt.Sprintf("ticker.%s", symbol)

		// Track the subscription
		c.subscriptions[topic] = struct{}{}

		c.logger.Info("subscribing to ticker updates",
			logging.String("symbol", symbol),
			logging.String("topic", topic))

		if err := c.ws.Subscribe(topic, func(message []byte) {
			// TODO: Implement actual message parsing from Bybit format
			// For now, just send a mock ticker
			handler(interfaces.Ticker{
				Symbol:    symbol,
				LastPrice: 50000,
				Volume24h: 1000,
			})
		}); err != nil {
			c.logger.Error("failed to subscribe to ticker updates",
				logging.String("symbol", symbol),
				logging.Error(err))
			return interfaces.NewMarketError(symbol, "failed to subscribe to ticker updates", err)
		}
	}

	return nil
}

// GetOrderBook retrieves the current order book for a specific symbol with the specified depth.
// This provides a snapshot of all buy and sell orders currently on the market.
//
// Parameters:
// - ctx: Context for timeout/cancellation control
// - symbol: The trading pair symbol to get data for (e.g., "BTCUSDT")
// - depth: Maximum number of price levels to retrieve
//
// Returns:
// - *interfaces.OrderBook: Order book data with bids and asks
// - error: An error if the request fails or parameters are invalid
//
// Example:
//
//	orderBook, err := connector.GetOrderBook(ctx, "BTCUSDT", 20)
//	if err != nil {
//		log.Printf("Error getting order book: %v", err)
//		return
//	}
//	fmt.Printf("Order book for %s:\n", orderBook.Symbol)
//	fmt.Println("Bids:")
//	for _, bid := range orderBook.Bids {
//		fmt.Printf("  Price: %.2f, Quantity: %.8f\n", bid.Price, bid.Quantity)
//	}
//	fmt.Println("Asks:")
//	for _, ask := range orderBook.Asks {
//		fmt.Printf("  Price: %.2f, Quantity: %.8f\n", ask.Price, ask.Quantity)
//	}
func (c *Connector) GetOrderBook(ctx context.Context, symbol string, depth int) (*interfaces.OrderBook, error) {
	// Connection check
	if !c.connected {
		return nil, interfaces.ErrNotConnected
	}

	// Validate parameters
	if !c.isValidSymbol(symbol) {
		return nil, interfaces.ErrInvalidSymbol
	}

	// Enforce reasonable depth limits
	if depth <= 0 {
		depth = 20 // Default depth
	} else if depth > 200 {
		depth = 200 // Maximum depth
	}

	// TODO: Implement actual order book fetching from Bybit API
	// For now, return mock data
	c.logger.Info("fetching order book",
		logging.String("symbol", symbol),
		logging.Int("depth", depth))

	// Create mock order book with specified depth
	bids := make([]interfaces.OrderBookLevel, depth)
	asks := make([]interfaces.OrderBookLevel, depth)

	basePrice := 50000.0
	for i := 0; i < depth; i++ {
		// Bids decrease in price (highest first)
		bids[i] = interfaces.OrderBookLevel{
			Price:    basePrice - float64(i)*10,
			Quantity: 1.0 + float64(i)*0.1,
		}

		// Asks increase in price (lowest first)
		asks[i] = interfaces.OrderBookLevel{
			Price:    basePrice + float64(i)*10,
			Quantity: 1.0 + float64(i)*0.1,
		}
	}

	orderBook := &interfaces.OrderBook{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
	}

	return orderBook, nil
}

// SubscribeOrderBook sets up a real-time subscription for order book updates.
// The provided handler function will be called whenever the order book changes.
//
// Parameters:
// - ctx: Context for controlling the subscription lifetime
// - symbols: List of trading pair symbols to subscribe to
// - handler: Callback function that will be invoked with each order book update
//
// Returns:
// - error: An error if the subscription cannot be established
//
// Example:
//
//	symbols := []string{"BTCUSDT"}
//	err := connector.SubscribeOrderBook(ctx, symbols, func(orderBook interfaces.OrderBook) {
//		fmt.Printf("Order book update for %s, %d bids, %d asks\n",
//			orderBook.Symbol, len(orderBook.Bids), len(orderBook.Asks))
//
//		// Print top bid and ask
//		if len(orderBook.Bids) > 0 && len(orderBook.Asks) > 0 {
//			spread := orderBook.Asks[0].Price - orderBook.Bids[0].Price
//			fmt.Printf("Top bid: %.2f, Top ask: %.2f, Spread: %.2f\n",
//				orderBook.Bids[0].Price, orderBook.Asks[0].Price, spread)
//		}
//	})
//	if err != nil {
//		log.Printf("Error subscribing to order book: %v", err)
//	}
func (c *Connector) SubscribeOrderBook(ctx context.Context, symbols []string, handler interfaces.OrderBookHandler) error {
	// Connection check
	if !c.connected {
		return interfaces.ErrNotConnected
	}

	// Parameter validation
	if len(symbols) == 0 {
		return interfaces.ErrInvalidSymbol
	}

	// Check that all symbols are valid
	for _, symbol := range symbols {
		if !c.isValidSymbol(symbol) {
			return interfaces.NewMarketError(symbol, "invalid symbol", interfaces.ErrInvalidSymbol)
		}
	}

	// Subscribe to order book updates for each symbol
	for _, symbol := range symbols {
		topic := fmt.Sprintf("orderbook.%s", symbol)

		// Track the subscription
		c.subscriptions[topic] = struct{}{}

		c.logger.Info("subscribing to order book updates",
			logging.String("symbol", symbol),
			logging.String("topic", topic))

		if err := c.ws.Subscribe(topic, func(message []byte) {
			// TODO: Implement actual message parsing from Bybit format
			// For now, just send a mock order book
			handler(interfaces.OrderBook{
				Symbol: symbol,
				Bids: []interfaces.OrderBookLevel{
					{Price: 49900, Quantity: 1.0},
					{Price: 49800, Quantity: 2.0},
				},
				Asks: []interfaces.OrderBookLevel{
					{Price: 50100, Quantity: 1.0},
					{Price: 50200, Quantity: 2.0},
				},
			})
		}); err != nil {
			c.logger.Error("failed to subscribe to order book updates",
				logging.String("symbol", symbol),
				logging.Error(err))
			return interfaces.NewMarketError(symbol, "failed to subscribe to order book updates", err)
		}
	}

	return nil
}

// Unsubscribe terminates an active WebSocket subscription.
//
// Parameters:
// - ctx: Context for timeout/cancellation control
// - subscriptionID: The topic identifier to unsubscribe from
//
// Returns:
// - error: An error if the unsubscription fails
//
// Example:
//
//	err := connector.Unsubscribe(ctx, "ticker.BTCUSDT")
//	if err != nil {
//		log.Printf("Error unsubscribing: %v", err)
//	}
func (c *Connector) Unsubscribe(ctx context.Context, subscriptionID string) error {
	// Connection check
	if !c.connected {
		return interfaces.ErrNotConnected
	}

	// Check if subscription exists
	if _, exists := c.subscriptions[subscriptionID]; !exists {
		return interfaces.ErrSubscriptionNotFound
	}

	c.logger.Info("unsubscribing from topic", logging.String("topic", subscriptionID))

	if err := c.ws.Unsubscribe(subscriptionID); err != nil {
		c.logger.Error("failed to unsubscribe from topic",
			logging.String("topic", subscriptionID),
			logging.Error(err))
		return fmt.Errorf("failed to unsubscribe from %s: %w", subscriptionID, err)
	}

	// Remove from tracked subscriptions
	delete(c.subscriptions, subscriptionID)

	c.logger.Info("successfully unsubscribed from topic", logging.String("topic", subscriptionID))
	return nil
}
