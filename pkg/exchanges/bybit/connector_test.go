package bybit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/veiloq/exchange-connector/pkg/exchanges/interfaces"
	"github.com/veiloq/exchange-connector/pkg/websocket"
)

// --- Mock HTTP RoundTripper ---

// mockRoundTripper implements http.RoundTripper for mocking HTTP requests.
type mockRoundTripper struct {
	Response *http.Response
	Err      error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	// Ensure the response body can be read multiple times if needed
	if m.Response != nil && m.Response.Body != nil {
		bodyBytes, _ := io.ReadAll(m.Response.Body)
		m.Response.Body.Close()                                    // Close original body
		m.Response.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Replace with new reader
	}
	return m.Response, nil
}

// --- Existing Mock WSConnector ---
// (Keep the existing mockWSConnector struct and methods as they are)
type mockWSConnector struct {
	websocket.WSConnector // Embed the interface
	connectCalled         bool
	closeCalled           bool
	subscribeCalled       bool
	unsubscribeCalled     bool
	sendCalled            bool
	isConnectedResult     bool
	config                websocket.Config
	lastSubscribedTopic   string
	lastUnsubscribedTopic string
	lastSentMessage       interface{}
	connectError          error
	closeError            error
	subscribeError        error
	unsubscribeError      error
	sendError             error
}

func (m *mockWSConnector) Connect(ctx context.Context) error {
	m.connectCalled = true
	return m.connectError
}

func (m *mockWSConnector) Close() error {
	m.closeCalled = true
	m.isConnectedResult = false // Simulate disconnection on close
	return m.closeError
}

func (m *mockWSConnector) Subscribe(topic string, handler websocket.MessageHandler) error {
	m.subscribeCalled = true
	m.lastSubscribedTopic = topic
	return m.subscribeError
}

func (m *mockWSConnector) Unsubscribe(topic string) error {
	m.unsubscribeCalled = true
	m.lastUnsubscribedTopic = topic
	return m.unsubscribeError
}

func (m *mockWSConnector) Send(message interface{}) error {
	m.sendCalled = true
	m.lastSentMessage = message
	return m.sendError
}

func (m *mockWSConnector) IsConnected() bool {
	return m.isConnectedResult
}

func (m *mockWSConnector) GetConfig() websocket.Config {
	return m.config
}

// --- Existing Test Helper Functions ---
// (Keep TestIsValidInterval, TestValidateTimeRange, TestConvertIntervalToBybit)

func TestIsValidInterval(t *testing.T) {
	c := NewConnector(nil) // Need an instance to call the method

	validIntervals := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "12h", "1d", "1w", "1M"}
	for _, interval := range validIntervals {
		assert.True(t, c.isValidInterval(interval), "Expected interval %s to be valid", interval)
	}

	invalidIntervals := []string{"", "1s", "1y", "2m", "1H"}
	for _, interval := range invalidIntervals {
		assert.False(t, c.isValidInterval(interval), "Expected interval %s to be invalid", interval)
	}
}

