// Package ratelimit provides mechanisms for controlling the rate of operations
// such as API requests to external services. It implements configurable rate limiting
// to prevent overwhelming dependent services and to comply with external API rate limits.
//
// This package offers a flexible abstraction over third-party rate limit implementations
// while providing a consistent interface that can be used throughout the application.
// The current implementation uses Uber's rate limiter as the underlying mechanism.
//
// Architecture Integration:
//
// The ratelimit package is a core supporting component that integrates with several
// other parts of the system:
//
//   - pkg/common/http: HTTP clients use rate limiters to respect exchange API rate limits,
//     preventing request throttling and IP bans
//
//   - pkg/exchanges/interfaces: Exchange connectors configure and use rate limiters
//     based on exchange-specific requirements
//
//   - pkg/websocket: WebSocket connections may use rate limiting for connection attempts
//     and control message frequencies
//
// Key features of this package include:
//
// 1. Token bucket algorithm for smooth rate limiting
// 2. Dynamic rate limit adjustment at runtime
// 3. Context-aware waiting to support cancellation
// 4. Configurable time intervals and operation counts
//
// The design philosophy emphasizes flexibility and composability, allowing rate
// limiting to be applied at various levels of the application as needed, from
// individual API endpoints to global connection limits.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/ratelimit"
)

// Rate represents a rate limit configuration with a specified number of operations
// allowed within a given time interval.
//
// This struct is used to define rate limits in a way that is both human-readable
// and easily adjustable. It combines the number of allowed operations and the
// time window in which those operations are permitted.
type Rate struct {
	// Limit specifies the maximum number of operations allowed within the interval.
	// For example, a limit of 100 with an interval of 1 minute means 100 operations
	// are allowed per minute.
	Limit int

	// Interval defines the time duration over which the limit applies.
	// Common intervals include time.Second, time.Minute, or time.Hour.
	// The rate limiter will calculate operations per second based on this interval.
	Interval time.Duration
}

// RateLimiter defines the interface for rate limiting functionality.
// Implementations of this interface control the pace of operations by forcing
// callers to wait when necessary to comply with defined rate limits.
//
// This interface allows for different rate limiting strategies and implementations
// to be used interchangeably within the application, providing flexibility and
// ease of testing.
type RateLimiter interface {
	// Wait blocks until a token is available (i.e., an operation is permitted)
	// or the context is cancelled.
	//
	// Parameters:
	// - ctx: Context for cancellation control. If the context is cancelled
	//   while waiting, the method will return immediately with an error.
	//
	// Returns:
	// - error: nil if a token was successfully acquired, or a context-related
	//   error if the context was cancelled before a token could be acquired.
	//
	// This method should be called before performing a rate-limited operation.
	// If the rate limit has been reached, the call will block until either
	// a token becomes available or the context is cancelled.
	Wait(ctx context.Context) error

	// SetLimit updates the rate limiting configuration.
	//
	// Parameters:
	// - limit: The new Rate configuration to apply, specifying both the number
	//   of operations and the time interval.
	//
	// Returns:
	// - error: An error if the provided rate limit is invalid (e.g., non-positive
	//   limit or interval), or nil if the update was successful.
	//
	// This method allows for dynamic adjustment of rate limits at runtime,
	// which can be useful for adapting to changing conditions or requirements.
	SetLimit(limit Rate) error
}

// uberLimiter implements RateLimiter using Uber's rate limiter library.
// This implementation uses a token bucket algorithm to control the rate
// of operations.
type uberLimiter struct {
	// limiter is the underlying Uber rate limiter instance
	limiter ratelimit.Limiter

	// rate stores the current rate limit configuration
	rate Rate
}

// NewTokenBucketLimiter creates a new rate limiter using Uber's token bucket implementation.
//
// Parameters:
//   - rate: The Rate configuration specifying the number of operations allowed
//     and the time interval in which they are permitted.
//
// Returns:
// - RateLimiter: A new rate limiter instance configured with the specified rate.
//
// This function converts the provided Rate into operations per second as required
// by the underlying Uber rate limiter. For example, if rate specifies 120 operations
// per minute, it will be converted to 2 operations per second.
//
// Example usage:
//
//	// Create a rate limiter allowing 100 operations per minute
//	limiter := NewTokenBucketLimiter(Rate{
//		Limit:    100,
//		Interval: time.Minute,
//	})
//
//	// Use the limiter before making an API call
//	ctx := context.Background()
//	if err := limiter.Wait(ctx); err != nil {
//		// Handle cancellation error
//		return err
//	}
//	// Proceed with the API call
func NewTokenBucketLimiter(rate Rate) RateLimiter {
	rps := float64(rate.Limit) / rate.Interval.Seconds()
	return &uberLimiter{
		limiter: ratelimit.New(int(rps)),
		rate:    rate,
	}
}

// Wait implements the RateLimiter interface.
//
// This method blocks until a token is available from the underlying rate limiter
// or the provided context is cancelled. It ensures that operations proceed at
// a controlled pace according to the configured rate limit.
//
// Parameters:
//   - ctx: Context for cancellation control. If the context is cancelled while
//     waiting for a token, this method returns immediately with an error.
//
// Returns:
//   - error: A context cancellation error if the context is cancelled before
//     a token could be acquired, or nil if a token was successfully acquired.
//
// If the context is active, this method will call the underlying limiter's Take()
// method, which may block until a token is available according to the rate limit.
func (l *uberLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("rate limit wait cancelled: %w", ctx.Err())
	default:
		l.limiter.Take()
		return nil
	}
}

// SetLimit implements the RateLimiter interface.
//
// This method updates the rate limit configuration of the limiter. It validates
// the new rate limit settings before applying them, ensuring they are reasonable
// and won't cause unexpected behavior.
//
// Parameters:
//   - rate: The new Rate configuration to apply, specifying both the number of
//     operations and the time interval.
//
// Returns:
//   - error: An error if the provided rate configuration is invalid (e.g., the limit
//     or interval is zero or negative), or nil if the update was successful.
//
// After validation, this method creates a new underlying limiter with the updated
// rate and replaces the current one. This allows for dynamic adjustment of rate
// limits at runtime without requiring the creation of a new RateLimiter instance.
//
// Example usage:
//
//	// Adjust the rate limit based on server load
//	if serverLoad > highThreshold {
//		// Reduce the rate to protect the system
//		limiter.SetLimit(Rate{Limit: 50, Interval: time.Minute})
//	} else if serverLoad < lowThreshold {
//		// Increase the rate for better throughput
//		limiter.SetLimit(Rate{Limit: 200, Interval: time.Minute})
//	}
func (l *uberLimiter) SetLimit(rate Rate) error {
	if rate.Limit <= 0 || rate.Interval <= 0 {
		return fmt.Errorf("invalid rate limit: %+v", rate)
	}
	rps := float64(rate.Limit) / rate.Interval.Seconds()
	l.limiter = ratelimit.New(int(rps))
	l.rate = rate
	return nil
}
