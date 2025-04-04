package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/avast/retry-go"
	"github.com/bybit/pkg/logging"
	"github.com/bybit/pkg/ratelimit"
)

// HTTPClient defines the interface for making HTTP requests
type HTTPClient interface {
	// Do executes an HTTP request with retries and rate limiting
	Do(ctx context.Context, req *http.Request) (*http.Response, error)

	// Get is a convenience method for making GET requests
	Get(ctx context.Context, url string) (*http.Response, error)

	// Post is a convenience method for making POST requests with JSON body
	Post(ctx context.Context, url string, body interface{}) (*http.Response, error)

	// SetRateLimit updates the rate limiter configuration
	SetRateLimit(limit ratelimit.Rate) error
}

// ClientConfig holds configuration for the HTTP client
type ClientConfig struct {
	// Base configuration
	BaseURL   string
	Timeout   time.Duration
	RateLimit ratelimit.Rate

	// Retry configuration
	MaxRetries uint
	RetryDelay time.Duration

	// Optional logger
	Logger logging.Logger
}

// DefaultConfig returns a default client configuration
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		Timeout: 30 * time.Second,
		RateLimit: ratelimit.Rate{
			Limit:    10,
			Interval: time.Second,
		},
		MaxRetries: 3,
		RetryDelay: time.Second,
		Logger:     logging.NewLogger(),
	}
}

// client implements the HTTPClient interface
type client struct {
	config     *ClientConfig
	httpClient *http.Client
	limiter    ratelimit.RateLimiter
	logger     logging.Logger
}

// NewHTTPClient creates a new HTTP client with the given configuration
func NewHTTPClient(config *ClientConfig) HTTPClient {
	if config == nil {
		config = DefaultConfig()
	}

	return &client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		limiter: ratelimit.NewTokenBucketLimiter(config.RateLimit),
		logger:  config.Logger,
	}
}

// Do implements HTTPClient interface
func (c *client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response

	// Wait for rate limit token
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait error: %w", err)
	}

	// Execute request with retries
	err := retry.Do(
		func() error {
			var err error

			// Clone request to ensure it can be retried
			reqClone := req.Clone(ctx)
			if req.Body != nil {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return fmt.Errorf("error reading request body: %w", err)
				}
				reqClone.Body = io.NopCloser(bytes.NewReader(body))
				req.Body = io.NopCloser(bytes.NewReader(body))
			}

			// Execute request
			resp, err = c.httpClient.Do(reqClone)
			if err != nil {
				return fmt.Errorf("http request error: %w", err)
			}

			// Check for retryable status codes
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				return fmt.Errorf("retryable status code: %d", resp.StatusCode)
			}

			return nil
		},
		retry.Attempts(c.config.MaxRetries),
		retry.Delay(c.config.RetryDelay),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			c.logger.Warn("retrying request",
				logging.Int("attempt", int(n)),
				logging.String("url", req.URL.String()),
				logging.Error(err),
			)
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.config.MaxRetries, err)
	}

	return resp, nil
}

// Get implements HTTPClient interface
func (c *client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	return c.Do(ctx, req)
}

// Post implements HTTPClient interface
func (c *client) Post(ctx context.Context, url string, body interface{}) (*http.Response, error) {
	// Marshal body to JSON
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return c.Do(ctx, req)
}

// SetRateLimit implements HTTPClient interface
func (c *client) SetRateLimit(limit ratelimit.Rate) error {
	return c.limiter.SetLimit(limit)
}
