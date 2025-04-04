package interfaces

import (
	"context"
	"time"
)

// ExchangeConnector defines the main interface for interacting with cryptocurrency exchanges.
// All exchange-specific implementations must satisfy this interface.
type ExchangeConnector interface {
	// Core functionality
	Connect(ctx context.Context) error
	Close() error

	// Market data operations
	GetCandles(ctx context.Context, req CandleRequest) ([]Candle, error)
	GetTicker(ctx context.Context, symbol string) (*Ticker, error)
	GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error)

	// WebSocket subscriptions
	SubscribeCandles(ctx context.Context, req CandleSubscription, handler CandleHandler) error
	SubscribeTicker(ctx context.Context, symbols []string, handler TickerHandler) error
	SubscribeOrderBook(ctx context.Context, symbols []string, handler OrderBookHandler) error
	Unsubscribe(ctx context.Context, subscriptionID string) error
}

// ExchangeOptions defines configuration options for exchange connectors
type ExchangeOptions struct {
	APIKey    string
	APISecret string

	HTTPTimeout time.Duration

	MaxRequestsPerSecond int

	WSReconnectInterval time.Duration
	WSHeartbeatInterval time.Duration

	LogLevel string
}

// Candle represents OHLCV market data
type Candle struct {
	Symbol    string
	StartTime time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// CandleRequest defines parameters for historical candle requests
type CandleRequest struct {
	Symbol    string
	Interval  string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

// CandleSubscription defines parameters for real-time candle subscriptions
type CandleSubscription struct {
	Symbols  []string
	Interval string
}

// Ticker represents current market data
type Ticker struct {
	Symbol    string
	LastPrice float64
	Volume24h float64
}

// OrderBookLevel represents a single level in the order book
type OrderBookLevel struct {
	Price    float64
	Quantity float64
}

// OrderBook represents the current state of the order book
type OrderBook struct {
	Symbol string
	Bids   []OrderBookLevel
	Asks   []OrderBookLevel
}

// Handler types for WebSocket callbacks
type (
	CandleHandler    func(Candle)
	TickerHandler    func(Ticker)
	OrderBookHandler func(OrderBook)
)

// NewExchangeOptions returns default exchange options
func NewExchangeOptions() *ExchangeOptions {
	return &ExchangeOptions{
		HTTPTimeout:          15 * time.Second,
		MaxRequestsPerSecond: 10,
		WSReconnectInterval:  5 * time.Second,
		WSHeartbeatInterval:  20 * time.Second,
		LogLevel:             "info",
	}
}
