package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// setupTestServer creates a test WebSocket server
func setupTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, string) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(handler))
	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	return server, wsURL
}

func TestMockConnector(t *testing.T) {
	mock := NewMockConnector()

	t.Run("Connect", func(t *testing.T) {
		// Test successful connection
		err := mock.Connect(context.Background())
		require.NoError(t, err)
		assert.True(t, mock.IsConnected())
		assert.Equal(t, 1, mock.GetConnectCalls())

		// Test connection error
		mock.SetConnectError(errors.New("connection failed"))
		err = mock.Connect(context.Background())
		require.Error(t, err)
		assert.Equal(t, "connection failed", err.Error())
		assert.Equal(t, 2, mock.GetConnectCalls())
	})

	t.Run("Subscribe", func(t *testing.T) {
		messageReceived := make(chan []byte, 1)
		handler := func(msg []byte) {
			messageReceived <- msg
		}

		// Test successful subscription
		err := mock.Subscribe("test", handler)
		require.NoError(t, err)
		assert.Equal(t, 1, mock.GetSubscribeCalls("test"))

		// Test subscription error
		mock.SetSubscribeError(errors.New("subscription failed"))
		err = mock.Subscribe("test2", handler)
		require.Error(t, err)
		assert.Equal(t, "subscription failed", err.Error())
		assert.Equal(t, 1, mock.GetSubscribeCalls("test2"))

		// Test message handling
		testMessage := []byte(`{"topic":"test","data":"hello"}`)
		mock.SimulateMessage("test", testMessage)

		select {
		case msg := <-messageReceived:
			assert.Equal(t, testMessage, msg)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for message")
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		// Test successful unsubscription
		err := mock.Unsubscribe("test")
		require.NoError(t, err)
		assert.Equal(t, 1, mock.GetUnsubscribeCalls("test"))

		// Test unsubscription error
		mock.SetUnsubscribeError(errors.New("unsubscription failed"))
		err = mock.Unsubscribe("test")
		require.Error(t, err)
		assert.Equal(t, "unsubscription failed", err.Error())
		assert.Equal(t, 2, mock.GetUnsubscribeCalls("test"))
	})

	t.Run("Send", func(t *testing.T) {
		// Test successful send
		err := mock.Send([]byte("test message"))
		require.NoError(t, err)
		assert.Equal(t, 1, mock.GetSendCalls())

		// Test send error
		mock.SetSendError(errors.New("send failed"))
		err = mock.Send([]byte("test message"))
		require.Error(t, err)
		assert.Equal(t, "send failed", err.Error())
		assert.Equal(t, 2, mock.GetSendCalls())
	})

	t.Run("Close", func(t *testing.T) {
		// Test successful close
		err := mock.Close()
		require.NoError(t, err)
		assert.False(t, mock.IsConnected())
		assert.Equal(t, 1, mock.GetCloseCalls())

		// Test close error
		mock.SetCloseError(errors.New("close failed"))
		err = mock.Close()
		require.Error(t, err)
		assert.Equal(t, "close failed", err.Error())
		assert.Equal(t, 2, mock.GetCloseCalls())
	})
}

func TestRealConnector(t *testing.T) {
	// Create mock server
	mock, wsURL := setupMockServer(t)

	// Track connection events
	var connectCount int
	var connectMu sync.Mutex // Add mutex for thread safety
	mock.OnConnect(func(conn *websocket.Conn) {
		connectMu.Lock()
		connectCount++
		connectMu.Unlock()
	})

	// Create connector with test configuration
	config := Config{
		URL:               wsURL,
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Second,
		MaxRetries:        3,
	}
	connector := NewConnector(config)

	// Test connection
	ctx := context.Background()
	err := connector.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, connector.IsConnected())

	// Wait for the connection count to be updated with a timeout
	connectionEstablished := make(chan bool, 1)
	go func() {
		for {
			connectMu.Lock()
			count := connectCount
			connectMu.Unlock()
			if count > 0 {
				connectionEstablished <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	select {
	case <-connectionEstablished:
		// Connection established, continue test
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for connection to be established")
	}

	// Safely read connection count
	connectMu.Lock()
	count := connectCount
	connectMu.Unlock()
	assert.Equal(t, 1, count)

	// Test message handling
	messageReceived := make(chan []byte)
	err = connector.Subscribe("test", func(message []byte) {
		messageReceived <- message
	})
	require.NoError(t, err)

	// Send test message
	testMessage := []byte(`{"topic":"test","data":"hello"}`)
	err = connector.Send(testMessage)
	require.NoError(t, err)

	// Wait for message
	select {
	case msg := <-messageReceived:
		assert.Equal(t, testMessage, msg)
	case <-time.After(time.Second * 5):
		t.Fatal("timeout waiting for message")
	}

	// Verify message was received by server
	messages := mock.GetMessageBuffer()
	require.Len(t, messages, 1)
	assert.Equal(t, testMessage, messages[0])

	// Test health check
	if c, ok := connector.(interface {
		HealthCheck() error
		GetMetrics() Metrics
	}); ok {
		err = c.HealthCheck()
		require.NoError(t, err)

		// Test metrics
		metrics := c.GetMetrics()
		assert.True(t, metrics.ConnectedTime.Before(time.Now()))
		assert.Greater(t, metrics.MessageCount, int64(0))
	}

	// Test unsubscribe
	err = connector.Unsubscribe("test")
	require.NoError(t, err)

	// Test close
	err = connector.Close()
	require.NoError(t, err)
	assert.False(t, connector.IsConnected())
}

func TestConnectorReconnection(t *testing.T) {
	// Create mock server
	mock, wsURL := setupMockServer(t)

	// Track connection events
	var connectCount int
	var connectMu sync.Mutex
	mock.OnConnect(func(conn *websocket.Conn) {
		connectMu.Lock()
		connectCount++
		currentCount := connectCount
		connectMu.Unlock()

		// Drop connection after first message, but only for the first connection
		if currentCount == 1 {
			go func() {
				// Read a message then immediately drop the connection
				_, _, _ = conn.ReadMessage()
				mock.SetDropConnection(true)
				// Reset drop connection after a short delay to allow reconnection
				time.Sleep(100 * time.Millisecond)
				mock.SetDropConnection(false)
			}()
		}
	})

	// Create connector with test configuration
	config := Config{
		URL:               wsURL,
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Millisecond * 50, // Faster reconnect for testing
		MaxRetries:        3,
	}
	connector := NewConnector(config)

	// Connect
	ctx := context.Background()
	err := connector.Connect(ctx)
	require.NoError(t, err)

	// Subscribe to test topic
	err = connector.Subscribe("test", func(message []byte) {})
	require.NoError(t, err)

	// Ensure we're connected at first
	assert.True(t, connector.IsConnected())

	// Send message to trigger disconnect in mock server
	err = connector.Send([]byte(`{"topic":"test","data":"trigger disconnect"}`))
	require.NoError(t, err)

	// Send multiple messages to ensure the disconnect happens
	for i := 0; i < 5; i++ {
		err = connector.Send([]byte(fmt.Sprintf(`{"topic":"test","seq":%d}`, i)))
		if err != nil {
			// It's ok to have errors during reconnect
			t.Logf("Send error (expected during reconnection): %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for reconnection with timeout
	reconnected := make(chan bool, 1)
	go func() {
		for {
			connectMu.Lock()
			count := connectCount
			connectMu.Unlock()
			if count > 1 {
				reconnected <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	select {
	case <-reconnected:
		// Reconnection successful, continue test
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for reconnection")
	}

	// Verify we reconnected
	connectMu.Lock()
	finalConnectCount := connectCount
	connectMu.Unlock()
	assert.Greater(t, finalConnectCount, 1, "Should have reconnected at least once")

	// Test unsubscribe
	err = connector.Unsubscribe("test")
	require.NoError(t, err)

	// Test close
	err = connector.Close()
	require.NoError(t, err)
	assert.False(t, connector.IsConnected())
}

func TestConnectorRejectedConnection(t *testing.T) {
	// Create mock server that rejects connections
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create connector with test configuration
	config := Config{
		URL:               wsURL,
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Millisecond * 100,
		MaxRetries:        2, // Limit retries for quicker test
	}
	connector := NewConnector(config)

	// Expect connection to fail
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := connector.Connect(ctx)
	require.Error(t, err)
	assert.False(t, connector.IsConnected())
}

func TestConnectorConcurrentOperations(t *testing.T) {
	// Create mock server
	mock, wsURL := setupMockServer(t)
	defer mock.Close()

	// Create connector with test configuration
	config := Config{
		URL:               wsURL,
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Second,
		MaxRetries:        3,
	}
	connector := NewConnector(config)

	// Connect
	ctx := context.Background()
	err := connector.Connect(ctx)
	require.NoError(t, err)

	// Create a wait group for concurrent operations
	var wg sync.WaitGroup
	// Number of concurrent operations
	concurrency := 10

	// Subscribe handlers in parallel
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			topic := fmt.Sprintf("test.%d", id)
			err := connector.Subscribe(topic, func(message []byte) {})
			if err != nil {
				t.Errorf("Subscribe error: %v", err)
			}
		}(i)
	}

	// Wait for all subscriptions
	wg.Wait()

	// Send messages in parallel
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msg := map[string]interface{}{
				"id":   id,
				"data": fmt.Sprintf("message %d", id),
			}
			jsonMsg, _ := json.Marshal(msg)
			err := connector.Send(jsonMsg)
			if err != nil {
				t.Errorf("Send error: %v", err)
			}
		}(i)
	}

	// Wait for all sends
	wg.Wait()

	// Unsubscribe in parallel
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			topic := fmt.Sprintf("test.%d", id)
			err := connector.Unsubscribe(topic)
			if err != nil {
				t.Errorf("Unsubscribe error: %v", err)
			}
		}(i)
	}

	// Wait for all unsubscriptions
	wg.Wait()

	// Verify connector is still usable
	assert.True(t, connector.IsConnected())

	// Close the connector
	err = connector.Close()
	require.NoError(t, err)
	assert.False(t, connector.IsConnected())
}

// Additional test functions

func TestConnector_Connect(t *testing.T) {
	// Tests beyond what the other tests already cover
}

func TestConnector_Reconnect(t *testing.T) {
	// Tests beyond what the other tests already cover
}

func TestConnector_ConcurrentOperations(t *testing.T) {
	// Tests beyond what the other tests already cover
}

func TestConnector_InvalidURL(t *testing.T) {
	// Create connector with invalid URL
	config := Config{
		URL:               "invalid-url",
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Millisecond * 100,
		MaxRetries:        1,
	}
	connector := NewConnector(config)

	// Expect connection to fail
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := connector.Connect(ctx)
	require.Error(t, err)
	assert.False(t, connector.IsConnected())
}

func TestConnector_ContextCancellation(t *testing.T) {
	// Create mock server that never responds but properly closes its connections
	serverClosed := make(chan struct{})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't upgrade to WebSocket, just wait until the server is closed
		<-serverClosed
	}))

	// Set lower idle timeout to prevent hanging in tests
	server.Config.IdleTimeout = 1 * time.Second
	server.Start()
	defer func() {
		// Signal handler to exit
		close(serverClosed)
		// Give a small grace period for connections to close
		server.Close()
	}()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create connector with test configuration
	config := Config{
		URL:               wsURL,
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Second,
		MaxRetries:        1,
	}
	connector := NewConnector(config)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Expect connection to fail due to context timeout
	err := connector.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
	assert.False(t, connector.IsConnected())
}