func TestValidateTimeRange(t *testing.T) {
	c := NewConnector(nil)
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		expectErr bool
		errType   error // Expected error type if expectErr is true
	}{
		{"Valid Range", yesterday, now, false, nil},
		{"End Before Start", now, yesterday, true, interfaces.ErrInvalidTimeRange},
		{"Zero Start Time", time.Time{}, now, true, interfaces.ErrInvalidTimeRange},
		{"Zero End Time", yesterday, time.Time{}, true, interfaces.ErrInvalidTimeRange},
		{"Zero Both Times", time.Time{}, time.Time{}, true, interfaces.ErrInvalidTimeRange},
		{"Equal Times", now, now, false, nil}, // Equal times are technically valid
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.validateTimeRange(tt.startTime, tt.endTime)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertIntervalToBybit(t *testing.T) {
	tests := []struct {
		interval string
		expected string
	}{
		{"1m", "1"},
		{"5m", "5"},
		{"1h", "60"},
		{"4h", "240"},
		{"1d", "D"},
		{"1w", "W"},
		{"1M", "M"},
		{"unknown", "unknown"}, // Should return as-is
		{"", ""},               // Should return as-is
	}

	for _, tt := range tests {
		t.Run(tt.interval, func(t *testing.T) {
			assert.Equal(t, tt.expected, convertIntervalToBybit(tt.interval))
		})
	}
}

// --- Existing Test NewConnector ---
// (Keep TestNewConnector_DefaultOptions, TestNewConnector_WithOptions, TestNewConnector_InvalidLogLevel)

func TestNewConnector_DefaultOptions(t *testing.T) {
	c := NewConnector(nil)
	require.NotNil(t, c)
	require.NotNil(t, c.options)
	require.NotNil(t, c.ws)
	require.NotNil(t, c.logger)
	assert.Equal(t, "wss://stream.bybit.com/v5/public/spot", c.ws.GetConfig().URL)
	assert.Equal(t, 20*time.Second, c.ws.GetConfig().HeartbeatInterval)
	assert.Equal(t, 5*time.Second, c.ws.GetConfig().ReconnectInterval)
	assert.Equal(t, 3, c.ws.GetConfig().MaxRetries)
	assert.False(t, c.connected)
	assert.NotNil(t, c.subscriptions)
	assert.Empty(t, c.subscriptions)
	assert.NotNil(t, c.instruments)
	assert.Empty(t, c.instruments)
	// Check default log level (assuming INFO is default in logging pkg)
	// This might require exposing a GetLevel method in the logger interface or checking internal state
}

func TestNewConnector_WithOptions(t *testing.T) {
	customWsURL := "wss://custom.bybit.stream/ws"
	customRestURL := "https://custom.bybit.api"
	customHeartbeat := 30 * time.Second
	customReconnect := 10 * time.Second

	options := &interfaces.ExchangeOptions{
		BaseURL:             customWsURL, // BaseURL used for WebSocket
		RestURL:             customRestURL,
		WSHeartbeatInterval: customHeartbeat,
		WSReconnectInterval: customReconnect,
		LogLevel:            "debug",
	}

	c := NewConnector(options)
	require.NotNil(t, c)
	assert.Equal(t, customWsURL, c.ws.GetConfig().URL)
	assert.Equal(t, customRestURL, c.options.RestURL) // Check RestURL is stored
	assert.Equal(t, customHeartbeat, c.ws.GetConfig().HeartbeatInterval)
	assert.Equal(t, customReconnect, c.ws.GetConfig().ReconnectInterval)
	assert.Equal(t, 3, c.ws.GetConfig().MaxRetries) // MaxRetries not overridden in options
}

// --- Test Connect ---

// TestConnect_Success tests the successful connection flow.
// It assumes fetchInstrumentsInfo succeeds (by pre-populating) and mocks ws.Connect.
func TestConnect_Success(t *testing.T) {
	mockWs := &mockWSConnector{
		config: websocket.Config{URL: "wss://test.bybit.com"},
	}
	c := NewConnector(nil)
	c.ws = mockWs
	// Pre-populate instruments to simulate a successful fetchInstrumentsInfo call
	// which happens before ws.Connect within the actual Connect method.
	c.instruments = map[string]BybitInstrumentInfo{"BTCUSDT": {Symbol: "BTCUSDT", Status: "Trading"}}

	// Mock the HTTP client to return a successful instruments response
	// This prevents the real fetchInstrumentsInfo from making an actual HTTP call
	// and ensures the test focuses only on the ws.Connect part after a successful fetch.
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: &mockRoundTripper{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
					"retCode": 0,
					"retMsg": "OK",
					"result": {
						"category": "spot",
						"list": [
							{"symbol": "BTCUSDT", "status": "Trading"}
						]
					},
					"time": 1672531200000
				}`)),
				Header: make(http.Header),
			},
		},
	}
	// Restore original client after test
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})

	ctx := context.Background()
	err := c.Connect(ctx)

	require.NoError(t, err)
	assert.True(t, mockWs.connectCalled, "ws.Connect should have been called")
	assert.True(t, c.connected, "connector should be marked as connected")
}

// TestConnect_FetchInstrumentsFail tests the scenario where fetching instruments fails.
func TestConnect_FetchInstrumentsFail(t *testing.T) {
	// --- Test Case 1: HTTP Request Error ---
	t.Run("HTTPRequestError", func(t *testing.T) {
		expectedHTTPErr := errors.New("network error")
		mockWs := &mockWSConnector{} // WS should not be called
		c := NewConnector(nil)
		c.ws = mockWs

		// Mock the HTTP client to return an error
		originalClient := http.DefaultClient
		http.DefaultClient = &http.Client{
			Transport: &mockRoundTripper{Err: expectedHTTPErr},
		}
		t.Cleanup(func() { http.DefaultClient = originalClient }) // Restore original client

		ctx := context.Background()
		err := c.Connect(ctx)

		require.Error(t, err)
		assert.False(t, mockWs.connectCalled, "ws.Connect should NOT have been called")
		assert.False(t, c.connected, "connector should NOT be marked as connected")
		assert.Contains(t, err.Error(), "failed to fetch Bybit instruments info")
		// Check that the underlying HTTP error is wrapped (twice, once in doBybitGet, once in fetchInstrumentsInfo)
		assert.ErrorContains(t, err, "failed to fetch Bybit instruments info: failed to execute Bybit API request: Get \"https://api.bybit.com/v5/market/instruments-info?category=spot\": network error")
	})

	// --- Test Case 2: Non-200 Status Code ---
	t.Run("Non200StatusCode", func(t *testing.T) {
		mockWs := &mockWSConnector{} // WS should not be called
		c := NewConnector(nil)
		c.ws = mockWs

		// Mock the HTTP client to return a 500 status
		originalClient := http.DefaultClient
		http.DefaultClient = &http.Client{
			Transport: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBufferString("internal server error")),
					Header:     make(http.Header),
				},
			},
		}
		t.Cleanup(func() { http.DefaultClient = originalClient }) // Restore original client

		ctx := context.Background()
		err := c.Connect(ctx)

		require.Error(t, err)
		assert.False(t, mockWs.connectCalled, "ws.Connect should NOT have been called")
		assert.False(t, c.connected, "connector should NOT be marked as connected")
		assert.Contains(t, err.Error(), "failed to fetch Bybit instruments info")
		assert.Contains(t, err.Error(), "Bybit API error: status 500") // Check for status code error
	})

	// --- Test Case 3: API Error Code (retCode != 0) ---
	t.Run("APIErrorCode", func(t *testing.T) {
		mockWs := &mockWSConnector{} // WS should not be called
		c := NewConnector(nil)
		c.ws = mockWs

		// Mock the HTTP client to return success status but error retCode
		originalClient := http.DefaultClient
		http.DefaultClient = &http.Client{
			Transport: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusOK, // HTTP is fine
					Body: io.NopCloser(bytes.NewBufferString(`{
						"retCode": 10001,
						"retMsg": "Parameter error.",
						"result": {},
						"time": 1672531200000
					}`)),
					Header: make(http.Header),
				},
			},
		}
		t.Cleanup(func() { http.DefaultClient = originalClient }) // Restore original client

		ctx := context.Background()
		err := c.Connect(ctx)

		require.Error(t, err)
		assert.False(t, mockWs.connectCalled, "ws.Connect should NOT have been called")
		assert.False(t, c.connected, "connector should NOT be marked as connected")
		assert.Contains(t, err.Error(), "failed to parse Bybit instruments info JSON") // Error comes from parseBybitResponse
		assert.Contains(t, err.Error(), "Bybit API error: code 10001")                 // Check for retCode error
	})

	// --- Test Case 4: Malformed JSON Response ---
	t.Run("MalformedJSON", func(t *testing.T) {
		mockWs := &mockWSConnector{} // WS should not be called
		c := NewConnector(nil)
		c.ws = mockWs

		// Mock the HTTP client to return malformed JSON
		originalClient := http.DefaultClient
		http.DefaultClient = &http.Client{
			Transport: &mockRoundTripper{
				Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"retCode": 0, "result": malformed`)), // Invalid JSON
					Header:     make(http.Header),
				},
			},
		}
		t.Cleanup(func() { http.DefaultClient = originalClient }) // Restore original client

		ctx := context.Background()
		err := c.Connect(ctx)

		require.Error(t, err)
		assert.False(t, mockWs.connectCalled, "ws.Connect should NOT have been called")
		assert.False(t, c.connected, "connector should NOT be marked as connected")
		assert.Contains(t, err.Error(), "failed to parse Bybit instruments info JSON")       // Error comes from parseBybitResponse
		assert.ErrorContains(t, err, "invalid character 'm' looking for beginning of value") // Check for JSON parsing error
	})
}

