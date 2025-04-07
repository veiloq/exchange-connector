package interfaces

import (
	"errors"
	"fmt"
)

// Common error variables that exchange connectors may return
var (
	// ErrNotConnected is returned when an operation is attempted on a connector
	// that hasn't been connected yet or lost connection
	ErrNotConnected = errors.New("exchange connector not connected: please ensure a successful connection before performing operations")

	// ErrInvalidSymbol is returned when an invalid trading pair symbol is provided
	ErrInvalidSymbol = errors.New("invalid trading pair symbol: verify that the symbol is supported by the exchange")

	// ErrInvalidInterval is returned when an unsupported time interval is provided
	ErrInvalidInterval = errors.New("invalid time interval: please choose a valid interval option supported by the exchange")

	// ErrInvalidTimeRange is returned when an invalid time range is provided
	// (e.g., end time before start time)
	ErrInvalidTimeRange = errors.New("invalid time range: start time must be earlier than end time")

	// ErrRateLimitExceeded is returned when the exchange rate limit is exceeded
	ErrRateLimitExceeded = errors.New("exchange rate limit exceeded: please wait or retry after the specified cooldown period")

	// ErrAuthenticationRequired is returned when attempting an operation that requires
	// authentication without providing credentials
	ErrAuthenticationRequired = errors.New("authentication required for this operation: please provide valid credentials before proceeding")

	// ErrInvalidCredentials is returned when the provided API credentials are invalid
	ErrInvalidCredentials = errors.New("invalid API credentials: check your API keys or passphrase to ensure they are correct")

	// ErrSubscriptionFailed is returned when a WebSocket subscription cannot be established
	ErrSubscriptionFailed = errors.New("failed to establish subscription: please check your network connection or subscription parameters")

	// ErrSubscriptionNotFound is returned when trying to unsubscribe from a non-existent subscription
	ErrSubscriptionNotFound = errors.New("subscription not found: ensure you are referencing an active subscription before unsubscribing")

	// ErrExchangeUnavailable is returned when the exchange API is unavailable
	ErrExchangeUnavailable = errors.New("exchange API unavailable: the service might be down or under maintenance, please retry later")
)

// MarketError represents a market-specific error condition
type MarketError struct {
	Symbol  string
	Message string
	Err     error
}

// Error implements the error interface
func (e *MarketError) Error() string {
	return fmt.Sprintf("market error for %s: %s (underlying error: %v)", e.Symbol, e.Message, e.Err)
}

// Unwrap returns the underlying error
func (e *MarketError) Unwrap() error {
	return e.Err
}

// NewMarketError creates a new market-specific error
func NewMarketError(symbol, message string, err error) error {
	return &MarketError{
		Symbol:  symbol,
		Message: message,
		Err:     err,
	}
}
