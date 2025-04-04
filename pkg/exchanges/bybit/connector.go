package bybit

import (
	"context"
	"fmt"
	"time"

	"github.com/bybit/pkg/exchanges/interfaces"
	"github.com/bybit/pkg/websocket"
)

type Connector struct {
	options *interfaces.ExchangeOptions
	ws      websocket.WSConnector
}

func NewConnector(options *interfaces.ExchangeOptions) *Connector {
	return &Connector{
		options: options,
		ws: websocket.NewConnector(websocket.Config{
			URL:               "wss://stream.bybit.com/v5/public/spot",
			HeartbeatInterval: 20 * time.Second,
			ReconnectInterval: 5 * time.Second,
			MaxRetries:        3,
		}),
	}
}

func (c *Connector) Connect(ctx context.Context) error {
	return c.ws.Connect(ctx)
}

func (c *Connector) Close() error {
	return c.ws.Close()
}

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

func (c *Connector) GetTicker(ctx context.Context, symbol string) (interfaces.Ticker, error) {
	return interfaces.Ticker{
		Symbol:    symbol,
		LastPrice: 50000,
		Volume24h: 1000,
	}, nil
}

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
