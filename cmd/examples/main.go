package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/veiloq/exchange-connector/pkg/exchanges/bybit"
	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
	"github.com/veiloq/exchange-connector/pkg/logging"
)

func main() {
	// Create logger
	logger := logging.NewLogger()
	logger.SetLevel(logging.DEBUG)

	// Create exchange options
	options := &interfaces.ExchangeOptions{
		// API credentials (optional for public endpoints)
		APIKey:    os.Getenv("BYBIT_API_KEY"),
		APISecret: os.Getenv("BYBIT_API_SECRET"),

		// Connection settings
		HTTPTimeout: 15 * time.Second,

		// Rate limiting
		MaxRequestsPerSecond: 10,

		// WebSocket settings
		WSReconnectInterval: 5 * time.Second,
		WSHeartbeatInterval: 20 * time.Second,

		// Logging
		LogLevel: "debug",
	}

	// Create Bybit connector
	connector := bybit.NewConnector(options)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to exchange
	logger.Info("connecting to exchange")
	if err := connector.Connect(ctx); err != nil {
		logger.Error("failed to connect", logging.Error(err))
		os.Exit(1)
	}
	defer connector.Close()

	// Get historical candles
	logger.Info("fetching historical candles")
	candles, err := connector.GetCandles(ctx, interfaces.CandleRequest{
		Symbol:    "BTCUSDT",
		Interval:  "1m",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Limit:     60,
	})
	if err != nil {
		logger.Error("failed to get candles", logging.Error(err))
		os.Exit(1)
	}

	// Print historical candles
	for _, candle := range candles {
		logger.Info("historical candle",
			logging.String("symbol", candle.Symbol),
			logging.String("time", candle.StartTime.Format(time.RFC3339)),
			logging.Float64("open", candle.Open),
			logging.Float64("close", candle.Close),
		)
	}

	// Subscribe to real-time candle updates
	logger.Info("subscribing to real-time candles")
	err = connector.SubscribeCandles(ctx, interfaces.CandleSubscription{
		Symbols:  []string{"BTCUSDT", "ETHUSDT"},
		Interval: "1m",
	}, func(candle interfaces.Candle) {
		logger.Info("real-time candle",
			logging.String("symbol", candle.Symbol),
			logging.String("time", candle.StartTime.Format(time.RFC3339)),
			logging.Float64("open", candle.Open),
			logging.Float64("close", candle.Close),
		)
	})
	if err != nil {
		logger.Error("failed to subscribe to candles", logging.Error(err))
		os.Exit(1)
	}

	// Get current ticker
	logger.Info("fetching current ticker")
	ticker, err := connector.GetTicker(ctx, "BTCUSDT")
	if err != nil {
		logger.Error("failed to get ticker", logging.Error(err))
		os.Exit(1)
	}

	logger.Info("current ticker",
		logging.String("symbol", ticker.Symbol),
		logging.Float64("last_price", ticker.LastPrice),
		logging.Float64("24h_volume", ticker.Volume24h),
	)

	// Subscribe to ticker updates
	logger.Info("subscribing to ticker updates")
	err = connector.SubscribeTicker(ctx, []string{"BTCUSDT", "ETHUSDT"},
		func(ticker interfaces.Ticker) {
			logger.Info("ticker update",
				logging.String("symbol", ticker.Symbol),
				logging.Float64("last_price", ticker.LastPrice),
				logging.Float64("24h_volume", ticker.Volume24h),
			)
		})
	if err != nil {
		logger.Error("failed to subscribe to ticker", logging.Error(err))
		os.Exit(1)
	}

	// Get order book
	logger.Info("fetching order book")
	orderBook, err := connector.GetOrderBook(ctx, "BTCUSDT", 25)
	if err != nil {
		logger.Error("failed to get order book", logging.Error(err))
		os.Exit(1)
	}

	logger.Info("order book snapshot",
		logging.String("symbol", orderBook.Symbol),
		logging.Int("bid_levels", len(orderBook.Bids)),
		logging.Int("ask_levels", len(orderBook.Asks)),
	)

	// Subscribe to order book updates
	logger.Info("subscribing to order book updates")
	err = connector.SubscribeOrderBook(ctx, []string{"BTCUSDT"},
		func(orderBook interfaces.OrderBook) {
			logger.Info("order book update",
				logging.String("symbol", orderBook.Symbol),
				logging.Int("bid_levels", len(orderBook.Bids)),
				logging.Int("ask_levels", len(orderBook.Asks)),
			)
		})
	if err != nil {
		logger.Error("failed to subscribe to order book", logging.Error(err))
		os.Exit(1)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("running... press Ctrl+C to exit")
	<-sigChan

	// Cleanup
	logger.Info("shutting down")
	cancel()
}
