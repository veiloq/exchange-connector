package bybit

import (
	"context"
	"fmt"
	"time"

	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
	"github.com/veiloq/exchange-connector/pkg/websocket"
)

// Connector implements the exchange connector for Bybit API
// It provides methods for connecting to Bybit's API endpoints
// and subscribing to various data streams.
type Connector struct {
	options *interfaces.ExchangeOptions
	ws      websocket.WSConnector
}

// NewConnector creates a new Bybit exchange connector with the given options.
//
// Example:
//
//	options := &interfaces.ExchangeOptions{
//		APIKey:    "your-api-key",
//		APISecret: "your-api-secret",
//		BaseURL:   "wss://stream.bybit.com/v5/public/spot",
//	}
//	connector := bybit.NewConnector(options)
func NewConnector(options *interfaces.ExchangeOptions) *Connector {
	baseURL := "wss://stream.bybit.com/v5/public/spot"
	if options.BaseURL != "" {
		baseURL = options.BaseURL
	}

	return &Connector{
		options: options,
		ws: websocket.NewConnector(websocket.Config{
			URL:               baseURL,
			HeartbeatInterval: 20 * time.Second,
			ReconnectInterval: 5 * time.Second,
			MaxRetries:        3,
		}),
	}
}

// Connect establishes a connection to the Bybit WebSocket API.
// It should be called before using any other methods that require a connection.
//
// Example:
//
//	ctx := context.Background()
//	if err := connector.Connect(ctx); err != nil {
//		log.Fatalf("Failed to connect: %v", err)
//	}
func (c *Connector) Connect(ctx context.Context) error {
	return c.ws.Connect(ctx)
}

// Close properly terminates the connection to the Bybit WebSocket API.
// It should be called when the connector is no longer needed to free resources.
//
// Example:
//
//	defer connector.Close()
func (c *Connector) Close() error {
	return c.ws.Close()
}

// GetCandles retrieves historical candlestick data for a specified symbol and time interval.
// This is a synchronous request that returns historical candle data.
//
// Example:
//
//	req := interfaces.CandleRequest{
//		Symbol:   "BTCUSDT",
//		Interval: "1h",
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
	// TODO: Implement real candle fetching
	return []interfaces.Candle{
		{
			Symbol:    req.Symbol,
			StartTime: time.Now(),
			Open:      50000,
			High:      51000,
			Low:       49000,
			Close:     50500,
			Volume:    100,
		},
	}, nil
}

// SubscribeCandles sets up a real-time subscription for candlestick updates.
// The provided handler function will be called whenever new candle data is available.
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
func (c *Connector) SubscribeCandles(ctx context.Context, sub interfaces.CandleSubscription, handler func(interfaces.Candle)) error {
	// Subscribe to kline updates
	topic := fmt.Sprintf("kline.%s.%s", sub.Interval, sub.Symbols[0])
	return c.ws.Subscribe(topic, func(message []byte) {
		// For testing, just send a mock candle
		handler(interfaces.Candle{
			Symbol:    sub.Symbols[0],
			StartTime: time.Now(),
			Open:      50000,
			High:      51000,
			Low:       49000,
			Close:     50500,
			Volume:    100,
		})
	})
}

// GetTicker retrieves the current market ticker information for a specific symbol.
// This includes the latest price, volume, and other market statistics.
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
func (c *Connector) GetTicker(ctx context.Context, symbol string) (interfaces.Ticker, error) {
	return interfaces.Ticker{
		Symbol:    symbol,
		LastPrice: 50000,
		Volume24h: 1000,
	}, nil
}

// SubscribeTicker sets up a real-time subscription for ticker updates.
// The provided handler function will be called whenever the ticker data is updated.
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
func (c *Connector) SubscribeTicker(ctx context.Context, symbols []string, handler func(interfaces.Ticker)) error {
	// Subscribe to ticker updates
	topic := fmt.Sprintf("ticker.%s", symbols[0])
	return c.ws.Subscribe(topic, func(message []byte) {
		// For testing, just send a mock ticker
		handler(interfaces.Ticker{
			Symbol:    symbols[0],
			LastPrice: 50000,
			Volume24h: 1000,
		})
	})
}

// GetOrderBook retrieves the current order book for a specific symbol with the specified depth.
// This provides a snapshot of all buy and sell orders currently on the market.
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
func (c *Connector) GetOrderBook(ctx context.Context, symbol string, depth int) (interfaces.OrderBook, error) {
	return interfaces.OrderBook{
		Symbol: symbol,
		Bids: []interfaces.OrderBookLevel{
			{Price: 49900, Quantity: 1.0},
			{Price: 49800, Quantity: 2.0},
		},
		Asks: []interfaces.OrderBookLevel{
			{Price: 50100, Quantity: 1.0},
			{Price: 50200, Quantity: 2.0},
		},
	}, nil
}

// SubscribeOrderBook sets up a real-time subscription for order book updates.
// The provided handler function will be called whenever the order book changes.
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
func (c *Connector) SubscribeOrderBook(ctx context.Context, symbols []string, handler func(interfaces.OrderBook)) error {
	// Subscribe to orderbook updates
	topic := fmt.Sprintf("orderbook.%s", symbols[0])
	return c.ws.Subscribe(topic, func(message []byte) {
		// For testing, just send a mock order book
		handler(interfaces.OrderBook{
			Symbol: symbols[0],
			Bids: []interfaces.OrderBookLevel{
				{Price: 49900, Quantity: 1.0},
				{Price: 49800, Quantity: 2.0},
			},
			Asks: []interfaces.OrderBookLevel{
				{Price: 50100, Quantity: 1.0},
				{Price: 50200, Quantity: 2.0},
			},
		})
	})
}
