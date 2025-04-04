package websocket

import (
	"context"
	"sync"
)

// MockConnector implements WSConnector interface for testing
type MockConnector struct {
	mu sync.RWMutex

	connected bool
	handlers  map[string]MessageHandler
	messages  chan []byte
	config    Config

	// For verifying test expectations
	connectCalls     int
	subscribeCalls   map[string]int
	unsubscribeCalls map[string]int
	sendCalls        int
	closeCalls       int

	// For simulating errors
	connectError     error
	subscribeError   error
	unsubscribeError error
	sendError        error
	closeError       error
}

// NewMockConnector creates a new mock connector for testing
func NewMockConnector() *MockConnector {
	return &MockConnector{
		handlers:         make(map[string]MessageHandler),
		messages:         make(chan []byte, 100),
		subscribeCalls:   make(map[string]int),
		unsubscribeCalls: make(map[string]int),
		config: Config{
			URL:               "ws://mock-server.test",
			HeartbeatInterval: 20 * 1000000000, // 20 seconds in nanoseconds
			ReconnectInterval: 5 * 1000000000,  // 5 seconds in nanoseconds
			MaxRetries:        3,
		},
	}
}

// Connect implements WSConnector interface
func (m *MockConnector) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connectCalls++
	if m.connectError != nil {
		return m.connectError
	}

	m.connected = true
	return nil
}

// Close implements WSConnector interface
func (m *MockConnector) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeCalls++
	if m.closeError != nil {
		return m.closeError
	}

	m.connected = false
	return nil
}

// Subscribe implements WSConnector interface
func (m *MockConnector) Subscribe(topic string, handler MessageHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscribeCalls[topic]++
	if m.subscribeError != nil {
		return m.subscribeError
	}

	m.handlers[topic] = handler
	return nil
}

// Unsubscribe implements WSConnector interface
func (m *MockConnector) Unsubscribe(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.unsubscribeCalls[topic]++
	if m.unsubscribeError != nil {
		return m.unsubscribeError
	}

	delete(m.handlers, topic)
	return nil
}

// Send implements WSConnector interface
func (m *MockConnector) Send(message interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendCalls++
	if m.sendError != nil {
		return m.sendError
	}

	return nil
}

// IsConnected implements WSConnector interface
func (m *MockConnector) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// GetConfig implements WSConnector interface
func (m *MockConnector) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SimulateMessage simulates receiving a message for testing
func (m *MockConnector) SimulateMessage(topic string, message []byte) {
	m.mu.RLock()
	handler, exists := m.handlers[topic]
	m.mu.RUnlock()

	if exists {
		handler(message)
	}
}

// SetConnectError sets an error to be returned by Connect
func (m *MockConnector) SetConnectError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectError = err
}

// SetSubscribeError sets an error to be returned by Subscribe
func (m *MockConnector) SetSubscribeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribeError = err
}

// SetUnsubscribeError sets an error to be returned by Unsubscribe
func (m *MockConnector) SetUnsubscribeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unsubscribeError = err
}

// SetSendError sets an error to be returned by Send
func (m *MockConnector) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendError = err
}

// SetCloseError sets an error to be returned by Close
func (m *MockConnector) SetCloseError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeError = err
}

// GetConnectCalls returns the number of times Connect was called
func (m *MockConnector) GetConnectCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectCalls
}

// GetSubscribeCalls returns the number of times Subscribe was called for a topic
func (m *MockConnector) GetSubscribeCalls(topic string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.subscribeCalls[topic]
}

// GetUnsubscribeCalls returns the number of times Unsubscribe was called for a topic
func (m *MockConnector) GetUnsubscribeCalls(topic string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.unsubscribeCalls[topic]
}

// GetSendCalls returns the number of times Send was called
func (m *MockConnector) GetSendCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sendCalls
}

// GetCloseCalls returns the number of times Close was called
func (m *MockConnector) GetCloseCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closeCalls
}

// SetConfig allows setting mock config for testing
func (m *MockConnector) SetConfig(config Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}
