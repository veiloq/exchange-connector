package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/veiloq/exchange-connector/pkg/logging"
	"github.com/gorilla/websocket"
)

// MessageHandler is a callback function type for handling incoming WebSocket messages
type MessageHandler func(message []byte)

// WSConnector defines the interface for managing WebSocket connections
type WSConnector interface {
	// Connect establishes the WebSocket connection
	Connect(ctx context.Context) error

	// Close cleanly closes the WebSocket connection
	Close() error

	// Subscribe adds a message handler for a topic
	Subscribe(topic string, handler MessageHandler) error

	// Unsubscribe removes a message handler for a topic
	Unsubscribe(topic string) error

	// Send sends a message through the WebSocket connection
	Send(message interface{}) error

	// IsConnected returns the current connection status
	IsConnected() bool
}

// Config holds WebSocket connection configuration
type Config struct {
	URL               string
	HeartbeatInterval time.Duration
	ReconnectInterval time.Duration
	MaxRetries        int
}

// Metrics holds connection and message statistics
type Metrics struct {
	ConnectedTime  time.Time
	MessageCount   int64
	ReconnectCount int64
	ErrorCount     int64
}

// connector implements the WSConnector interface
type connector struct {
	config Config
	conn   *websocket.Conn

	handlers   map[string]MessageHandler
	handlersMu sync.RWMutex // Protect handlers map
	writeMu    sync.Mutex

	connected bool
	done      chan struct{}
	doneMu    sync.Mutex
	closed    bool

	// For managing reconnection attempts
	reconnectMu   sync.Mutex
	reconnecting  bool
	lastConnected time.Time

	// Metrics
	metrics   Metrics
	metricsMu sync.RWMutex

	// Logger
	logger logging.Logger
}

// NewConnector creates a new WebSocket connector with the given configuration
func NewConnector(config Config) WSConnector {
	return &connector{
		config:   config,
		handlers: make(map[string]MessageHandler),
		logger:   logging.NewLogger(),
	}
}

// GetMetrics returns the current connection metrics
func (c *connector) GetMetrics() Metrics {
	c.metricsMu.RLock()
	defer c.metricsMu.RUnlock()
	return c.metrics
}

// HealthCheck performs a health check of the WebSocket connection
func (c *connector) HealthCheck() error {
	if !c.IsConnected() {
		return fmt.Errorf("websocket not connected")
	}

	c.metricsMu.RLock()
	lastMessage := time.Since(c.metrics.ConnectedTime)
	c.metricsMu.RUnlock()

	if lastMessage > c.config.HeartbeatInterval*3 {
		return fmt.Errorf("no messages received in %v", lastMessage)
	}

	return nil
}

// Connect establishes the WebSocket connection and starts background routines
func (c *connector) Connect(ctx context.Context) error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	if c.connected {
		return nil
	}

	// Check if context is already cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("context already cancelled: %w", ctx.Err())
	}

	c.logger.Debug("attempting websocket connection",
		logging.String("url", c.config.URL),
		logging.Duration("heartbeat", c.config.HeartbeatInterval),
		logging.Duration("reconnect", c.config.ReconnectInterval),
	)

	var lastErr error
	attempt := 0

	for {
		attempt++
		if attempt > c.config.MaxRetries {
			return fmt.Errorf("max retries exceeded: %w", lastErr)
		}

		// Check context before each attempt
		if ctx.Err() != nil {
			return ctx.Err()
		}

		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		}
		conn, _, err := dialer.DialContext(ctx, c.config.URL, nil)
		if err != nil {
			lastErr = err
			c.metricsMu.Lock()
			c.metrics.ErrorCount++
			c.metricsMu.Unlock()
			c.logger.Warn("connection attempt failed",
				logging.Int("attempt", attempt),
				logging.Error(err),
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.ReconnectInterval):
				continue
			}
		}

		c.conn = conn
		c.connected = true
		c.metricsMu.Lock()
		c.metrics.ConnectedTime = time.Now()
		c.metrics.ReconnectCount++
		c.metricsMu.Unlock()

		c.doneMu.Lock()
		c.done = make(chan struct{})
		c.closed = false
		c.doneMu.Unlock()

		// Start background routines
		go c.readPump(ctx)
		go c.heartbeat()

		// Monitor context cancellation
		go func() {
			select {
			case <-ctx.Done():
				c.logger.Info("context cancelled, closing connection")
				c.Close()
			case <-c.done:
				return
			}
		}()

		c.logger.Info("websocket connected successfully")

		// Resubscribe to topics
		if err := c.resubscribe(); err != nil {
			c.logger.Warn("failed to resubscribe", logging.Error(err))
		}

		return nil
	}
}

// readPump continuously reads messages from the WebSocket
func (c *connector) readPump(ctx context.Context) {
	defer func() {
		c.connected = false
		if c.conn != nil {
			_ = c.conn.Close()
		}

		c.doneMu.Lock()
		if !c.closed {
			close(c.done)
			c.closed = true
		}
		c.doneMu.Unlock()

		c.logger.Info("readPump stopped")

		// Only attempt reconnection if not explicitly closed and context is not cancelled
		if !c.reconnecting && ctx.Err() == nil {
			go c.reconnect()
		}
	}()

	c.conn.SetReadDeadline(time.Now().Add(c.config.HeartbeatInterval * 3))

	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.HeartbeatInterval * 3))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("context cancelled, closing readPump")
			return
		default:
			c.conn.SetReadDeadline(time.Now().Add(c.config.HeartbeatInterval * 3))
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Warn("read error", logging.Error(err))
					c.metricsMu.Lock()
					c.metrics.ErrorCount++
					c.metricsMu.Unlock()
				}
				return
			}

			c.metricsMu.Lock()
			c.metrics.MessageCount++
			c.metricsMu.Unlock()

			c.processMessage(message)
		}
	}
}

