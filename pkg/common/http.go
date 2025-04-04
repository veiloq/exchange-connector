// Package common provides utilities and shared functionality used across the application.
// This package contains reusable components that don't fit into more specific packages,
// including HTTP clients, error handling utilities, and common data structures.
//
// The HTTP utilities in this package offer robust HTTP client implementations with
// built-in support for retries, rate limiting, and observability.
//
// Architecture Integration:
//
// The common package, particularly its HTTP components, integrates with other key parts
// of the system:
//
//   - pkg/exchanges/interfaces: Exchange connectors use this HTTP client for all REST API
//     communication with exchanges, complementing the WebSocket-based real-time data
//
//   - pkg/ratelimit: HTTP clients leverage the rate limiting package to enforce API
//     rate limits, preventing throttling or bans from exchanges
//
//   - pkg/logging: HTTP clients use structured logging to record request/response details,
//     errors, and performance metrics
//
//   - pkg/websocket: Works alongside the HTTP client to provide complete exchange
//     connectivity (HTTP for REST API, WebSocket for real-time streaming)
//
// The HTTP client implementation features:
//
// 1. Automatic retries with exponential backoff for transient failures
// 2. Context-based cancellation for timeout and request lifecycle management
// 3. Rate limiting to respect exchange API constraints
// 4. Detailed logging of requests, responses, and errors
// 5. Convenient methods for common operations (GET, POST with JSON)
//
// This modular design allows exchange-specific connectors to focus on their API
// peculiarities rather than the mechanics of making reliable HTTP requests.
package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/veiloq/exchange-connector/pkg/logging"
	"github.com/veiloq/exchange-connector/pkg/ratelimit"
)

// HTTPClient defines the interface for making HTTP requests with advanced features
// like automatic retries, rate limiting, and context cancellation support.
//
// This interface abstracts the details of HTTP communication and provides a consistent
// way to interact with HTTP-based APIs throughout the application.
type HTTPClient interface {
	// Do executes an HTTP request with retries and rate limiting.
	//
	// Parameters:
	// - ctx: Context for timeout and cancellation control
	// - req: The HTTP request to execute
	//
	// Returns:
	// - *http.Response: The HTTP response if successful
	// - error: An error if the request fails after all retry attempts
	//
	// The method respects rate limits and implements retry logic for transient failures.
	// The context can be used to cancel long-running requests or control timeouts.
	Do(ctx context.Context, req *http.Request) (*http.Response, error)

	// Get is a convenience method for making GET requests.
	//
	// Parameters:
	// - ctx: Context for timeout and cancellation control
	// - url: The URL to request
	//
	// Returns:
	// - *http.Response: The HTTP response if successful
	// - error: An error if the request fails
	//
	// This method creates and executes a GET request to the specified URL.
	Get(ctx context.Context, url string) (*http.Response, error)

	// Post is a convenience method for making POST requests with JSON body.
	//
	// Parameters:
	// - ctx: Context for timeout and cancellation control
	// - url: The URL to request
	// - body: Object to be JSON-encoded and sent as the request body
	//
	// Returns:
	// - *http.Response: The HTTP response if successful
	// - error: An error if the request fails
	//
	// This method creates a POST request with JSON content type, serializes the body,
	// and executes the request.
	Post(ctx context.Context, url string, body interface{}) (*http.Response, error)

	// SetRateLimit updates the rate limiter configuration.
	//
	// Parameters:
	// - limit: The new rate limit configuration
	//
	// Returns:
	// - error: An error if the rate limit cannot be updated
	//
	// This method allows dynamically adjusting the rate limits based on
	// application needs or API constraints.
	SetRateLimit(limit ratelimit.Rate) error
}

