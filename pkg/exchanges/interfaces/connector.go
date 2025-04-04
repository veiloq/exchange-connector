package interfaces

import (
	"context"
	"time"
)

// ExchangeConnector defines the main interface for interacting with cryptocurrency exchanges.
// All exchange-specific implementations must satisfy this interface.
//
// This interface provides a unified way to access market data and trading functionality
// across different cryptocurrency exchanges. It enables both REST API access for
// historical/current data and WebSocket connections for real-time data streaming.
//
// Implementations should handle:
// - Authentication and authorization with the exchange
// - Rate limiting according to exchange requirements
// - Reconnection logic for WebSocket streams
// - Data normalization to match the defined types
// - Error handling and appropriate error wrapping
type ExchangeConnector interface {
	// Core functionality

	// Connect establishes a connection to the exchange using the provided configuration.
	// It initializes any required resources and prepares the connector for data operations.
	//
	// The context can be used to control the connection timeout or cancellation.
	// Returns an error if the connection cannot be established.
	Connect(ctx context.Context) error

	// Close terminates all connections to the exchange and releases any resources.
	// This should be called when the connector is no longer needed to prevent resource leaks.
	//
	// Returns an error if the shutdown process encounters issues.
	Close() error

	// Market data operations

	// GetCandles retrieves historical OHLCV (Open, High, Low, Close, Volume) candle data.
	//
	// Parameters:
	// - ctx: Context for timeout/cancellation control
	// - req: CandleRequest containing symbol, timeframe, and time range parameters
	//
	// Returns:
	// - A slice of Candle objects containing the requested historical data
	// - Error if the request fails or data cannot be retrieved
	//
	// The number of candles returned may be limited by the exchange, even if a larger
	// limit is specified in the request. Some exchanges may require multiple API calls
	// for large date ranges, which implementations should handle transparently.
	GetCandles(ctx context.Context, req CandleRequest) ([]Candle, error)

	// GetTicker retrieves the current market ticker information for a specific symbol.
	//
	// Parameters:
	// - ctx: Context for timeout/cancellation control
	// - symbol: Trading pair symbol in exchange format (e.g., "BTCUSDT")
	//
	// Returns:
	// - Pointer to Ticker object with current price and 24h volume
	// - Error if the request fails or data cannot be retrieved
	GetTicker(ctx context.Context, symbol string) (*Ticker, error)

	// GetOrderBook retrieves the current order book (market depth) for a specific symbol.
	//
	// Parameters:
	// - ctx: Context for timeout/cancellation control
	// - symbol: Trading pair symbol in exchange format (e.g., "BTCUSDT")
	// - depth: Number of price levels to retrieve on each side (bids/asks)
	//
	// Returns:
	// - Pointer to OrderBook object containing bids and asks
	// - Error if the request fails or data cannot be retrieved
	//
	// Note: The actual depth returned may be limited by the exchange's API constraints.
	GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error)

	// WebSocket subscriptions

	// SubscribeCandles establishes a real-time subscription to candle data updates.
	//
	// Parameters:
	// - ctx: Context for controlling the lifetime of the subscription
	// - req: CandleSubscription specifying symbols and interval
	// - handler: Callback function that will be invoked with each new candle update
	//
	// Returns:
	// - Error if the subscription cannot be established
	//
	// The subscription remains active until Unsubscribe is called or the context is canceled.
	// The handler will be called from a goroutine managed by the implementation.
	SubscribeCandles(ctx context.Context, req CandleSubscription, handler CandleHandler) error

	// SubscribeTicker establishes a real-time subscription to ticker updates.
	//
	// Parameters:
	// - ctx: Context for controlling the lifetime of the subscription
	// - symbols: Slice of trading pair symbols to subscribe to
	// - handler: Callback function that will be invoked with each ticker update
	//
	// Returns:
	// - Error if the subscription cannot be established
	//
	// The subscription remains active until Unsubscribe is called or the context is canceled.
	// The handler will be called from a goroutine managed by the implementation.
	SubscribeTicker(ctx context.Context, symbols []string, handler TickerHandler) error

	// SubscribeOrderBook establishes a real-time subscription to order book updates.
	//
	// Parameters:
	// - ctx: Context for controlling the lifetime of the subscription
	// - symbols: Slice of trading pair symbols to subscribe to
	// - handler: Callback function that will be invoked with each order book update
	//
	// Returns:
	// - Error if the subscription cannot be established
	//
	// The subscription remains active until Unsubscribe is called or the context is canceled.
	// The handler will be called from a goroutine managed by the implementation.
	// Depending on the exchange, updates may be full snapshots or incremental updates.
	SubscribeOrderBook(ctx context.Context, symbols []string, handler OrderBookHandler) error

	// Unsubscribe terminates an active WebSocket subscription.
	//
	// Parameters:
	// - ctx: Context for timeout/cancellation control
	// - subscriptionID: Identifier of the subscription to terminate
	//
	// Returns:
	// - Error if the unsubscription fails or the ID is invalid
	Unsubscribe(ctx context.Context, subscriptionID string) error
}