// processMessage processes an incoming WebSocket message
func (c *connector) processMessage(message []byte) {
	// Parse message to determine topic
	var msg struct {
		Topic string `json:"topic"`
	}
	if err := json.Unmarshal(message, &msg); err != nil {
		c.logger.Warn("failed to unmarshal message", logging.Error(err))
		return
	}

	// Call registered handler for the topic
	c.handlersMu.RLock()
	handler, exists := c.handlers[msg.Topic]
	c.handlersMu.RUnlock()

	if exists {
		go func(topic string, data []byte, h MessageHandler) {
			defer func() {
				if r := recover(); r != nil {
					c.logger.Error("handler panic recovered",
						logging.String("topic", topic),
						logging.String("panic", fmt.Sprintf("%v", r)),
					)
				}
			}()

			// Create a timeout context for the handler
			handlerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			done := make(chan struct{})
			go func() {
				h(data)
				close(done)
			}()

			select {
			case <-done:
				// Handler completed successfully
			case <-handlerCtx.Done():
				c.logger.Warn("handler timeout", logging.String("topic", topic))
			}
		}(msg.Topic, message, handler)
	}
}

// heartbeat sends periodic ping messages to keep the connection alive
func (c *connector) heartbeat() {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.writeMu.Lock()
			if !c.connected {
				c.writeMu.Unlock()
				return
			}
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			c.writeMu.Unlock()
			if err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

// reconnect attempts to reestablish the connection
func (c *connector) reconnect() {
	c.reconnectMu.Lock()
	if c.reconnecting {
		c.reconnectMu.Unlock()
		return
	}
	c.reconnecting = true
	c.reconnectMu.Unlock()

	defer func() {
		c.reconnectMu.Lock()
		c.reconnecting = false
		c.reconnectMu.Unlock()
	}()

	// Create context with timeout for reconnection
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Increment reconnection metrics before attempting
	c.metricsMu.Lock()
	c.metrics.ReconnectCount++
	c.metricsMu.Unlock()

	err := retry.Do(
		func() error {
			if ctx.Err() != nil {
				return retry.Unrecoverable(ctx.Err())
			}
			return c.Connect(ctx)
		},
		retry.Attempts(uint(c.config.MaxRetries)),
		retry.Delay(c.config.ReconnectInterval),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			c.logger.Warn("reconnection attempt failed",
				logging.Int("attempt", int(n+1)),
				logging.Error(err))
		}),
	)

	if err != nil {
		c.logger.Error("reconnection failed", logging.Error(err))
		c.metricsMu.Lock()
		c.metrics.ErrorCount++
		c.metricsMu.Unlock()
		return
	}

	c.logger.Info("reconnection successful")
}

// Subscribe implements WSConnector interface
func (c *connector) Subscribe(topic string, handler MessageHandler) error {
	if !c.IsConnected() {
		return fmt.Errorf("websocket not connected")
	}

	c.handlersMu.Lock()
	c.handlers[topic] = handler
	c.handlersMu.Unlock()
	return nil
}

// Unsubscribe implements WSConnector interface
func (c *connector) Unsubscribe(topic string) error {
	c.handlersMu.Lock()
	delete(c.handlers, topic)
	c.handlersMu.Unlock()
	return nil
}

// Send implements WSConnector interface
func (c *connector) Send(message interface{}) error {
	if !c.connected {
		return fmt.Errorf("websocket not connected")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// If message is already []byte, send it directly
	if data, ok := message.([]byte); ok {
		return c.conn.WriteMessage(websocket.TextMessage, data)
	}

	// Otherwise, marshal to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// IsConnected implements WSConnector interface
func (c *connector) IsConnected() bool {
	return c.connected
}

// Close implements WSConnector interface
func (c *connector) Close() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.doneMu.Lock()
	wasClosed := c.closed
	if !c.closed {
		close(c.done)
		c.closed = true
	}
	c.doneMu.Unlock()

	if wasClosed {
		return nil // Already closed
	}

	// Stop all background goroutines
	c.connected = false

	// Safely close the connection
	if c.conn != nil {
		// Try to send close message but don't error if it fails
		_ = c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closed connection"))

		// Give a bit of time for the close message to be sent before closing
		time.Sleep(100 * time.Millisecond)

		// Close the connection and ignore any "use of closed network connection" errors
		err := c.conn.Close()
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			return err
		}
	}

	return nil
}

// resubscribe resubscribes to all previously registered topics
func (c *connector) resubscribe() error {
	c.handlersMu.RLock()
	handlers := make(map[string]MessageHandler, len(c.handlers))
	for topic, handler := range c.handlers {
		handlers[topic] = handler
	}
	c.handlersMu.RUnlock()

	var errs []error
	for topic, handler := range handlers {
		if err := c.Subscribe(topic, handler); err != nil {
			c.logger.Error("failed to resubscribe",
				logging.String("topic", topic),
				logging.Error(err),
			)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to resubscribe to %d topics", len(errs))
	}
	return nil
}