// ClientConfig holds configuration for the HTTP client.
// This structure allows customizing the behavior of HTTP clients
// to meet specific requirements for different APIs or use cases.
type ClientConfig struct {
	// Base configuration

	// BaseURL is the base URL prefix for all requests
	BaseURL string

	// Timeout defines the maximum duration for HTTP requests
	Timeout time.Duration

	// RateLimit controls how many requests can be made in a time interval
	RateLimit ratelimit.Rate

	// Retry configuration

	// MaxRetries defines how many times to retry failed requests
	MaxRetries uint

	// RetryDelay specifies the initial delay between retry attempts
	RetryDelay time.Duration

	// Optional logger

	// Logger is used for recording request/response information and errors
	Logger logging.Logger
}

// DefaultConfig returns a default client configuration with reasonable values.
//
// Returns:
// - *ClientConfig: A configuration structure with pre-populated default values
//
// The default configuration includes:
// - 30 second timeout
// - Rate limit of 10 requests per second
// - 3 retry attempts with 1 second delay
// - Default logger instance
//
// This function can be used as a starting point for creating client configurations,
// which can then be customized as needed.
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

// client implements the HTTPClient interface.
// This structure maintains the HTTP client configuration, rate limiter,
// and other components needed for HTTP communication.
type client struct {
	config     *ClientConfig
	httpClient *http.Client
	limiter    ratelimit.RateLimiter
	logger     logging.Logger
}

// NewHTTPClient creates a new HTTP client with the given configuration.
//
// Parameters:
// - config: Configuration for the HTTP client. If nil, default configuration is used.
//
// Returns:
// - HTTPClient: A configured HTTP client ready for use
//
// This function initializes an HTTP client with the specified configuration,
// creating the underlying http.Client, rate limiter, and other components.
//
// Example usage:
//
//	config := &common.ClientConfig{
//		Timeout: 10 * time.Second,
//		RateLimit: ratelimit.Rate{
//			Limit:    20,
//			Interval: time.Second,
//		},
//		MaxRetries: 5,
//	}
//
//	client := common.NewHTTPClient(config)
//
//	resp, err := client.Get(context.Background(), "https://api.exchange.com/markets")
//	if err != nil {
//		log.Fatalf("Request failed: %v", err)
//	}
//	defer resp.Body.Close()
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

// Do implements the HTTPClient interface.
//
// This method executes an HTTP request with rate limiting and retry logic.
// It respects the context for cancellation, clones the request for retries,
// and applies the configured retry policy for failed requests.
//
// Parameters:
// - ctx: Context for controlling request lifetime
// - req: The HTTP request to execute
//
// Returns:
// - *http.Response: The HTTP response if successful
// - error: An error if the request fails after all retry attempts
//
// The method will retry on server errors (5xx) and rate limit errors (429).
// Each retry respects the rate limiter to avoid overwhelming the server.
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

// Get implements the HTTPClient interface.
//
// This convenience method creates and executes a GET request to the specified URL.
// It handles the creation of the request object and delegates to the Do method
// for execution with all the configured behaviors.
//
// Parameters:
// - ctx: Context for controlling request lifetime
// - url: The URL to request
//
// Returns:
// - *http.Response: The HTTP response if successful
// - error: An error if the request fails
func (c *client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	return c.Do(ctx, req)
}

// Post implements the HTTPClient interface.
//
// This convenience method creates and executes a POST request with a JSON body.
// It serializes the provided object to JSON, sets the appropriate content type header,
// and executes the request with all the configured behaviors.
//
// Parameters:
// - ctx: Context for controlling request lifetime
// - url: The URL to request
// - body: Object to be JSON-encoded and sent as the request body
//
// Returns:
// - *http.Response: The HTTP response if successful
// - error: An error if the request fails
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

// SetRateLimit implements the HTTPClient interface.
//
// This method updates the rate limiter configuration to adjust how many
// requests can be made in a given time period. This allows for dynamic
// adjustment of request rates based on changing requirements or server capacity.
//
// Parameters:
// - limit: The new rate limit configuration
//
// Returns:
// - error: An error if the rate limit cannot be updated
func (c *client) SetRateLimit(limit ratelimit.Rate) error {
	return c.limiter.SetLimit(limit)
}
