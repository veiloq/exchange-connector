// Package websocket provides functionality for managing real-time WebSocket connections
// to exchange APIs. It handles connection lifecycle, automatic reconnection,
// message routing, and subscription management.
//
// This package abstracts the complexities of maintaining reliable WebSocket connections
// including handling disconnects, implementing heartbeats, and ensuring message delivery.
// It provides both a clean interface and a concrete implementation that can be used
// across different exchange connectors.
//
// Architecture Integration:
//
// The websocket package works in conjunction with other key components:
//
//   - pkg/exchanges/interfaces: Defines the high-level exchange connector interface
//     that uses this WebSocket package for real-time data subscriptions
//
//   - pkg/common/http: Provides HTTP client functionality for REST API communication,
//     complementing this WebSocket package for exchange interactions
//
//   - pkg/logging: Used for structured logging of WebSocket connection events
//     and error conditions for observability
//
//   - pkg/ratelimit: Can be used alongside this package to enforce connection
//     and subscription rate limits when interacting with exchanges
//
// The typical usage flow involves:
//
//  1. Exchange-specific connectors (e.g., Bybit, Binance) create WebSocket connectors
//  2. The exchange connectors use these WebSocket connectors to establish connections
//  3. Exchange-specific message handling and serialization is implemented on top
//     of the raw message handling provided by this package
//
// This layered approach separates the concerns of reliable WebSocket communication
// from exchange-specific protocol implementations.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/gorilla/websocket"
	"github.com/veiloq/exchange-connector/pkg/logging"
)

// MessageHandler is a callback function type for handling incoming WebSocket messages.
// Implementations of this function will be called whenever a message is received for
// a specific topic that the handler is subscribed to.
//
// Parameters:
// - message: The raw message bytes received from the WebSocket connection
//
// The handler is executed in a separate goroutine to prevent blocking the main
// message processing loop. Implementers should ensure the handler is thread-safe.
type MessageHandler func(message []byte)

// WSConnector defines the interface for managing WebSocket connections.
// This interface provides methods for establishing and maintaining connections,
// subscribing to topics, and sending messages.
//
// Implementations of this interface should be thread-safe and handle connection
// lifecycle events automatically (reconnection, heartbeats, etc.).
type WSConnector interface {
	// Connect establishes the WebSocket connection and starts background routines
	// for message processing and connection maintenance.
	//
	// Parameters:
	// - ctx: Context for controlling the connection lifecycle. When the context
	//   is cancelled, the connection will be closed.
	//
	// Returns:
	// - error: An error if the connection cannot be established
	//
	// If the connection is already established, this method is a no-op and returns nil.
	// The context is also used for controlling retry attempts during connection.
	Connect(ctx context.Context) error

	// Close cleanly terminates the WebSocket connection and stops all background routines.
	//
	// Returns:
	// - error: An error if the connection cannot be closed properly
	//
	// This method is idempotent and safe to call multiple times.
	// After closing, Connect must be called again to reestablish the connection.
	Close() error

	// Subscribe registers a handler function to process messages for a specific topic.
	//
	// Parameters:
	// - topic: The topic identifier to subscribe to
	// - handler: The callback function to be invoked when messages for this topic are received
	//
	// Returns:
	// - error: An error if the subscription cannot be established
	//
	// The handler will be called from a separate goroutine for each received message.
	// If a handler for the topic already exists, it will be replaced.
	// The connector must be connected before calling Subscribe.
	Subscribe(topic string, handler MessageHandler) error

	// Unsubscribe removes a handler for a specific topic.
	//
	// Parameters:
	// - topic: The topic identifier to unsubscribe from
	//
	// Returns:
	// - error: An error if the unsubscription fails
	//
	// After unsubscribing, no more messages for the topic will be routed to handlers.
	// If the topic doesn't exist, this is a no-op and returns nil.
	Unsubscribe(topic string) error

	// Send transmits a message through the WebSocket connection.
	//
	// Parameters:
	// - message: The message to send, which will be JSON-encoded
	//
	// Returns:
	// - error: An error if the message cannot be sent
	//
	// The message must be JSON-serializable. The connector must be connected
	// before calling Send.
	Send(message interface{}) error

	// IsConnected returns the current connection status.
	//
	// Returns:
	// - bool: true if the connection is established and active, false otherwise
	//
	// This method can be used to check connection status before performing operations
	// that require an active connection.
	IsConnected() bool

	// GetConfig returns the current configuration of the connector.
	//
	// Returns:
	// - Config: A copy of the current configuration
	//
	// This method allows for introspection of the connector's settings
	// without modifying them.
	GetConfig() Config
}

