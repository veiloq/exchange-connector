# Debugging, Testing, and Refactoring Guide

## Debugging

### Common Issues and Solutions

#### WebSocket Connection Issues

1. **Connection Failures**
   ```go
   // Enable debug logging
   logger.SetLevel(logging.DEBUG)
   
   // Check connection status
   if !connector.IsConnected() {
       // Attempt reconnection
       err := connector.Connect(ctx)
   }
   ```

2. **Message Handling Issues**
   - Enable verbose WebSocket logging
   - Check message format in handlers
   - Verify topic subscriptions

3. **Rate Limiting Issues**
   - Monitor rate limiter logs
   - Check rate limit configuration
   - Verify exchange limits

### Debugging Tools

1. **Logging**
   ```go
   logger := logging.NewLogger()
   logger.SetLevel(logging.DEBUG)
   logger.Debug("websocket message",
       logging.String("topic", topic),
       logging.String("message", string(message)),
   )
   ```

2. **Network Monitoring**
   ```bash
   # Monitor WebSocket traffic
   tcpdump -i lo0 -n port 443 -w dump.pcap
   
   # Analyze with Wireshark
   wireshark dump.pcap
   ```

3. **Performance Profiling**
   ```go
   import "net/http/pprof"
   
   // Enable pprof endpoints
   go func() {
       log.Println(http.ListenAndServe("localhost:6060", nil))
   }()
   ```

   ```bash
   # CPU profile
   go tool pprof http://localhost:6060/debug/pprof/profile
   
   # Memory profile
   go tool pprof http://localhost:6060/debug/pprof/heap
   ```

### Debugging Process

1. **Identify the Issue**
   - Check logs for errors
   - Review recent changes
   - Isolate the problem area

2. **Gather Information**
   - Enable debug logging
   - Check system resources
   - Monitor network connections

3. **Reproduce the Issue**
   - Create minimal test case
   - Document steps to reproduce
   - Note environment details

4. **Fix and Verify**
   - Apply fix in isolated branch
   - Add regression tests
   - Verify in test environment

## Testing

### Unit Tests

1. **WebSocket Tests**
   ```go
   func TestWebSocketReconnection(t *testing.T) {
       // Setup test server
       server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           // Mock WebSocket server
       }))
       defer server.Close()
       
       // Test reconnection logic
   }
   ```

2. **Rate Limiter Tests**
   ```go
   func TestRateLimiter(t *testing.T) {
       limiter := ratelimit.NewTokenBucketLimiter(ratelimit.Rate{
           Limit:    10,
           Interval: time.Second,
       })
       
       // Test rate limiting behavior
   }
   ```

### Integration Tests

1. **Exchange API Tests**
   ```go
   func TestExchangeConnector(t *testing.T) {
       if testing.Short() {
           t.Skip("skipping integration test")
       }
       
       // Test against real exchange API
   }
   ```

2. **WebSocket Integration**
   ```go
   func TestWebSocketIntegration(t *testing.T) {
       // Test real-time data streaming
       // Verify message handling
       // Check reconnection
   }
   ```

### End-to-End Tests

1. **Setup Test Environment**
   ```bash
   # Set test credentials
   export BYBIT_API_KEY=your_test_key
   export BYBIT_API_SECRET=your_test_secret
   
   # Run e2e tests
   make e2e-test
   ```

2. **Test Scenarios**
   - Market data retrieval
   - WebSocket subscriptions
   - Error handling
   - Rate limiting
   - Reconnection

### Test Coverage

```bash
# Generate coverage report
make test-cover

# View coverage in browser
go tool cover -html=coverage.out
```

## Refactoring

### Code Quality

1. **Linting**
   ```bash
   # Run linter
   make lint
   
   # Fix common issues
   golangci-lint run --fix
   ```

2. **Code Format**
   ```bash
   # Format code
   make fmt
   ```

### Refactoring Patterns

1. **Interface Segregation**
   ```go
   // Before
   type ExchangeConnector interface {
       // Many methods
   }
   
   // After
   type MarketDataConnector interface {
       // Market data methods
   }
   
   type TradingConnector interface {
       // Trading methods
   }
   ```

2. **Error Handling**
   ```go
   // Before
   if err != nil {
       return err
   }
   
   // After
   if err != nil {
       return fmt.Errorf("failed to connect to exchange: %w", err)
   }
   ```

### Performance Optimization

1. **Connection Pooling**
   ```go
   // Implement connection pool
   type Pool struct {
       conns    chan *websocket.Conn
       factory  func() (*websocket.Conn, error)
       capacity int
   }
   ```

2. **Message Buffering**
   ```go
   // Buffer messages
   type MessageBuffer struct {
       messages chan []byte
       size     int
   }
   ```

### Monitoring

1. **Metrics Collection**
   ```go
   type Metrics struct {
       ConnectedTime    time.Time
       MessageCount     int64
       ReconnectCount   int64
       ErrorCount       int64
   }
   ```

2. **Health Checks**
   ```go
   func (c *Connector) HealthCheck() error {
       if !c.IsConnected() {
           return errors.New("websocket not connected")
       }
       return nil
   }
   ```

## Best Practices

### Error Handling

1. **Use Error Wrapping**
   ```go
   if err != nil {
       return fmt.Errorf("failed to fetch market data: %w", err)
   }
   ```

2. **Custom Error Types**
   ```go
   type ExchangeError struct {
       Code    int
       Message string
       Err     error
   }
   ```

### Logging

1. **Structured Logging**
   ```go
   logger.Info("processing order",
       logging.String("order_id", orderID),
       logging.Float64("price", price),
       logging.Int("quantity", quantity),
   )
   ```

2. **Log Levels**
   ```go
   logger.Debug("websocket message received")  // Detailed debugging
   logger.Info("order placed")                 // Normal operations
   logger.Warn("rate limit approaching")       // Warning conditions
   logger.Error("connection failed")           // Error conditions
   ```

### Testing

1. **Table-Driven Tests**
   ```go
   tests := []struct {
       name     string
       input    string
       expected Result
       wantErr  bool
   }{
       // Test cases
   }
   
   for _, tt := range tests {
       t.Run(tt.name, func(t *testing.T) {
           // Test logic
       })
   }
   ```

2. **Mock Dependencies**
   ```go
   type MockConnector struct {
       mock.Mock
   }
   
   func (m *MockConnector) Connect(ctx context.Context) error {
       args := m.Called(ctx)
       return args.Error(0)
   }
   ```

### Documentation

1. **Code Comments**
   ```go
   // Connect establishes a connection to the exchange.
   // It will retry the connection according to the configured retry policy.
   // Returns an error if the connection cannot be established.
   func (c *Connector) Connect(ctx context.Context) error {
       // Implementation
   }
   ```

2. **Package Documentation**
   ```go
   // Package websocket provides a WebSocket client implementation
   // for connecting to cryptocurrency exchanges.
   package websocket
   ``` 