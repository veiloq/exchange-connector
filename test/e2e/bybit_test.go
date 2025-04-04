package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/bybit/pkg/exchanges/bybit"
	"github.com/bybit/pkg/exchanges/interfaces"
	"github.com/bybit/pkg/logging"
	"github.com/stretchr/testify/require"
)

// TestBybitConnector_E2E performs end-to-end testing of the Bybit connector
// against the actual Bybit API.
//
// To run this test:
// BYBIT_API_KEY=your_api_key BYBIT_API_SECRET=your_api_secret go test -v -tags=e2e ./test/e2e
func TestBybitConnector_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Create logger for debugging
	logger := logging.NewLogger()
	logger.SetLevel(logging.DEBUG)

	// Get API credentials
	apiKey := os.Getenv("BYBIT_API_KEY")
	apiSecret := os.Getenv("BYBIT_API_SECRET")

	// Check if we're running in CI or missing credentials
	runningInCI := os.Getenv("CI") != ""

	// Create exchange options
	options := &interfaces.ExchangeOptions{
		APIKey:    apiKey,
		APISecret: apiSecret,
		LogLevel:  "debug",
	}

	// Create connector
	connector := bybit.NewConnector(options)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Connect to exchange
	err := connector.Connect(ctx)
	require.NoError(t, err, "failed to connect to exchange")
	defer connector.Close()

	// Test getting historical candles
	t.Run("GetCandles", func(t *testing.T) {
		candles, err := connector.GetCandles(ctx, interfaces.CandleRequest{
			Symbol:    "BTCUSDT",
			Interval:  "1m",
			StartTime: time.Now().Add(-1 * time.Hour),
			EndTime:   time.Now(),
			Limit:     60,
		})
		require.NoError(t, err, "failed to get candles")
		require.NotEmpty(t, candles, "no candles returned")
		require.Equal(t, "BTCUSDT", candles[0].Symbol)
	})

	// Test getting ticker
	t.Run("GetTicker", func(t *testing.T) {
		ticker, err := connector.GetTicker(ctx, "BTCUSDT")
		require.NoError(t, err, "failed to get ticker")
		require.Equal(t, "BTCUSDT", ticker.Symbol)
		require.Greater(t, ticker.LastPrice, float64(0))
	})

	// Test getting order book
	t.Run("GetOrderBook", func(t *testing.T) {
		orderBook, err := connector.GetOrderBook(ctx, "BTCUSDT", 25)
		require.NoError(t, err, "failed to get order book")
		require.Equal(t, "BTCUSDT", orderBook.Symbol)
		require.NotEmpty(t, orderBook.Bids)
		require.NotEmpty(t, orderBook.Asks)
		require.LessOrEqual(t, len(orderBook.Bids), 25)
		require.LessOrEqual(t, len(orderBook.Asks), 25)
	})

	// Test WebSocket subscriptions
	t.Run("WebSocketSubscriptions", func(t *testing.T) {
		// Skip this test if we don't have valid API credentials or we're running in CI
		if apiKey == "" || apiSecret == "" || runningInCI {
			t.Skip("Skipping WebSocket subscription test - requires valid API credentials and not running in CI")
			return
		}

		// Channel to receive updates
		candleCh := make(chan interfaces.Candle, 10)
		tickerCh := make(chan interfaces.Ticker, 10)
		orderBookCh := make(chan interfaces.OrderBook, 10)

		// Subscribe to candles
		err := connector.SubscribeCandles(ctx, interfaces.CandleSubscription{
			Symbols:  []string{"BTCUSDT"},
			Interval: "1m",
		}, func(candle interfaces.Candle) {
			select {
			case candleCh <- candle:
			default:
			}
		})
		require.NoError(t, err, "failed to subscribe to candles")

		// Subscribe to ticker
		err = connector.SubscribeTicker(ctx, []string{"BTCUSDT"},
			func(ticker interfaces.Ticker) {
				select {
				case tickerCh <- ticker:
				default:
				}
			})
		require.NoError(t, err, "failed to subscribe to ticker")

		// Subscribe to order book
		err = connector.SubscribeOrderBook(ctx, []string{"BTCUSDT"},
			func(orderBook interfaces.OrderBook) {
				select {
				case orderBookCh <- orderBook:
				default:
				}
			})
		require.NoError(t, err, "failed to subscribe to order book")

		// Wait for updates with retry
		var receivedCandle, receivedTicker, receivedOrderBook bool

		err = retry.Do(
			func() error {
				// Check candles
				if !receivedCandle {
					select {
					case candle := <-candleCh:
						if candle.Symbol == "BTCUSDT" {
							receivedCandle = true
						}
					default:
						// No message yet
					}
				}

				// Check ticker
				if !receivedTicker {
					select {
					case ticker := <-tickerCh:
						if ticker.Symbol == "BTCUSDT" {
							receivedTicker = true
						}
					default:
						// No message yet
					}
				}

				// Check order book
				if !receivedOrderBook {
					select {
					case orderBook := <-orderBookCh:
						if orderBook.Symbol == "BTCUSDT" {
							receivedOrderBook = true
						}
					default:
						// No message yet
					}
				}

				// If we haven't received all updates, return an error to retry
				if !receivedCandle || !receivedTicker || !receivedOrderBook {
					return fmt.Errorf("waiting for WebSocket updates")
				}

				return nil
			},
			retry.Attempts(30),
			retry.Delay(1*time.Second),
			retry.DelayType(retry.FixedDelay),
			retry.OnRetry(func(n uint, err error) {
				t.Logf("Retry %d: Waiting for WebSocket updates: Candle=%v, Ticker=%v, OrderBook=%v",
					n+1, receivedCandle, receivedTicker, receivedOrderBook)
			}),
		)

		require.NoError(t, err, "timeout waiting for WebSocket updates")

		// Validate we received all updates
		require.True(t, receivedCandle, "did not receive candle update")
		require.True(t, receivedTicker, "did not receive ticker update")
		require.True(t, receivedOrderBook, "did not receive order book update")
	})

	// Test reconnection
	t.Run("Reconnection", func(t *testing.T) {
		// Force close and reconnect
		err := connector.Close()
		require.NoError(t, err, "failed to close connection")

		err = connector.Connect(ctx)
		require.NoError(t, err, "failed to reconnect")

		// Verify we can still get data
		ticker, err := connector.GetTicker(ctx, "BTCUSDT")
		require.NoError(t, err, "failed to get ticker after reconnect")
		require.Equal(t, "BTCUSDT", ticker.Symbol)
	})
}
