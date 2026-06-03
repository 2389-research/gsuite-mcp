// ABOUTME: This file implements retry logic with exponential backoff for HTTP operations.
// ABOUTME: It retries idempotent calls on rate limits (429) and server errors (5xx), not on other 4xx.

package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/api/googleapi"
)

// HTTPError is implemented by errors that carry an HTTP status code.
type HTTPError interface {
	error
	HTTPStatusCode() int
}

// WithRetry executes an IDEMPOTENT operation with exponential backoff.
//
// Only wrap idempotent calls (GET/LIST/UPDATE/DELETE). Operations that create or
// send resources must NOT be retried: a retry after a request that actually
// succeeded but whose response was lost would duplicate the side effect.
//
// It retries on 429 and 5xx, honors a server Retry-After header, adds full jitter,
// and gives up after maxRetries additional attempts.
//
// WithRetry delegates to WithRetryCtx with a background context. New call sites
// should prefer WithRetryCtx so an upstream cancellation stops the retry loop.
func WithRetry(operation func() error, maxRetries int, baseDelay time.Duration) error {
	return WithRetryCtx(context.Background(), operation, maxRetries, baseDelay)
}

// WithRetryCtx is WithRetry plus context cancellation: if ctx is cancelled while
// waiting between attempts, it stops and returns ctx.Err().
func WithRetryCtx(ctx context.Context, operation func() error, maxRetries int, baseDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == maxRetries {
			break
		}

		retryable, retryAfter := shouldRetry(err)
		if !retryable {
			return err
		}

		delay := backoffDelay(baseDelay, attempt, retryAfter)
		if delay <= 0 {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return lastErr
}

// backoffDelay returns the wait before the next attempt. A server-provided
// Retry-After wins; otherwise the delay is exponential (baseDelay * 2^attempt)
// with full jitter in [0, cap].
func backoffDelay(baseDelay time.Duration, attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	if baseDelay <= 0 {
		return 0
	}
	capped := baseDelay * time.Duration(1<<uint(attempt))
	if capped <= 0 { // overflow guard for absurd inputs
		return 0
	}
	return time.Duration(rand.Int63n(int64(capped) + 1))
}

// shouldRetry reports whether err is retryable and any Retry-After delay it carries.
func shouldRetry(err error) (bool, time.Duration) {
	statusCode, header, ok := httpStatus(err)
	if !ok {
		return false, 0
	}
	if statusCode == http.StatusTooManyRequests || (statusCode >= 500 && statusCode < 600) {
		return true, parseRetryAfter(header)
	}
	return false, 0
}

// httpStatus extracts an HTTP status (and response header, if any) from err,
// recognizing both *googleapi.Error (what the Google SDK returns) and HTTPError.
func httpStatus(err error) (int, http.Header, bool) {
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code, gErr.Header, true
	}
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.HTTPStatusCode(), nil, true
	}
	return 0, nil, false
}

// parseRetryAfter reads a Retry-After header given in integer seconds. The
// HTTP-date form is intentionally unsupported; Google APIs send seconds.
func parseRetryAfter(header http.Header) time.Duration {
	if header == nil {
		return 0
	}
	secs, err := strconv.Atoi(header.Get("Retry-After"))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// RetryableError wraps an HTTP status code as an error.
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

// NewRetryableError creates a new retryable error with the given status code.
func NewRetryableError(statusCode int, message string) *RetryableError {
	return &RetryableError{StatusCode: statusCode, Message: message}
}
