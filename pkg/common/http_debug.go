package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/veiloq/exchange-connector/pkg/logging"
	"github.com/veiloq/exchange-connector/pkg/ratelimit"
)

// DebugClientConfig holds configuration for the HTTP debug client
type DebugClientConfig struct {
	// Inherits the base client configuration
	*ClientConfig

	// Debug-specific settings
	LogRequestBody  bool
	LogResponseBody bool

	// Maximum size of request/response body to log (to avoid massive logs)
	MaxBodyLogSize int
}

// DefaultDebugConfig returns a default debug client configuration
func DefaultDebugConfig() *DebugClientConfig {
	return &DebugClientConfig{
		ClientConfig:    DefaultConfig(),
		LogRequestBody:  true,
		LogResponseBody: true,
		MaxBodyLogSize:  4096, // 4KB max by default
	}
}

// NewDebugHTTPClient creates a new HTTP client with verbose debug logging
func NewDebugHTTPClient(config *DebugClientConfig) HTTPClient {
	if config == nil {
		config = DefaultDebugConfig()
	}

	// Create zap logger with debug level if not specified
	_, isZapLogger := config.Logger.(*logging.ZapLogger)
	if !isZapLogger {
		config.Logger = logging.NewZapLogger(
			logging.WithDebugLevel(),
			logging.WithDevelopmentMode(),
		)
	}

	return &debugClient{
		client: NewHTTPClient(config.ClientConfig).(*client),
		config: config,
	}
}

// debugClient implements the HTTPClient interface with additional debug logging
type debugClient struct {
	client *client
	config *DebugClientConfig
}

// Do implements HTTPClient interface with debug logging
func (c *debugClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Log detailed request information
	c.logRequest(req)

	// Execute request using base client
	resp, err := c.client.Do(ctx, req)

	// Log detailed response or error information
	duration := time.Since(start)
	if err != nil {
		c.logError(req, err, duration)
		return nil, err
	}

	c.logResponse(req, resp, duration)
	return resp, nil
}

// Get implements HTTPClient interface
func (c *debugClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	return c.Do(ctx, req)
}

// Post implements HTTPClient interface
func (c *debugClient) Post(ctx context.Context, url string, body interface{}) (*http.Response, error) {
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
func (c *debugClient) SetRateLimit(limit ratelimit.Rate) error {
	return c.client.SetRateLimit(limit)
}

// logRequest logs detailed information about the HTTP request
func (c *debugClient) logRequest(req *http.Request) {
	logger := c.client.logger

	// Use httputil.DumpRequest to get raw request details, but limit body size
	var reqDump []byte
	var err error

	if c.config.LogRequestBody {
		// Dump with body but read body first to preserve it
		if req.Body != nil {
			bodyBytes, bodyErr := io.ReadAll(req.Body)
			if bodyErr != nil {
				logger.Warn("failed to read request body for logging",
					logging.Error(bodyErr))
			} else {
				// Truncate body if too large
				logBody := bodyBytes
				if len(bodyBytes) > c.config.MaxBodyLogSize {
					logBody = bodyBytes[:c.config.MaxBodyLogSize]
					logger.Debug("request body truncated for logging",
						logging.Int("original_size", len(bodyBytes)),
						logging.Int("logged_size", len(logBody)))
				}

				// Create dump with body
				reqDump, err = httputil.DumpRequestOut(req, false)
				if err == nil {
					// Append the body separately to avoid re-reading it
					reqDump = append(reqDump, "\r\n"...)
					reqDump = append(reqDump, logBody...)
				}

				// Restore the body
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		} else {
			reqDump, err = httputil.DumpRequestOut(req, true)
		}
	} else {
		// Dump without body
		reqDump, err = httputil.DumpRequestOut(req, false)
	}

	if err != nil {
		logger.Warn("failed to dump request for logging",
			logging.Error(err))
	}

	// Log the request details
	logger.Debug("http request",
		logging.String("method", req.Method),
		logging.String("url", req.URL.String()),
		logging.String("host", req.Host),
		logging.Int("headers", len(req.Header)),
		logging.String("dump", string(reqDump)))
}

// logResponse logs detailed information about the HTTP response
func (c *debugClient) logResponse(req *http.Request, resp *http.Response, duration time.Duration) {
	logger := c.client.logger

	// Use httputil.DumpResponse to get raw response details, but limit body size
	var respDump []byte
	var err error

	if c.config.LogResponseBody {
		// Dump with body but read body first to preserve it
		if resp.Body != nil {
			bodyBytes, bodyErr := io.ReadAll(resp.Body)
			if bodyErr != nil {
				logger.Warn("failed to read response body for logging",
					logging.Error(bodyErr))
			} else {
				// Truncate body if too large
				logBody := bodyBytes
				if len(bodyBytes) > c.config.MaxBodyLogSize {
					logBody = bodyBytes[:c.config.MaxBodyLogSize]
					logger.Debug("response body truncated for logging",
						logging.Int("original_size", len(bodyBytes)),
						logging.Int("logged_size", len(logBody)))
				}

				// Create dump with headers
				respDump, err = httputil.DumpResponse(resp, false)
				if err == nil {
					// Append the body separately
					respDump = append(respDump, "\r\n"...)
					respDump = append(respDump, logBody...)
				}

				// Restore the body
				resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		} else {
			respDump, err = httputil.DumpResponse(resp, true)
		}
	} else {
		// Dump without body
		respDump, err = httputil.DumpResponse(resp, false)
	}

	if err != nil {
		logger.Warn("failed to dump response for logging",
			logging.Error(err))
	}

	// Log the response details
	logger.Debug("http response",
		logging.String("method", req.Method),
		logging.String("url", req.URL.String()),
		logging.Int("status", resp.StatusCode),
		logging.String("status_text", resp.Status),
		logging.Int("headers", len(resp.Header)),
		logging.Duration("duration", duration),
		logging.String("dump", string(respDump)))
}

// logError logs detailed information about an HTTP error
func (c *debugClient) logError(req *http.Request, err error, duration time.Duration) {
	logger := c.client.logger

	// Log the error details
	logger.Error("http request failed",
		logging.String("method", req.Method),
		logging.String("url", req.URL.String()),
		logging.Duration("duration", duration),
		logging.Error(err))
}