// ExchangeOptions defines configuration options for exchange connectors.
// These options control the behavior and performance characteristics
// of the exchange connector implementation.
type ExchangeOptions struct {
	// APIKey is the authentication key for the exchange API.
	// Required for authenticated endpoints (trading, account info).
	APIKey string

	// APISecret is the secret key paired with the API key.
	// Required for generating signatures for authenticated requests.
	APISecret string

	// HTTPTimeout specifies the maximum duration to wait for HTTP requests.
	// This applies to all REST API calls to the exchange.
	HTTPTimeout time.Duration

	// MaxRequestsPerSecond controls the rate limiting for API requests.
	// Implementation should ensure this rate is not exceeded to prevent
	// IP bans or connection throttling by the exchange.
	MaxRequestsPerSecond int

	// WSReconnectInterval is the duration to wait before attempting
	// to reconnect a dropped WebSocket connection.
	WSReconnectInterval time.Duration

	// WSHeartbeatInterval is the frequency at which heartbeat messages
	// should be sent to keep WebSocket connections alive.
	WSHeartbeatInterval time.Duration

	// LogLevel controls the verbosity of connector logging.
	// Common values include: "debug", "info", "warn", "error"
	LogLevel string
}

// Candle represents OHLCV (Open, High, Low, Close, Volume) market data for a specific time period.
// Each candle represents a discrete time interval in a price chart.
type Candle struct {
	// Symbol is the trading pair identifier (e.g., "BTCUSDT")
	Symbol string

	// StartTime marks the beginning of the time interval represented by this candle
	StartTime time.Time

	// Open is the opening price for the interval
	Open float64

	// High is the highest price reached during the interval
	High float64

	// Low is the lowest price reached during the interval
	Low float64

	// Close is the closing price at the end of the interval
	Close float64

	// Volume is the trading volume during the interval
	Volume float64
}

// CandleRequest defines parameters for historical candle data requests.
// Used to specify the criteria for retrieving OHLCV data from an exchange.
type CandleRequest struct {
	// Symbol is the trading pair to fetch data for (e.g., "BTCUSDT")
	Symbol string

	// Interval specifies the time period each candle represents.
	// Common values include: "1m", "5m", "15m", "1h", "4h", "1d"
	// The exact intervals supported depend on the specific exchange.
	Interval string

	// StartTime marks the beginning of the requested time range
	StartTime time.Time

	// EndTime marks the end of the requested time range
	EndTime time.Time

	// Limit is the maximum number of candles to retrieve.
	// Exchanges typically have a maximum limit (e.g., 1000).
	Limit int
}

// CandleSubscription defines parameters for real-time candle data subscriptions.
// Used when establishing WebSocket connections for streaming OHLCV updates.
type CandleSubscription struct {
	// Symbols is a list of trading pairs to subscribe to
	Symbols []string

	// Interval specifies the time period each candle represents.
	// Common values include: "1m", "5m", "15m", "1h", "4h", "1d"
	// The exact intervals supported depend on the specific exchange.
	Interval string
}

// Ticker represents current market data for a specific trading pair.
// It provides a snapshot of the most recent trading activity.
type Ticker struct {
	// Symbol is the trading pair identifier (e.g., "BTCUSDT")
	Symbol string

	// LastPrice is the most recent traded price
	LastPrice float64

	// Volume24h is the trading volume over the last 24 hours
	Volume24h float64
}

// OrderBookLevel represents a single level in the order book,
// consisting of a price and the available quantity at that price.
type OrderBookLevel struct {
	// Price at which orders are placed
	Price float64

	// Quantity available at the specified price
	Quantity float64
}

// OrderBook represents the current state of the order book for a trading pair.
// It contains lists of buy orders (bids) and sell orders (asks) at various price levels.
type OrderBook struct {
	// Symbol is the trading pair identifier (e.g., "BTCUSDT")
	Symbol string

	// Bids are buy orders, sorted in descending order by price (highest first)
	Bids []OrderBookLevel

	// Asks are sell orders, sorted in ascending order by price (lowest first)
	Asks []OrderBookLevel
}

// Handler types for WebSocket callbacks.
// These function types define the signatures for callback handlers
// that process real-time data from WebSocket subscriptions.
type (
	// CandleHandler processes real-time candle updates.
	// The callback is invoked with each new or updated candle received.
	CandleHandler func(Candle)

	// TickerHandler processes real-time ticker updates.
	// The callback is invoked with each ticker update received.
	TickerHandler func(Ticker)

	// OrderBookHandler processes real-time order book updates.
	// The callback is invoked with each order book update received.
	// Depending on the exchange, this may be a complete snapshot or an incremental update.
	OrderBookHandler func(OrderBook)
)

// NewExchangeOptions returns default exchange options with reasonable values.
// These defaults can be used as a starting point and modified as needed.
//
// Default values:
// - HTTP timeout: 15 seconds
// - Max requests per second: 10
// - WebSocket reconnect interval: 5 seconds
// - WebSocket heartbeat interval: 20 seconds
// - Log level: "info"
//
// Example usage:
//
//	options := NewExchangeOptions()
//	options.APIKey = "your-api-key"
//	options.APISecret = "your-api-secret"
//	connector := bybit.NewConnector(options)
func NewExchangeOptions() *ExchangeOptions {
	return &ExchangeOptions{
		HTTPTimeout:          15 * time.Second,
		MaxRequestsPerSecond: 10,
		WSReconnectInterval:  5 * time.Second,
		WSHeartbeatInterval:  20 * time.Second,
		LogLevel:             "info",
	}
}
