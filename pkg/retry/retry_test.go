// ABOUTME: This file contains tests for the retry logic with exponential backoff.
// ABOUTME: It verifies retry behavior for rate limits, server errors, and non-retryable errors.

package retry

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
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

	start := time.Now()
	err := WithRetry(operation, 5, 10*time.Millisecond)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected retry to succeed, got error: %v", err)
	}

	if attemptCount != maxAttempts {
		t.Errorf("Expected %d attempts, got %d", maxAttempts, attemptCount)
	}

	// Verify exponential backoff occurred (should take at least 10ms + 20ms = 30ms)
	minExpectedDuration := 30 * time.Millisecond
	if duration < minExpectedDuration {
		t.Errorf("Expected backoff duration >= %v, got %v", minExpectedDuration, duration)
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

	// Verify exponential backoff (with some tolerance for timing variance)
	tolerance := 10 * time.Millisecond

	// First delay should be ~baseDelay (20ms)
	if delays[0] < baseDelay-tolerance || delays[0] > baseDelay+tolerance {
		t.Errorf("First delay expected ~%v, got %v", baseDelay, delays[0])
	}

	// Second delay should be ~2*baseDelay (40ms)
	expectedSecond := 2 * baseDelay
	if delays[1] < expectedSecond-tolerance || delays[1] > expectedSecond+tolerance {
		t.Errorf("Second delay expected ~%v, got %v", expectedSecond, delays[1])
	}

	// Third delay should be ~4*baseDelay (80ms)
	expectedThird := 4 * baseDelay
	if delays[2] < expectedThird-tolerance || delays[2] > expectedThird+tolerance {
		t.Errorf("Third delay expected ~%v, got %v", expectedThird, delays[2])
	}
}
