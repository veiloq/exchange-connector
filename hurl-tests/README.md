# WebSocket Connector Hurl Tests

This directory contains Hurl test files for testing the WebSocket connector package functionality.

## What is Hurl?

[Hurl](https://hurl.dev/) is a command-line tool for running HTTP requests defined in simple text files. It's useful for API testing and verification.

## Testing WebSockets with Hurl

Since Hurl is primarily designed for HTTP, not WebSockets, these tests simulate WebSocket operations using HTTP REST endpoints. In a real environment, you would need to:

1. Run a REST API service that provides WebSocket management endpoints
2. Use these endpoints to control and monitor WebSocket connections
3. Execute the Hurl tests against that REST API

## Test File to Method Mapping

| Hurl File | WebSocket Method | Description |
|-----------|------------------|-------------|
| [connect.hurl](connector/connect.hurl) | `Connect(ctx context.Context) error` | Tests establishing a WebSocket connection |
| [close.hurl](connector/close.hurl) | `Close() error` | Tests closing a WebSocket connection |
| [subscribe.hurl](connector/subscribe.hurl) | `Subscribe(topic string, handler MessageHandler) error` | Tests subscribing to WebSocket topics |
| [unsubscribe.hurl](connector/unsubscribe.hurl) | `Unsubscribe(topic string) error` | Tests unsubscribing from WebSocket topics |
| [send.hurl](connector/send.hurl) | `Send(message interface{}) error` | Tests sending messages through WebSocket |
| [health_check.hurl](connector/health_check.hurl) | `HealthCheck() error` | Tests checking the health of WebSocket connections |
| [get_metrics.hurl](connector/get_metrics.hurl) | `GetMetrics() Metrics` | Tests retrieving metrics from WebSocket connections |

## Exchange-Specific Examples

- [Bybit Exchange](examples/bybit.hurl) - Tests for Bybit WebSocket API integration
- [Binance Exchange](examples/binance.hurl) - Tests for Binance WebSocket API integration

## Running the Tests

### Setup

1. Install Hurl:

```bash
# With Homebrew (macOS/Linux)
brew install hurl

# With apt (Debian/Ubuntu)
sudo apt install hurl
```

2. Set up environment variables:

```bash
# Create a .env file with your API credentials
cp .env.example .env
# Edit .env with your API keys
```

3. Run the tests:

```bash
# Run all tests
hurl --variables-file .env hurl-tests/connector/*.hurl

# Run a specific test
hurl --variables-file .env hurl-tests/connector/connect.hurl

# Run exchange-specific tests
hurl --variables-file .env hurl-tests/examples/bybit.hurl
```

## Creating a Local Test Environment

For development testing without connecting to real exchanges, you can use a mock WebSocket server:

```bash
# Install wscat for WebSocket testing
npm install -g wscat

# Start a local WebSocket echo server
npm install -g ws
ws -p 8080

# Connect to the WebSocket server
wscat -c ws://localhost:8080
```

This allows you to manually test WebSocket functionality before running automated tests. 