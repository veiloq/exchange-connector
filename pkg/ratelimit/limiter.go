package ratelimit

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/ratelimit"
)

// Rate represents a rate limit configuration
type Rate struct {
	Limit    int           // number of requests
	Interval time.Duration // time interval
}

// RateLimiter defines the interface for rate limiting functionality
type RateLimiter interface {
	// Wait blocks until a token is available or the context is cancelled
	Wait(ctx context.Context) error
	// SetLimit updates the rate limit
	SetLimit(limit Rate) error
}

// uberLimiter implements RateLimiter using Uber's rate limiter
type uberLimiter struct {
	limiter ratelimit.Limiter
	rate    Rate
}

// NewTokenBucketLimiter creates a new rate limiter using Uber's implementation
func NewTokenBucketLimiter(rate Rate) RateLimiter {
	rps := float64(rate.Limit) / rate.Interval.Seconds()
	return &uberLimiter{
		limiter: ratelimit.New(int(rps)),
		rate:    rate,
	}
}

// Wait implements RateLimiter interface
func (l *uberLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("rate limit wait cancelled: %w", ctx.Err())
	default:
		l.limiter.Take()
		return nil
	}
}

// SetLimit implements RateLimiter interface
func (l *uberLimiter) SetLimit(rate Rate) error {
	if rate.Limit <= 0 || rate.Interval <= 0 {
		return fmt.Errorf("invalid rate limit: %+v", rate)
	}
	rps := float64(rate.Limit) / rate.Interval.Seconds()
	l.limiter = ratelimit.New(int(rps))
	l.rate = rate
	return nil
}
