package bybit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
	"github.com/veiloq/exchange-connector/pkg/websocket"
)

func TestConnector_SubscribeOrderBook(t *testing.T) {
	tests := []struct {
		name         string
		symbols      []string
		isConnected  bool
		mockResponse string
		setupMock    func(*websocket.MockConnector)
		expectedErr  error
		expectedBook interfaces.OrderBook
	}{
		{
			name:        "success",
			symbols:     []string{"BTCUSDT"},
			isConnected: true,
			mockResponse: `{
				"topic": "orderbook.BTCUSDT",
				"type": "snapshot",
				"data": {
					"s": "BTCUSDT",
					"b": [
						{"p": "50000", "s": "1.5"},
						{"p": "49900", "s": "2.5"}
					],
					"a": [
						{"p": "50100", "s": "1.2"},
						{"p": "50200", "s": "2.2"}
					]
				},
				"ts": 1616819632000
			}`,
			setupMock: func(mock *websocket.MockConnector) {
				// No errors to set
			},
			expectedErr: nil,
			expectedBook: interfaces.OrderBook{
				Symbol: "BTCUSDT",
				Bids: []interfaces.OrderBookLevel{
					{Price: 50000, Quantity: 1.5},
					{Price: 49900, Quantity: 2.5},
				},
				Asks: []interfaces.OrderBookLevel{
					{Price: 50100, Quantity: 1.2},
					{Price: 50200, Quantity: 2.2},
				},
			},
		},
		{
			name:        "not connected",
			symbols:     []string{"BTCUSDT"},
			isConnected: false,
			setupMock:   func(mock *websocket.MockConnector) {},
			expectedErr: interfaces.ErrNotConnected,
		},
		{
			name:        "empty symbol list",
			symbols:     []string{},
			isConnected: true,
			setupMock:   func(mock *websocket.MockConnector) {},
			expectedErr: interfaces.ErrInvalidSymbol,
		},
		{
			name:        "invalid symbol",
			symbols:     []string{"BTC"},
			isConnected: true,
			setupMock:   func(mock *websocket.MockConnector) {},
			expectedErr: interfaces.ErrInvalidSymbol,
		},
		{
			name:        "subscription error",
			symbols:     []string{"BTCUSDT"},
			isConnected: true,
			setupMock: func(mock *websocket.MockConnector) {
				mock.SetSubscribeError(interfaces.ErrSubscriptionFailed)
			},
			expectedErr: interfaces.ErrSubscriptionFailed,
		},
		{
			name:         "empty response",
			symbols:      []string{"BTCUSDT"},
			isConnected:  true,
			mockResponse: "",
			setupMock:    func(mock *websocket.MockConnector) {},
			expectedErr:  nil,
		},
		{
			name:        "malformed JSON",
			symbols:     []string{"BTCUSDT"},
			isConnected: true,
			mockResponse: `{
				"topic": "orderbook.BTCUSDT",
				"type": "snapshot",
				"data": {
					"s": "BTCUSDT",
					"b": [
						{"p": "invalid", "s": "1.5"},
						{"p": "49900", "s": "2.5"}
					],
					"a": [
						{"p": "50100", "s": "1.2"},
						{"p": "50200", "s": "2.2"}
					]
				},
				"ts": 1616819632000
			}`,
			setupMock:   func(mock *websocket.MockConnector) {},
			expectedErr: nil, // We handle parse errors internally
			expectedBook: interfaces.OrderBook{
				Symbol: "BTCUSDT",
				Bids: []interfaces.OrderBookLevel{
					{Price: 49900, Quantity: 2.5},
				},
				Asks: []interfaces.OrderBookLevel{
					{Price: 50100, Quantity: 1.2},
					{Price: 50200, Quantity: 2.2},
				},
			},
		},
		{
			name:        "zero quantity values",
			symbols:     []string{"BTCUSDT"},
			isConnected: true,
			mockResponse: `{
				"topic": "orderbook.BTCUSDT",
				"type": "snapshot",
				"data": {
					"s": "BTCUSDT",
					"b": [
						{"p": "50000", "s": "0"},
						{"p": "49900", "s": "2.5"}
					],
					"a": [
						{"p": "50100", "s": "1.2"},
						{"p": "50200", "s": "0"}
					]
				},
				"ts": 1616819632000
			}`,
			setupMock:   func(mock *websocket.MockConnector) {},
			expectedErr: nil,
			expectedBook: interfaces.OrderBook{
				Symbol: "BTCUSDT",
				Bids: []interfaces.OrderBookLevel{
					{Price: 49900, Quantity: 2.5},
				},
				Asks: []interfaces.OrderBookLevel{
					{Price: 50100, Quantity: 1.2},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up mock websocket connector
			mockWS := websocket.NewMockConnector()
			tc.setupMock(mockWS)

			// Create connector with mock websocket
			connector := NewConnector(nil)
			connector.ws = mockWS
			connector.connected = tc.isConnected

			// Create context and channel to receive results
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			received := make(chan interfaces.OrderBook, 1)
			handler := func(ob interfaces.OrderBook) {
				received <- ob
			}

			// Call method under test
			err := connector.SubscribeOrderBook(ctx, tc.symbols, handler)

			// Check errors
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
				return
			} else {
				require.NoError(t, err)
			}

			// If we expect a successful subscription, validate the result
			if tc.mockResponse != "" && tc.isConnected {
				// Simulate an order book message
				topic := "orderbook." + tc.symbols[0]
				mockWS.SimulateMessage(topic, []byte(tc.mockResponse))

				// Wait for the message to be processed
				var result interfaces.OrderBook
				select {
				case result = <-received:
					// Validate the parsed orderbook matches expectations
					assert.Equal(t, tc.expectedBook.Symbol, result.Symbol)

					// Check bids
					require.Len(t, result.Bids, len(tc.expectedBook.Bids))
					for i, bid := range result.Bids {
						assert.Equal(t, tc.expectedBook.Bids[i].Price, bid.Price)
						assert.Equal(t, tc.expectedBook.Bids[i].Quantity, bid.Quantity)
					}

					// Check asks
					require.Len(t, result.Asks, len(tc.expectedBook.Asks))
					for i, ask := range result.Asks {
						assert.Equal(t, tc.expectedBook.Asks[i].Price, ask.Price)
						assert.Equal(t, tc.expectedBook.Asks[i].Quantity, ask.Quantity)
					}
				case <-ctx.Done():
					t.Fatal("timed out waiting for order book message")
				}
			}

			// Verify that the correct topic was subscribed to
			if tc.isConnected && err == nil && len(tc.symbols) > 0 {
				topic := "orderbook." + tc.symbols[0]
				assert.Equal(t, 1, mockWS.GetSubscribeCalls(topic),
					"expected topic %s to be subscribed to", topic)
			}
		})
	}
}

