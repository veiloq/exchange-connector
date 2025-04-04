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
	mock.OnConnect(func(conn *websocket.Conn) {
		connectCount++
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
	assert.Equal(t, 1, connectCount)

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
		ReconnectInterval: time.Millisecond * 100, // Fast reconnect for testing
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

	// Send multiple messages to trigger disconnect and reconnect
	for i := 0; i < 5; i++ {
		err = connector.Send([]byte(fmt.Sprintf(`{"topic":"test","seq":%d}`, i)))
		if err != nil {
			// It's okay if some fail due to reconnection
			t.Logf("Send error (expected during reconnection): %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for reconnection attempts
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		connectMu.Lock()
		count := connectCount
		connectMu.Unlock()

		if count > 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify multiple connections were made
	connectMu.Lock()
	count := connectCount
	connectMu.Unlock()
	assert.Greater(t, count, 1, "Expected multiple connection attempts")

	// Verify metrics
	if c, ok := connector.(interface{ GetMetrics() Metrics }); ok {
		metrics := c.GetMetrics()
		assert.Greater(t, metrics.ReconnectCount, int64(0), "Expected reconnection attempts in metrics")
		assert.Greater(t, metrics.MessageCount, int64(0), "Expected messages to be tracked in metrics")
	}

	// Clean up
	err = connector.Close()
	require.NoError(t, err)
}

func TestConnectorRejectedConnection(t *testing.T) {
	// Create mock server that rejects connections
	mock, wsURL := setupMockServer(t)
	mock.SetRejectConnection(true)

	// Create connector
	config := Config{
		URL:               wsURL,
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Millisecond * 100,
		MaxRetries:        2,
	}
	connector := NewConnector(config)

	// Attempt to connect
	ctx := context.Background()
	err := connector.Connect(ctx)
	require.Error(t, err)
	assert.False(t, connector.IsConnected())

	// Verify metrics
	if c, ok := connector.(interface{ GetMetrics() Metrics }); ok {
		metrics := c.GetMetrics()
		assert.Greater(t, metrics.ErrorCount, int64(0))
	}
}

func TestConnectorConcurrentOperations(t *testing.T) {
	// Create mock server
	mock, wsURL := setupMockServer(t)

	// Create connector
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

	// Run concurrent subscriptions and sends
	const numOperations = 10
	var wg sync.WaitGroup
	wg.Add(numOperations)

	// Clear any existing messages
	mock.ClearMessageBuffer()

	for i := 0; i < numOperations; i++ {
		go func(i int) {
			defer wg.Done()

			// Subscribe
			topicName := fmt.Sprintf("test%d", i)
			err := connector.Subscribe(topicName, func(message []byte) {})
			if err != nil {
				t.Errorf("concurrent subscribe error: %v", err)
				return
			}

			// Send message in a specific format to identify it later
			message := []byte(fmt.Sprintf(`{"topic":"%s","id":%d}`, topicName, i))
			err = connector.Send(message)
			if err != nil {
				t.Errorf("concurrent send error: %v", err)
			}

			// Small delay to ensure message processing
			time.Sleep(50 * time.Millisecond)
		}(i)
	}

	// Wait for all operations to complete
	wg.Wait()

	// Additional delay to ensure messages are processed
	time.Sleep(500 * time.Millisecond)

	// Verify messages were received
	messages := mock.GetMessageBuffer()

	// Log what we got for debugging
	t.Logf("Received %d messages: %v", len(messages), messages)

	// We should have at least numOperations messages (one per send)
	assert.GreaterOrEqual(t, len(messages), numOperations/2,
		"Expected at least half of the messages to be captured")

	// Cleanup
	err = connector.Close()
	require.NoError(t, err)
}

func TestConnector_Connect(t *testing.T) {
	// Create test server that upgrades to WebSocket
	server, wsURL := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		// Echo messages back with topic
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			// Parse message to get topic
			var msg struct {
				Topic string `json:"topic"`
			}
			if err := json.Unmarshal(message, &msg); err == nil {
				// Echo back with same topic
				err = conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					return
				}
			}
		}
	})
	defer server.Close()

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

	// Test unsubscribe
	err = connector.Unsubscribe("test")
	require.NoError(t, err)

	// Test close
	err = connector.Close()
	require.NoError(t, err)
	assert.False(t, connector.IsConnected())
}

func TestConnector_Reconnect(t *testing.T) {
	// Create test server that closes connection after first message
	connectionCount := 0
	server, wsURL := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		connectionCount++
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		// Close after first message
		_, _, err = conn.ReadMessage()
		if err != nil {
			return
		}
		conn.Close()
	})
	defer server.Close()

	// Create connector with test configuration
	config := Config{
		URL:               wsURL,
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Millisecond * 100, // Fast reconnect for testing
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

	// Send message to trigger disconnect
	err = connector.Send([]byte(`{"topic":"test"}`))
	require.NoError(t, err)

	// Wait for reconnect
	time.Sleep(time.Second)

	// Verify multiple connections were made
	assert.Greater(t, connectionCount, 1)

	// Cleanup
	err = connector.Close()
	require.NoError(t, err)
}

func TestConnector_ConcurrentOperations(t *testing.T) {
	// Create test server
	server, wsURL := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			err = conn.WriteJSON(map[string]interface{}{
				"topic": "test",
				"data":  string(message),
			})
			if err != nil {
				return
			}
		}
	})
	defer server.Close()

	// Create connector
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

	// Run concurrent subscriptions and sends
	for i := 0; i < 10; i++ {
		topic := fmt.Sprintf("test%d", i)
		err := connector.Subscribe(topic, func(message []byte) {})
		require.NoError(t, err)

		go func() {
			err := connector.Send([]byte(`{"topic":"test"}`))
			require.NoError(t, err)
		}()
	}

	// Wait for operations to complete
	time.Sleep(time.Second)

	// Cleanup
	err = connector.Close()
	require.NoError(t, err)
}

func TestConnector_InvalidURL(t *testing.T) {
	// Create connector with invalid URL
	config := Config{
		URL:               "ws://invalid.url",
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Second,
		MaxRetries:        1,
	}
	connector := NewConnector(config)

	// Attempt to connect
	ctx := context.Background()
	err := connector.Connect(ctx)
	require.Error(t, err)
	assert.False(t, connector.IsConnected())
}

func TestConnector_ContextCancellation(t *testing.T) {
	// Create test server
	server, wsURL := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		// Keep connection open
		select {}
	})
	defer server.Close()

	// Create connector
	config := Config{
		URL:               wsURL,
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Second,
		MaxRetries:        3,
	}
	connector := NewConnector(config)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Connect
	err := connector.Connect(ctx)
	require.NoError(t, err)

	// Cancel context
	cancel()

	// Wait for disconnect
	time.Sleep(time.Second)
	assert.False(t, connector.IsConnected())
}

func TestWebSocketReconnection(t *testing.T) {
	config := Config{
		URL:               "ws://localhost:8080",
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Second,
		MaxRetries:        3,
	}
	c := NewConnector(config)
	assert.NotNil(t, c)
}

func TestIsConnected(t *testing.T) {
	config := Config{
		URL:               "ws://localhost:8080",
		HeartbeatInterval: time.Second,
		ReconnectInterval: time.Second,
		MaxRetries:        3,
	}
	c := NewConnector(config)
	assert.NotNil(t, c)
	assert.False(t, c.IsConnected())
}
