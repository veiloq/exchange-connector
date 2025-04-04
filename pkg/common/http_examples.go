package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/veiloq/exchange-connector/pkg/logging"
)

// DebugClientExample demonstrates how to use the debug HTTP client
func DebugClientExample() {
	// Create a debug client with default config
	client := NewDebugHTTPClient(nil)

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Make a GET request to a sample API with retry
	var resp *http.Response
	err := retry.Do(
		func() error {
			var reqErr error
			resp, reqErr = client.Get(ctx, "https://api.github.com/users/golang")
			return reqErr
		},
		retry.Attempts(3),
		retry.Delay(time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(n uint, err error) {
			fmt.Printf("Retry attempt %d due to error: %v\n", n+1, err)
		}),
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)
}

// CustomDebugClientExample demonstrates how to create a debug client with custom configuration
func CustomDebugClientExample() {
	// Create a custom debug client config
	config := &DebugClientConfig{
		ClientConfig: &ClientConfig{
			BaseURL:    "https://api.github.com",
			Timeout:    10 * time.Second,
			MaxRetries: 2,
			RetryDelay: 500 * time.Millisecond,
			// Create a zap logger with debug level
			Logger: logging.NewZapLogger(
				logging.WithDebugLevel(),
				logging.WithDevelopmentMode(),
			),
		},
		LogRequestBody:  true,
		LogResponseBody: true,
		MaxBodyLogSize:  8192, // 8KB
	}

	// Create client with custom config
	client := NewDebugHTTPClient(config)

	// Create context with trace ID for logging
	ctx := context.WithValue(context.Background(), "trace_id", "example-trace-123")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Example request body
	type searchRequest struct {
		Query string `json:"q"`
		Sort  string `json:"sort,omitempty"`
		Order string `json:"order,omitempty"`
	}

	// Make a POST request with JSON body using retry
	reqBody := searchRequest{
		Query: "language:go",
		Sort:  "stars",
		Order: "desc",
	}

	var resp *http.Response
	err := retry.Do(
		func() error {
			var reqErr error
			resp, reqErr = client.Post(ctx, "https://api.github.com/search/repositories", reqBody)
			if reqErr != nil {
				return reqErr
			}
			// Retry on non-2xx status codes
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				return fmt.Errorf("API returned status code %d: %s", resp.StatusCode, string(body))
			}
			return nil
		},
		retry.Attempts(3),
		retry.Delay(time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(n uint, err error) {
			fmt.Printf("Retry attempt %d due to error: %v\n", n+1, err)
		}),
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)
}