func TestConnector_parseOrderBookMessage(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		symbol        string
		expectError   bool
		expectedBook  *interfaces.OrderBook
		errorContains string
	}{
		{
			name:   "valid message",
			symbol: "BTCUSDT",
			message: `{
				"topic": "orderbook.BTCUSDT",
				"type": "snapshot",
				"data": {
					"s": "BTCUSDT",
					"b": [
						{"p": "50000", "s": "1.5"},
						{"p": "49900", "s": "2.5"}
					],
					"a": [
						{"p": "50100", "s": "1.2"},
						{"p": "50200", "s": "2.2"}
					]
				},
				"ts": 1616819632000
			}`,
			expectError: false,
			expectedBook: &interfaces.OrderBook{
				Symbol: "BTCUSDT",
				Bids: []interfaces.OrderBookLevel{
					{Price: 50000, Quantity: 1.5},
					{Price: 49900, Quantity: 2.5},
				},
				Asks: []interfaces.OrderBookLevel{
					{Price: 50100, Quantity: 1.2},
					{Price: 50200, Quantity: 2.2},
				},
			},
		},
		{
			name:          "invalid JSON",
			symbol:        "BTCUSDT",
			message:       `{"invalid json`,
			expectError:   true,
			errorContains: "failed to unmarshal",
		},
		{
			name:          "empty message",
			symbol:        "BTCUSDT",
			message:       ``,
			expectError:   true,
			errorContains: "empty message",
		},
		{
			name:   "missing type",
			symbol: "BTCUSDT",
			message: `{
				"topic": "orderbook.BTCUSDT",
				"data": {
					"s": "BTCUSDT",
					"b": [],
					"a": []
				},
				"ts": 1616819632000
			}`,
			expectError:   true,
			errorContains: "missing message type",
		},
		{
			name:   "symbol mismatch",
			symbol: "BTCUSDT",
			message: `{
				"topic": "orderbook.ETHUSDT",
				"type": "snapshot",
				"data": {
					"s": "ETHUSDT",
					"b": [],
					"a": []
				},
				"ts": 1616819632000
			}`,
			expectError:   true,
			errorContains: "topic mismatch",
		},
		{
			name:   "data symbol mismatch",
			symbol: "BTCUSDT",
			message: `{
				"topic": "orderbook.BTCUSDT",
				"type": "snapshot",
				"data": {
					"s": "ETHUSDT",
					"b": [],
					"a": []
				},
				"ts": 1616819632000
			}`,
			expectError:   true,
			errorContains: "symbol mismatch",
		},
		{
			name:   "invalid bid price",
			symbol: "BTCUSDT",
			message: `{
				"topic": "orderbook.BTCUSDT",
				"type": "snapshot",
				"data": {
					"s": "BTCUSDT",
					"b": [
						{"p": "invalid", "s": "1.5"},
						{"p": "49900", "s": "2.5"}
					],
					"a": []
				},
				"ts": 1616819632000
			}`,
			expectError: false,
			expectedBook: &interfaces.OrderBook{
				Symbol: "BTCUSDT",
				Bids: []interfaces.OrderBookLevel{
					{Price: 49900, Quantity: 2.5},
				},
				Asks: []interfaces.OrderBookLevel{},
			},
		},
		{
			name:   "invalid ask quantity",
			symbol: "BTCUSDT",
			message: `{
				"topic": "orderbook.BTCUSDT",
				"type": "snapshot",
				"data": {
					"s": "BTCUSDT",
					"b": [
						{"p": "50000", "s": "1.5"}
					],
					"a": [
						{"p": "50100", "s": "invalid"},
						{"p": "50200", "s": "2.2"}
					]
				},
				"ts": 1616819632000
			}`,
			expectError: false,
			expectedBook: &interfaces.OrderBook{
				Symbol: "BTCUSDT",
				Bids: []interfaces.OrderBookLevel{
					{Price: 50000, Quantity: 1.5},
				},
				Asks: []interfaces.OrderBookLevel{
					{Price: 50200, Quantity: 2.2},
				},
			},
		},
		{
			name:   "empty order book",
			symbol: "BTCUSDT",
			message: `{
				"topic": "orderbook.BTCUSDT",
				"type": "snapshot",
				"data": {
					"s": "BTCUSDT",
					"b": [],
					"a": []
				},
				"ts": 1616819632000
			}`,
			expectError: false,
			expectedBook: &interfaces.OrderBook{
				Symbol: "BTCUSDT",
				Bids:   []interfaces.OrderBookLevel{},
				Asks:   []interfaces.OrderBookLevel{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			connector := NewConnector(nil)
			result, err := connector.parseOrderBookMessage([]byte(tc.message), tc.symbol)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check symbol
			assert.Equal(t, tc.expectedBook.Symbol, result.Symbol)

			// Check bids
			assert.Len(t, result.Bids, len(tc.expectedBook.Bids))
			for i, bid := range result.Bids {
				assert.Equal(t, tc.expectedBook.Bids[i].Price, bid.Price)
				assert.Equal(t, tc.expectedBook.Bids[i].Quantity, bid.Quantity)
			}

			// Check asks
			assert.Len(t, result.Asks, len(tc.expectedBook.Asks))
			for i, ask := range result.Asks {
				assert.Equal(t, tc.expectedBook.Asks[i].Price, ask.Price)
				assert.Equal(t, tc.expectedBook.Asks[i].Quantity, ask.Quantity)
			}
		})
	}
}