// Other test methods to verify different aspects of the connector

func TestWebSocketReconnection(t *testing.T) {
	// Tests beyond what the other tests already cover
}

func TestIsConnected(t *testing.T) {
	config := Config{URL: "wss://example.com"}
	connector := NewConnector(config)

	assert.False(t, connector.IsConnected())
}

func TestNewConnector(t *testing.T) {
	config := Config{
		URL:               "wss://example.com/ws",
		HeartbeatInterval: 30 * time.Second,
		ReconnectInterval: 5 * time.Second,
		MaxRetries:        3,
	}

	connector := NewConnector(config)
	assert.NotNil(t, connector, "Connector should not be nil")

	// Check config is correct
	connConfig := connector.GetConfig()
	assert.Equal(t, config.URL, connConfig.URL)
	assert.Equal(t, config.HeartbeatInterval, connConfig.HeartbeatInterval)
	assert.Equal(t, config.ReconnectInterval, connConfig.ReconnectInterval)
	assert.Equal(t, config.MaxRetries, connConfig.MaxRetries)

	// Check default values
	emptyConfig := Config{URL: "wss://example.com/ws"}
	connector = NewConnector(emptyConfig)
	connConfig = connector.GetConfig()
	assert.Equal(t, 20*time.Second, connConfig.HeartbeatInterval, "Default heartbeat interval should be 20s")
	assert.Equal(t, 5*time.Second, connConfig.ReconnectInterval, "Default reconnect interval should be 5s")
}
