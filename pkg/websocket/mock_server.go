package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// MockServer represents a mock WebSocket server for testing
type MockServer struct {
	server *httptest.Server
	url    string

	// For tracking connections
	connections   map[*websocket.Conn]bool
	connectionsMu sync.RWMutex
	onConnect     func(*websocket.Conn)
	onDisconnect  func(*websocket.Conn)
	onMessage     func(*websocket.Conn, []byte)
	messageBuffer [][]byte

	// For simulating errors
	shouldRejectConnection bool
	shouldDropConnection   bool
}

// NewMockServer creates a new mock WebSocket server
func NewMockServer() *MockServer {
	mock := &MockServer{
		connections:   make(map[*websocket.Conn]bool),
		messageBuffer: make([][]byte, 0),
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleConnection))
	mock.url = "ws" + strings.TrimPrefix(mock.server.URL, "http")

	return mock
}

// URL returns the WebSocket URL of the mock server
func (m *MockServer) URL() string {
	return m.url
}

// Close shuts down the mock server
func (m *MockServer) Close() {
	m.server.Close()
}

// SetRejectConnection configures whether the server should reject new connections
func (m *MockServer) SetRejectConnection(reject bool) {
	m.shouldRejectConnection = reject
}

// SetDropConnection configures whether the server should drop existing connections
func (m *MockServer) SetDropConnection(drop bool) {
	m.shouldDropConnection = drop
}

// OnConnect sets a callback for when a client connects
func (m *MockServer) OnConnect(callback func(*websocket.Conn)) {
	m.onConnect = callback
}

// OnDisconnect sets a callback for when a client disconnects
func (m *MockServer) OnDisconnect(callback func(*websocket.Conn)) {
	m.onDisconnect = callback
}

// OnMessage sets a callback for when a message is received
func (m *MockServer) OnMessage(callback func(*websocket.Conn, []byte)) {
	m.onMessage = callback
}

// Broadcast sends a message to all connected clients with timeout
func (m *MockServer) Broadcast(message []byte) {
	m.connectionsMu.RLock()
	defer m.connectionsMu.RUnlock()

	for conn := range m.connections {
		go func(c *websocket.Conn) {
			// Create a timeout context
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Create a channel for the write operation
			done := make(chan error, 1)
			go func() {
				done <- c.WriteMessage(websocket.TextMessage, message)
			}()

			// Wait for write to complete or timeout
			select {
			case err := <-done:
				if err != nil {
					m.removeConnection(c)
				}
			case <-ctx.Done():
				// Write operation timed out
				m.removeConnection(c)
			}
		}(conn)
	}
}

// GetConnectionCount returns the number of active connections
func (m *MockServer) GetConnectionCount() int {
	m.connectionsMu.RLock()
	defer m.connectionsMu.RUnlock()
	return len(m.connections)
}

// GetMessageBuffer returns a copy of the message buffer
func (m *MockServer) GetMessageBuffer() [][]byte {
	m.connectionsMu.RLock()
	defer m.connectionsMu.RUnlock()

	// Create a copy to avoid concurrent access issues
	messages := make([][]byte, len(m.messageBuffer))
	copy(messages, m.messageBuffer)
	return messages
}

// ClearMessageBuffer clears the message buffer
func (m *MockServer) ClearMessageBuffer() {
	m.connectionsMu.Lock()
	defer m.connectionsMu.Unlock()
	m.messageBuffer = make([][]byte, 0)
}

// handleConnection handles incoming WebSocket connections
func (m *MockServer) handleConnection(w http.ResponseWriter, r *http.Request) {
	if m.shouldRejectConnection {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	m.addConnection(conn)
	if m.onConnect != nil {
		m.onConnect(conn)
	}

	defer func() {
		m.removeConnection(conn)
		if m.onDisconnect != nil {
			m.onDisconnect(conn)
		}
		conn.Close()
	}()

	// Immediately store the connection in our buffer
	// This is critical for TestConnectorReconnection which tracks connection count
	m.connectionsMu.Lock()
	m.connectionsMu.Unlock()

	for {
		if m.shouldDropConnection {
			return
		}

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if messageType == websocket.TextMessage {
			// Store message in buffer
			m.connectionsMu.Lock()
			m.messageBuffer = append(m.messageBuffer, message)
			m.connectionsMu.Unlock()

			// Call message handler if set
			if m.onMessage != nil {
				m.onMessage(conn, message)
			}

			// Echo message back by default
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
}

// addConnection adds a connection to the tracking map
func (m *MockServer) addConnection(conn *websocket.Conn) {
	m.connectionsMu.Lock()
	defer m.connectionsMu.Unlock()
	m.connections[conn] = true
}

// removeConnection removes a connection from the tracking map
func (m *MockServer) removeConnection(conn *websocket.Conn) {
	m.connectionsMu.Lock()
	defer m.connectionsMu.Unlock()
	delete(m.connections, conn)
}

// Helper function to create a mock server for testing
func setupMockServer(t *testing.T) (*MockServer, string) {
	t.Helper()
	mock := NewMockServer()
	t.Cleanup(func() {
		mock.Close()
	})
	return mock, mock.URL()
}