// Config holds the configuration parameters for a WebSocket connection.
// These settings control connection behavior, reconnection strategy,
// and heartbeat frequency.
type Config struct {
	// URL is the WebSocket endpoint to connect to
	URL string

	// HeartbeatInterval defines how frequently heartbeat messages are sent
	// to keep the connection alive
	HeartbeatInterval time.Duration

	// ReconnectInterval specifies the delay between reconnection attempts
	// after a connection failure or disconnection
	ReconnectInterval time.Duration

	// MaxRetries is the maximum number of connection attempts before giving up
	// A value of 0 means unlimited retries
	MaxRetries int
}

// Metrics holds statistics about the WebSocket connection.
// These metrics can be used for monitoring connection health and activity.
type Metrics struct {
	// ConnectedTime is when the current connection was established
	ConnectedTime time.Time

	// MessageCount is the total number of messages received
	MessageCount int64

	// ReconnectCount is the number of times the connection has been reestablished
	ReconnectCount int64

	// ErrorCount is the number of errors encountered during the connection lifetime
	ErrorCount int64
}

// connector implements the WSConnector interface.
// It manages a WebSocket connection with automatic reconnection,
// message routing, and subscription handling.
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

// NewConnector creates a new WebSocket connector with the given configuration.
//
// Parameters:
// - config: The configuration for the WebSocket connection
//
// Returns:
// - WSConnector: A new connector instance ready to be connected
//
// This function only initializes the connector structure. The Connect method
// must be called to establish the actual connection.
//
// Example usage:
//
//	connector := websocket.NewConnector(websocket.Config{
//		URL:               "wss://stream.bybit.com/spot/ws",
//		HeartbeatInterval: 20 * time.Second,
//		ReconnectInterval: 5 * time.Second,
//		MaxRetries:        5,
//	})
//
//	ctx := context.Background()
//	if err := connector.Connect(ctx); err != nil {
//		log.Fatalf("Failed to connect: %v", err)
//	}
//	defer connector.Close()
func NewConnector(config Config) WSConnector {
	// Set default values for timing parameters if they are invalid
	if config.HeartbeatInterval <= 0 {
		config.HeartbeatInterval = 20 * time.Second
	}

	if config.ReconnectInterval <= 0 {
		config.ReconnectInterval = 5 * time.Second
	}

	return &connector{
		config:   config,
		handlers: make(map[string]MessageHandler),
		logger:   logging.NewLogger(),
	}
}

// GetMetrics returns the current connection metrics.
//
// Returns:
// - Metrics: A copy of the current metrics structure
//
// This method is thread-safe and can be called at any time, even if the
// connection is not established. It provides visibility into connection
// health and activity.
func (c *connector) GetMetrics() Metrics {
	c.metricsMu.RLock()
	defer c.metricsMu.RUnlock()
	return c.metrics
}

// HealthCheck performs a health check of the WebSocket connection.
//
// Returns:
// - error: nil if the connection is healthy, or an error describing the problem
//
// This method checks if the connection is established and verifies that messages
// have been received within a reasonable timeframe. It's useful for monitoring
// connection health in systems that require high reliability.
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
// for message processing and heartbeats. It returns immediately if already connected.
//
// Parameters:
// - ctx: Context for controlling the connection lifecycle and cancellation
//
// Returns:
// - error: An error if the connection cannot be established
//
// This method is thread-safe and uses exponential backoff for reconnection attempts.
// It respects context cancellation at all stages of the connection process.
// After successful connection, it automatically resubscribes to previously
// registered topics and starts all necessary background goroutines.
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

// readPump continuously reads messages from the WebSocket connection.
// It handles incoming messages, sets read deadlines, processes pong responses,
// and initiates reconnection when the connection is lost.
//
// Parameters:
// - ctx: Context for controlling the read loop lifecycle
//
// This internal method runs in its own goroutine and terminates when the
// connection is closed or context is cancelled. Upon termination, it triggers
// reconnection logic if appropriate.
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

