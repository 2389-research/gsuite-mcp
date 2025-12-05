// ABOUTME: This file implements retry logic with exponential backoff for HTTP operations.
// ABOUTME: It retries on rate limits (429), server errors (5xx), but not on client errors (4xx except 429).

package retry

import (
	"fmt"
	"time"
)

// HTTPError interface for errors that have an HTTP status code
type HTTPError interface {
	error
	HTTPStatusCode() int
}

// WithRetry executes an operation with exponential backoff retry logic
// It retries on:
// - 429 (Rate Limit)
// - 500 (Internal Server Error)
// - 503 (Service Unavailable)
// It does NOT retry on:
// - Other 4xx errors (client errors like 400, 401, 403, 404)
// - Non-HTTP errors
//
// Parameters:
// - operation: the function to retry
// - maxRetries: maximum number of retry attempts (not including the initial attempt)
// - baseDelay: initial delay between retries (doubles each attempt)
//
// Returns the error from the last attempt if all retries are exhausted
func WithRetry(operation func() error, maxRetries int, baseDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Execute the operation
		err := operation()

		// Success case
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if this is the last attempt
		if attempt == maxRetries {
			break
		}

		// Determine if we should retry
		if !shouldRetry(err) {
			return err
		}

		// Calculate delay with exponential backoff
		delay := baseDelay * time.Duration(1<<uint(attempt))
		time.Sleep(delay)
	}

	return lastErr
}

// shouldRetry determines if an error is retryable
func shouldRetry(err error) bool {
	// Check if it's an HTTP error
	httpErr, ok := err.(HTTPError)
	if !ok {
		// Not an HTTP error, don't retry
		return false
	}

	statusCode := httpErr.HTTPStatusCode()

	// Retry on rate limits
	if statusCode == 429 {
		return true
	}

	// Retry on server errors (5xx)
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	// Don't retry on client errors (4xx except 429)
	if statusCode >= 400 && statusCode < 500 {
		return false
	}

	// Don't retry on other status codes
	return false
}

// RetryableError wraps an HTTP status code as an error
type RetryableError struct {
	StatusCode int
	Message    string
}

func (e *RetryableError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("HTTP %d error", e.StatusCode)
}

func (e *RetryableError) HTTPStatusCode() int {
	return e.StatusCode
}

// NewRetryableError creates a new retryable error with the given status code
func NewRetryableError(statusCode int, message string) *RetryableError {
	return &RetryableError{
		StatusCode: statusCode,
		Message:    message,
	}
}