// TestConnect_WSConnectFail tests failure during the WebSocket connection phase.
func TestConnect_WSConnectFail(t *testing.T) {
	expectedErr := fmt.Errorf("websocket connection failed")
	mockWs := &mockWSConnector{
		config:       websocket.Config{URL: "wss://test.bybit.com"},
		connectError: expectedErr,
	}
	c := NewConnector(nil)
	c.ws = mockWs
	// Pre-populate instruments to simulate successful fetch, so the error must come from ws.Connect
	c.instruments = map[string]BybitInstrumentInfo{"BTCUSDT": {Symbol: "BTCUSDT", Status: "Trading"}}

	// Mock the HTTP client to return a successful instruments response
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: &mockRoundTripper{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
					"retCode": 0,
					"retMsg": "OK",
					"result": {
						"category": "spot",
						"list": [
							{"symbol": "BTCUSDT", "status": "Trading"}
						]
					},
					"time": 1672531200000
				}`)),
				Header: make(http.Header),
			},
		},
	}
	t.Cleanup(func() { http.DefaultClient = originalClient }) // Restore original client

	ctx := context.Background()
	err := c.Connect(ctx)

	require.Error(t, err)
	assert.True(t, mockWs.connectCalled, "ws.Connect should have been called")
	assert.False(t, c.connected, "connector should not be marked as connected")
	// Check if the error is wrapped correctly
	assert.Contains(t, err.Error(), "failed to connect to Bybit WebSocket API")
	assert.ErrorIs(t, err, expectedErr) // Check underlying error
}

// --- Test Close ---
// (Keep TestClose_Success, TestClose_NotConnected, TestClose_WSCloseFail)

func TestClose_Success(t *testing.T) {
	mockWs := &mockWSConnector{
		isConnectedResult: true, // Start connected
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = true                              // Manually set connected state for the test
	c.subscriptions["kline.1.BTCUSDT"] = struct{}{} // Add a dummy subscription

	err := c.Close()

	require.NoError(t, err)
	assert.True(t, mockWs.closeCalled, "ws.Close should have been called")
	assert.False(t, c.connected, "connector should be marked as not connected")
	assert.Empty(t, c.subscriptions, "subscriptions map should be cleared")
}

func TestClose_NotConnected(t *testing.T) {
	mockWs := &mockWSConnector{
		isConnectedResult: false, // Start not connected
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = false // Ensure state is not connected

	err := c.Close()

	require.Error(t, err)
	assert.ErrorIs(t, err, interfaces.ErrNotConnected)
	assert.False(t, mockWs.closeCalled, "ws.Close should not have been called")
}

func TestClose_WSCloseFail(t *testing.T) {
	expectedErr := fmt.Errorf("websocket close failed")
	mockWs := &mockWSConnector{
		isConnectedResult: true, // Start connected
		closeError:        expectedErr,
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = true // Manually set connected state

	err := c.Close()

	require.Error(t, err)
	assert.True(t, mockWs.closeCalled, "ws.Close should have been called")
	assert.False(t, c.connected, "connector should be marked as not connected") // State changes even if ws.Close fails
	assert.Empty(t, c.subscriptions, "subscriptions map should be cleared")     // State changes even if ws.Close fails
	assert.Contains(t, err.Error(), "error closing Bybit WebSocket connection")
	assert.ErrorIs(t, err, expectedErr)
}

// --- Test SubscribeCandles ---

func TestSubscribeCandles_Success(t *testing.T) {
	mockWs := &mockWSConnector{
		isConnectedResult: true, // Simulate connected state
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = true // Manually set connected state

	symbol := "BTCUSDT"
	interval := "1m"
	// Use the conversion function to get the expected Bybit interval format
	expectedBybitInterval := convertIntervalToBybit(interval)
	expectedTopic := fmt.Sprintf("kline.%s.%s", expectedBybitInterval, symbol)

	err := c.SubscribeCandles(symbol, interval)

	require.NoError(t, err)
	assert.True(t, mockWs.subscribeCalled, "ws.Subscribe should have been called")
	assert.Equal(t, expectedTopic, mockWs.lastSubscribedTopic, "Should subscribe to the correct topic")
	_, exists := c.subscriptions[expectedTopic]
	assert.True(t, exists, "Subscription should be recorded")
}

func TestSubscribeCandles_NotConnected(t *testing.T) {
	mockWs := &mockWSConnector{
		isConnectedResult: false, // Simulate disconnected state
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = false // Manually set disconnected state

	symbol := "BTCUSDT"
	interval := "1m"

	err := c.SubscribeCandles(symbol, interval)

	require.Error(t, err)
	assert.ErrorIs(t, err, interfaces.ErrNotConnected)
	assert.False(t, mockWs.subscribeCalled, "ws.Subscribe should not have been called")
	assert.Empty(t, c.subscriptions, "Subscriptions map should be empty")
}

func TestSubscribeCandles_InvalidInterval(t *testing.T) {
	mockWs := &mockWSConnector{
		isConnectedResult: true, // Simulate connected state
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = true // Manually set connected state

	symbol := "BTCUSDT"
	interval := "invalid-interval" // Use an interval guaranteed to be invalid

	err := c.SubscribeCandles(symbol, interval)

	require.Error(t, err)
	assert.ErrorIs(t, err, interfaces.ErrInvalidInterval)
	assert.False(t, mockWs.subscribeCalled, "ws.Subscribe should not have been called")
	assert.Empty(t, c.subscriptions, "Subscriptions map should be empty")
}

func TestSubscribeCandles_WSSubscribeFail(t *testing.T) {
	expectedErr := fmt.Errorf("websocket subscribe failed")
	mockWs := &mockWSConnector{
		isConnectedResult: true, // Simulate connected state
		subscribeError:    expectedErr,
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = true // Manually set connected state

	symbol := "BTCUSDT"
	interval := "1m"
	expectedBybitInterval := convertIntervalToBybit(interval)
	expectedTopic := fmt.Sprintf("kline.%s.%s", expectedBybitInterval, symbol)

	err := c.SubscribeCandles(symbol, interval)

	require.Error(t, err)
	assert.True(t, mockWs.subscribeCalled, "ws.Subscribe should have been called")
	assert.Equal(t, expectedTopic, mockWs.lastSubscribedTopic, "Should attempt to subscribe to the correct topic")
	assert.Empty(t, c.subscriptions, "Subscription should not be recorded on failure")
	assert.ErrorIs(t, err, expectedErr) // Check underlying error
}

// --- Test UnsubscribeCandles ---

func TestUnsubscribeCandles_Success(t *testing.T) {
	mockWs := &mockWSConnector{
		isConnectedResult: true, // Simulate connected state
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = true // Manually set connected state

	symbol := "BTCUSDT"
	interval := "1m"
	expectedBybitInterval := convertIntervalToBybit(interval)
	topic := fmt.Sprintf("kline.%s.%s", expectedBybitInterval, symbol)
	c.subscriptions[topic] = struct{}{} // Simulate existing subscription

	err := c.UnsubscribeCandles(symbol, interval)

	require.NoError(t, err)
	assert.True(t, mockWs.unsubscribeCalled, "ws.Unsubscribe should have been called")
	assert.Equal(t, topic, mockWs.lastUnsubscribedTopic, "Should unsubscribe from the correct topic")
	_, exists := c.subscriptions[topic]
	assert.False(t, exists, "Subscription should be removed")
}

func TestUnsubscribeCandles_NotConnected(t *testing.T) {
	mockWs := &mockWSConnector{
		isConnectedResult: false, // Simulate disconnected state
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = false // Manually set disconnected state

	symbol := "BTCUSDT"
	interval := "1m"
	expectedBybitInterval := convertIntervalToBybit(interval)
	topic := fmt.Sprintf("kline.%s.%s", expectedBybitInterval, symbol)
	c.subscriptions[topic] = struct{}{} // Simulate existing subscription

	err := c.UnsubscribeCandles(symbol, interval)

	require.Error(t, err)
	assert.ErrorIs(t, err, interfaces.ErrNotConnected)
	assert.False(t, mockWs.unsubscribeCalled, "ws.Unsubscribe should not have been called")
	_, exists := c.subscriptions[topic]
	assert.True(t, exists, "Subscription should not be removed")
}

func TestUnsubscribeCandles_NotSubscribed(t *testing.T) {
	mockWs := &mockWSConnector{
		isConnectedResult: true, // Simulate connected state
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = true // Manually set connected state

	symbol := "BTCUSDT"
	interval := "1m"

	// No subscription exists for this symbol/interval

	err := c.UnsubscribeCandles(symbol, interval)

	require.Error(t, err)
	// Check for a specific error indicating not subscribed, if the function provides one.
	// For now, just check that unsubscribe wasn't called on the websocket.
	assert.False(t, mockWs.unsubscribeCalled, "ws.Unsubscribe should not have been called")
}

func TestUnsubscribeCandles_WSUnsubscribeFail(t *testing.T) {
	expectedErr := fmt.Errorf("websocket unsubscribe failed")
	mockWs := &mockWSConnector{
		isConnectedResult: true, // Simulate connected state
		unsubscribeError:  expectedErr,
	}
	c := NewConnector(nil)
	c.ws = mockWs
	c.connected = true // Manually set connected state

	symbol := "BTCUSDT"
	interval := "1m"
	expectedBybitInterval := convertIntervalToBybit(interval)
	topic := fmt.Sprintf("kline.%s.%s", expectedBybitInterval, symbol)
	c.subscriptions[topic] = struct{}{} // Simulate existing subscription

	err := c.UnsubscribeCandles(symbol, interval)

	require.Error(t, err)
	assert.True(t, mockWs.unsubscribeCalled, "ws.Unsubscribe should have been called")
	assert.Equal(t, topic, mockWs.lastUnsubscribedTopic, "Should attempt to unsubscribe from the correct topic")
	assert.Contains(t, c.subscriptions, topic, "Subscription should not be removed on failure") // Subscription remains
	assert.ErrorIs(t, err, expectedErr)
}

// --- Test GetCandles ---

func TestGetCandles_Success(t *testing.T) {
	c := NewConnector(nil)
	c.connected = true // Simulate connected state
	// Pre-populate instruments to pass isValidSymbol check
	c.instruments = map[string]BybitInstrumentInfo{"BTCUSDT": {Symbol: "BTCUSDT", Status: "Trading"}}

	// Mock HTTP response for Kline API
	mockResponseBody := `{
		"retCode": 0,
		"retMsg": "OK",
		"result": {
			"category": "spot",
			"symbol": "BTCUSDT",
			"list": [
				[
					"1672531200000", 
					"16500.5",     
					"16550.0",     
					"16480.0",     
					"16520.5",     
					"100.5",       
					"1650000.75"   
				],
				[
					"1672531260000", 
					"16520.5",
					"16530.0",
					"16510.0",
					"16515.0",
					"50.2",
					"829000.50"
				]
			]
		},
		"retExtInfo": {},
		"time": 1672531320000 
	}`

	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: &mockRoundTripper{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(mockResponseBody)),
				Header:     make(http.Header),
			},
		},
	}
	t.Cleanup(func() { http.DefaultClient = originalClient }) // Restore original client

	// Define expected candles based on mock response
	expectedCandles := []interfaces.Candle{
		{
			Symbol:    "BTCUSDT",
			StartTime: time.UnixMilli(1672531200000),
			Open:      16500.5,
			High:      16550.0,
			Low:       16480.0,
			Close:     16520.5,
			Volume:    100.5,
		},
		{
			Symbol:    "BTCUSDT",
			StartTime: time.UnixMilli(1672531260000),
			Open:      16520.5,
			High:      16530.0,
			Low:       16510.0,
			Close:     16515.0,
			Volume:    50.2,
		},
	}

	// Prepare request
	req := interfaces.CandleRequest{
		Symbol:    "BTCUSDT",
		Interval:  "1m", // Corresponds to "1" in Bybit API
		StartTime: time.UnixMilli(1672531200000),
		EndTime:   time.UnixMilli(1672531260000),
		Limit:     2,
	}
	ctx := context.Background()

	// Call GetCandles
	candles, err := c.GetCandles(ctx, req)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, candles)
	assert.Equal(t, expectedCandles, candles)
}

func TestGetCandles_InvalidSymbol(t *testing.T) {
	c := NewConnector(nil)
	c.connected = true // Simulate connected state
	// Ensure instruments map does NOT contain the invalid symbol
	c.instruments = map[string]BybitInstrumentInfo{"BTCUSDT": {Symbol: "BTCUSDT", Status: "Trading"}} // Only valid symbols

	// Prepare request with an invalid symbol
	req := interfaces.CandleRequest{
		Symbol:    "INVALID_SYMBOL", // This symbol is not in c.instruments
		Interval:  "1m",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Limit:     10,
	}
	ctx := context.Background()

	// Call GetCandles
	candles, err := c.GetCandles(ctx, req)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, candles)
	assert.ErrorIs(t, err, interfaces.ErrInvalidSymbol)

	// --- Test with empty instruments map (fallback validation) ---
	t.Run("EmptyInstrumentsMap", func(t *testing.T) {
		cEmpty := NewConnector(nil)
		cEmpty.connected = true
		cEmpty.instruments = map[string]BybitInstrumentInfo{} // Empty map

		reqInvalidShort := interfaces.CandleRequest{
			Symbol:    "INV", // Too short for fallback validation
			Interval:  "1m",
			StartTime: time.Now().Add(-1 * time.Hour),
			EndTime:   time.Now(),
			Limit:     10,
		}

		_, errShort := cEmpty.GetCandles(ctx, reqInvalidShort)
		require.Error(t, errShort)
		assert.ErrorIs(t, errShort, interfaces.ErrInvalidSymbol)

		reqInvalidEmpty := interfaces.CandleRequest{
			Symbol:    "", // Empty symbol
			Interval:  "1m",
			StartTime: time.Now().Add(-1 * time.Hour),
			EndTime:   time.Now(),
			Limit:     10,
		}
		_, errEmpty := cEmpty.GetCandles(ctx, reqInvalidEmpty)
		require.Error(t, errEmpty)
		assert.ErrorIs(t, errEmpty, interfaces.ErrInvalidSymbol)
	})
}

func TestGetCandles_InvalidInterval(t *testing.T) {
	c := NewConnector(nil)
	c.connected = true // Simulate connected state
	// Pre-populate instruments to pass isValidSymbol check
	c.instruments = map[string]BybitInstrumentInfo{"BTCUSDT": {Symbol: "BTCUSDT", Status: "Trading"}}

	// Prepare request with an invalid interval
	req := interfaces.CandleRequest{
		Symbol:    "BTCUSDT",
		Interval:  "1s", // Invalid interval for Bybit
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Limit:     10,
	}
	ctx := context.Background()

	// Call GetCandles
	candles, err := c.GetCandles(ctx, req)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, candles)
	assert.ErrorIs(t, err, interfaces.ErrInvalidInterval)
}

func TestGetCandles_APIError(t *testing.T) {
	// Mock the HTTP client to return a 500 error
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: &mockRoundTripper{
			Response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString(`{"retCode": 10001, "retMsg": "Parameter error"}`)),
				Header:     make(http.Header),
			},
		},
	}
	t.Cleanup(func() { http.DefaultClient = originalClient }) // Restore original client

	c := NewConnector(nil)
	c.connected = true // Simulate connected state
	// Pre-populate instruments to pass initial checks
	c.instruments = map[string]BybitInstrumentInfo{"BTCUSDT": {Symbol: "BTCUSDT", Status: "Trading"}}

	req := interfaces.CandleRequest{
		Symbol:    "BTCUSDT",
		Interval:  "1m",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Limit:     10,
	}
	ctx := context.Background()

	// Call GetCandles
	candles, err := c.GetCandles(ctx, req)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, candles)
	assert.Contains(t, err.Error(), "status 500")      // Check for the status code in the error
	assert.Contains(t, err.Error(), "Parameter error") // Check for the Bybit error message
}

// Check log level (this assumes logger has a way to check its level)
// For example, if logger has a GetLevel() method:
// assert.Equal(t, logging.DEBUG, c.logger.GetLevel())

func TestNewConnector_InvalidLogLevel(t *testing.T) {
	options := &interfaces.ExchangeOptions{
		LogLevel: "invalid-level",
	}
	c := NewConnector(options)
	require.NotNil(t, c)
	// Assert that the logger level remains the default (e.g., INFO)
	// This requires a way to inspect the logger's level.
}

// --- Placeholder for future tests ---

// TODO: TestGetCandles_InvalidTimeRange
// TODO: TestGetCandles_APIErrorStatus
// TODO: TestGetCandles_APIErrorCode
// TODO: TestGetCandles_ParseError
// TODO: TestGetCandles_MalformedData
// TODO: TestGetTicker_Success
// TODO: TestGetTicker_InvalidSymbol
// TODO: TestGetTicker_APIError
// TODO: TestGetOrderBook_Success
// TODO: TestGetOrderBook_InvalidSymbol
// TODO: TestGetOrderBook_APIError
// TODO: TestSubscribeCandles_Success
// TODO: TestSubscribeCandles_NotConnected
// TODO: TestSubscribeCandles_InvalidSymbol
// TODO: TestSubscribeCandles_InvalidInterval
// TODO: TestSubscribeCandles_WSSubscribeFail
// TODO: TestSubscribeTicker_Success
// TODO: TestSubscribeOrderBook_Success
// TODO: TestUnsubscribe_Success (Needs implementation refinement)
// TODO: TestFetchInstrumentsInfo_Success
// TODO: TestFetchInstrumentsInfo_APIError
// TODO: TestIsValidSymbol (Needs fetched instruments)