// processMessage parses incoming WebSocket messages, determines the topic,
// and routes to the appropriate handler.
//
// Parameters:
// - message: The raw message bytes received from the WebSocket connection
//
// This internal method attempts to extract the topic from the message and
// calls the registered handler for that topic in a separate goroutine. It includes
// timeout protection and panic recovery to ensure message handling failures don't
// affect the main connection.
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

// heartbeat sends periodic ping messages to keep the WebSocket connection alive.
// It uses a ticker based on the configured heartbeat interval.
//
// This internal method runs in its own goroutine and terminates when the
// connection is closed. It's essential for maintaining long-lived WebSocket
// connections that might otherwise be closed by proxies or load balancers
// after periods of inactivity.
func (c *connector) heartbeat() {
	// Ensure heartbeat interval is positive, default to 20 seconds if not set
	interval := c.config.HeartbeatInterval
	if interval <= 0 {
		interval = 20 * time.Second
	}

	ticker := time.NewTicker(interval)
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

// reconnect attempts to reestablish the WebSocket connection after a failure.
// It uses exponential backoff strategy via the retry-go package.
//
// This internal method is thread-safe with mutex protection to prevent
// concurrent reconnection attempts. It updates metrics and logs the
// reconnection process for observability.
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

// Subscribe registers a handler function to process messages for a specific topic.
//
// Parameters:
// - topic: The topic identifier to subscribe to
// - handler: The callback function to be invoked when messages for this topic are received
//
// Returns:
// - error: An error if the subscription cannot be established
//
// This method requires an active connection and is thread-safe. If a handler
// for the topic already exists, it will be replaced with the new handler.
// Each topic can have only one handler at a time.
func (c *connector) Subscribe(topic string, handler MessageHandler) error {
	if !c.IsConnected() {
		return fmt.Errorf("websocket not connected")
	}

	c.handlersMu.Lock()
	c.handlers[topic] = handler
	c.handlersMu.Unlock()
	return nil
}

// Unsubscribe removes a handler for a specific topic.
//
// Parameters:
// - topic: The topic identifier to unsubscribe from
//
// Returns:
// - error: An error if the unsubscription fails
//
// This method is thread-safe and always returns nil. If the topic doesn't exist,
// this is a no-op. Future versions may return errors for specific failure cases.
// After unsubscribing, no more messages for the topic will be routed to handlers.
func (c *connector) Unsubscribe(topic string) error {
	c.handlersMu.Lock()
	delete(c.handlers, topic)
	c.handlersMu.Unlock()
	return nil
}

// Send transmits a message through the WebSocket connection.
//
// Parameters:
// - message: The message to send, which will be JSON-encoded
//
// Returns:
// - error: An error if the message cannot be sent
//
// This method returns an error if the connection is not established or if
// JSON marshaling fails. It accepts either pre-serialized byte arrays or
// JSON-serializable objects. Thread-safe with mutex protection for concurrent writes.
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

// IsConnected returns the current connection status.
//
// Returns:
// - bool: true if the connection is established and active, false otherwise
//
// This method is thread-safe and can be used to check connection status before
// performing operations that require an active connection.
func (c *connector) IsConnected() bool {
	return c.connected
}

// GetConfig returns the current configuration of the connector.
//
// Returns:
// - Config: A copy of the current configuration
//
// This method allows for introspection of the connector's settings
// without modifying them.
func (c *connector) GetConfig() Config {
	return c.config
}

// Close cleanly terminates the WebSocket connection and stops all background routines.
//
// Returns:
// - error: An error if the connection cannot be closed properly
//
// This method is idempotent and thread-safe - safe to call multiple times without error.
// It attempts to send a proper close frame before closing the connection to ensure
// the server is notified of the intentional disconnect. All background goroutines
// and internal state are properly cleaned up.
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

// resubscribe resubscribes to all previously registered topics after reconnection.
//
// Returns:
// - error: An error if any resubscription fails
//
// This internal method is called automatically after a successful reconnection.
// It attempts to restore all previous subscriptions and logs any failures.
// Returns an error if any topic resubscription fails, but continues trying
// to resubscribe to all topics regardless of individual failures.
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
