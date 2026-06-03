// ABOUTME: This file contains tests for the retry logic with exponential backoff.
// ABOUTME: It verifies retry behavior for rate limits, server errors, and non-retryable errors.

package retry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
)

// mockHTTPError simulates HTTP errors with status codes
type mockHTTPError struct {
	StatusCode int
}

func (e *mockHTTPError) Error() string {
	return fmt.Sprintf("HTTP %d error", e.StatusCode)
}

func (e *mockHTTPError) HTTPStatusCode() int {
	return e.StatusCode
}

// TestRetrySucceedsAfterFailures tests that retry succeeds after temporary failures (429)
func TestRetrySucceedsAfterFailures(t *testing.T) {
	attemptCount := 0
	maxAttempts := 3

	operation := func() error {
		attemptCount++
		if attemptCount < maxAttempts {
			return &mockHTTPError{StatusCode: http.StatusTooManyRequests} // 429
		}
		return nil // Success on third attempt
	}

	err := WithRetry(operation, 5, 10*time.Millisecond)

	if err != nil {
		t.Errorf("Expected retry to succeed, got error: %v", err)
	}

	if attemptCount != maxAttempts {
		t.Errorf("Expected %d attempts, got %d", maxAttempts, attemptCount)
	}
}

// TestRetryExhaustsAttempts tests that retry gives up after max attempts
func TestRetryExhaustsAttempts(t *testing.T) {
	attemptCount := 0
	maxRetries := 3

	operation := func() error {
		attemptCount++
		return &mockHTTPError{StatusCode: http.StatusServiceUnavailable} // 503 - always fails
	}

	err := WithRetry(operation, maxRetries, 5*time.Millisecond)

	if err == nil {
		t.Error("Expected retry to fail after exhausting attempts, got nil error")
	}

	expectedAttempts := maxRetries + 1 // initial attempt + retries
	if attemptCount != expectedAttempts {
		t.Errorf("Expected %d attempts, got %d", expectedAttempts, attemptCount)
	}
}

// TestNoRetryOn4xxExcept429 tests that we don't retry on 4xx errors except 429
func TestNoRetryOn4xxExcept429(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		shouldRetry bool
	}{
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"429 Rate Limit", http.StatusTooManyRequests, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attemptCount := 0

			operation := func() error {
				attemptCount++
				return &mockHTTPError{StatusCode: tc.statusCode}
			}

			err := WithRetry(operation, 3, 5*time.Millisecond)

			if err == nil {
				t.Error("Expected error, got nil")
			}

			if tc.shouldRetry {
				// Should make multiple attempts for 429
				if attemptCount <= 1 {
					t.Errorf("Expected multiple attempts for status %d, got %d", tc.statusCode, attemptCount)
				}
			} else {
				// Should NOT retry for other 4xx errors
				if attemptCount != 1 {
					t.Errorf("Expected exactly 1 attempt for status %d, got %d", tc.statusCode, attemptCount)
				}
			}
		})
	}
}

// TestRetryOn5xxErrors tests that we retry on server errors
func TestRetryOn5xxErrors(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attemptCount := 0

			operation := func() error {
				attemptCount++
				return &mockHTTPError{StatusCode: tc.statusCode}
			}

			err := WithRetry(operation, 2, 5*time.Millisecond)

			if err == nil {
				t.Error("Expected error, got nil")
			}

			expectedAttempts := 3 // initial + 2 retries
			if attemptCount != expectedAttempts {
				t.Errorf("Expected %d attempts for status %d, got %d", expectedAttempts, tc.statusCode, attemptCount)
			}
		})
	}
}

// TestNoRetryOnNonHTTPError tests that non-HTTP errors are not retried
func TestNoRetryOnNonHTTPError(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		return errors.New("some non-HTTP error")
	}

	err := WithRetry(operation, 3, 5*time.Millisecond)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attemptCount != 1 {
		t.Errorf("Expected exactly 1 attempt for non-HTTP error, got %d", attemptCount)
	}
}

// TestExponentialBackoff verifies that delays double each attempt
func TestExponentialBackoff(t *testing.T) {
	attemptCount := 0
	delays := []time.Duration{}
	lastAttempt := time.Now()

	operation := func() error {
		attemptCount++
		if attemptCount > 1 {
			delay := time.Since(lastAttempt)
			delays = append(delays, delay)
		}
		lastAttempt = time.Now()

		if attemptCount < 4 {
			return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
		}
		return nil
	}

	baseDelay := 20 * time.Millisecond
	err := WithRetry(operation, 5, baseDelay)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if len(delays) != 3 {
		t.Fatalf("Expected 3 delays, got %d", len(delays))
	}

	// Full jitter means each delay is in [0, cap]; assert the (jittered) upper bound.
	tolerance := 10 * time.Millisecond
	caps := []time.Duration{baseDelay, 2 * baseDelay, 4 * baseDelay} // 20ms, 40ms, 80ms
	for i, d := range delays {
		if d < 0 || d > caps[i]+tolerance {
			t.Errorf("delay %d = %v, want within [0, %v]", i, d, caps[i]+tolerance)
		}
	}
}

// TestRetryOnGoogleAPIError5xx verifies that a real *googleapi.Error is retried on 5xx.
func TestRetryOnGoogleAPIError5xx(t *testing.T) {
	attempts := 0
	op := func() error {
		attempts++
		if attempts < 3 {
			return &googleapi.Error{Code: http.StatusServiceUnavailable}
		}
		return nil
	}
	if err := WithRetry(op, 5, time.Millisecond); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// TestNoRetryOnGoogleAPIError4xx verifies that a *googleapi.Error 4xx is not retried.
func TestNoRetryOnGoogleAPIError4xx(t *testing.T) {
	attempts := 0
	op := func() error {
		attempts++
		return &googleapi.Error{Code: http.StatusForbidden}
	}
	if err := WithRetry(op, 5, time.Millisecond); err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retry on 403), got %d", attempts)
	}
}

// TestRetryAfterHeaderHonored verifies that a Retry-After header (integer seconds) is honored.
func TestRetryAfterHeaderHonored(t *testing.T) {
	if got := parseRetryAfter(http.Header{"Retry-After": []string{"2"}}); got != 2*time.Second {
		t.Fatalf("expected 2s, got %v", got)
	}
	if got := parseRetryAfter(http.Header{"Retry-After": []string{"0"}}); got != 0 {
		t.Fatalf("expected 0 for non-positive, got %v", got)
	}
	if got := parseRetryAfter(nil); got != 0 {
		t.Fatalf("expected 0 for nil header, got %v", got)
	}
}

// TestWithRetryCtxCancels verifies that WithRetryCtx aborts between attempts when ctx is cancelled.
func TestWithRetryCtxCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	op := func() error {
		attempts++
		if attempts == 1 {
			cancel() // cancel during the first backoff wait
		}
		return &googleapi.Error{Code: http.StatusServiceUnavailable}
	}
	err := WithRetryCtx(ctx, op, 10, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected context error")
	}
	if attempts > 2 {
		t.Fatalf("expected to stop early on cancel, got %d attempts", attempts)
	}
}
