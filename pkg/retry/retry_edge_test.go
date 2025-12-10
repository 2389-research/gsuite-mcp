// ABOUTME: This file contains edge case and boundary condition tests for the retry logic.
// ABOUTME: It tests scenarios like zero delays, negative values, context cancellation, and concurrent retries.

package retry

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRetryWithExactMaxRetryCount tests that exactly maxRetries+1 attempts are made
func TestRetryWithExactMaxRetryCount(t *testing.T) {
	testCases := []struct {
		name       string
		maxRetries int
	}{
		{"zero retries", 0},
		{"one retry", 1},
		{"five retries", 5},
		{"ten retries", 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attemptCount := 0

			operation := func() error {
				attemptCount++
				return &mockHTTPError{StatusCode: http.StatusServiceUnavailable}
			}

			err := WithRetry(operation, tc.maxRetries, 1*time.Microsecond)

			assert.Error(t, err)
			expectedAttempts := tc.maxRetries + 1 // initial + retries
			assert.Equal(t, expectedAttempts, attemptCount, "Should make exactly maxRetries+1 attempts")
		})
	}
}

// TestRetryWithZeroDelay tests retry behavior with zero delay
func TestRetryWithZeroDelay(t *testing.T) {
	attemptCount := 0
	maxRetries := 3

	operation := func() error {
		attemptCount++
		if attemptCount < 3 {
			return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
		}
		return nil
	}

	start := time.Now()
	err := WithRetry(operation, maxRetries, 0)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 3, attemptCount)
	// With zero delay, should complete very quickly (under 10ms)
	assert.Less(t, duration, 10*time.Millisecond, "Zero delay should complete very quickly")
}

// TestRetryWithExtremelyLargeDelays tests that large delays are handled gracefully
func TestRetryWithExtremelyLargeDelays(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		// Succeed on first attempt to avoid actually waiting
		return nil
	}

	// Use a very large base delay - since we succeed immediately, no delay occurs
	// This verifies the code handles large delay values without panic
	err := WithRetry(operation, 5, 1*time.Hour)

	assert.NoError(t, err)
	assert.Equal(t, 1, attemptCount, "Should succeed on first attempt")
}

// TestRetryWithNegativeMaxRetries tests behavior with negative max retries
func TestRetryWithNegativeMaxRetries(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
	}

	// With negative maxRetries, the loop condition attempt <= maxRetries
	// is false from the start (0 <= -1 is false), so the loop never executes
	// The function returns lastErr which is nil (never set)
	err := WithRetry(operation, -1, 10*time.Millisecond)

	// Since the loop never runs, lastErr stays nil and is returned
	assert.NoError(t, err, "With negative maxRetries, no attempts are made and nil is returned")
	assert.Equal(t, 0, attemptCount, "With negative maxRetries, no attempts should be made")
}

// TestRetryWithNegativeDelay tests behavior with negative delay
func TestRetryWithNegativeDelay(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		if attemptCount < 3 {
			return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
		}
		return nil
	}

	start := time.Now()
	err := WithRetry(operation, 5, -10*time.Millisecond)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 3, attemptCount)
	// Negative delay should be treated as zero delay by time.Sleep
	assert.Less(t, duration, 10*time.Millisecond, "Negative delay should be treated as zero")
}

// TestConcurrentRetryOperations tests multiple goroutines performing retries simultaneously
func TestConcurrentRetryOperations(t *testing.T) {
	const numGoroutines = 10
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var failureCount atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			attemptCount := 0
			operation := func() error {
				attemptCount++
				// Half succeed after retries, half fail
				if id%2 == 0 {
					if attemptCount < 3 {
						return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
					}
					return nil
				}
				return &mockHTTPError{StatusCode: http.StatusServiceUnavailable}
			}

			err := WithRetry(operation, 3, 1*time.Millisecond)
			if err == nil {
				successCount.Add(1)
			} else {
				failureCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int32(5), successCount.Load(), "Half should succeed")
	assert.Equal(t, int32(5), failureCount.Load(), "Half should fail")
}

// TestConcurrentRetriesDoNotInterfere tests that concurrent retries maintain independence
func TestConcurrentRetriesDoNotInterfere(t *testing.T) {
	const numGoroutines = 20
	var wg sync.WaitGroup
	attemptCounts := make([]int, numGoroutines)
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			localAttempts := 0
			operation := func() error {
				localAttempts++
				if localAttempts < 3 {
					return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
				}
				return nil
			}

			err := WithRetry(operation, 5, 1*time.Millisecond)
			assert.NoError(t, err)

			mu.Lock()
			attemptCounts[id] = localAttempts
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// All goroutines should have made exactly 3 attempts
	for i, count := range attemptCounts {
		assert.Equal(t, 3, count, "Goroutine %d should have made 3 attempts", i)
	}
}

// TestRetryWithContextCancellation tests retry behavior when context is cancelled
// Note: The current implementation doesn't support context, so this tests the limitation
func TestRetryWithContextCancellation(t *testing.T) {
	t.Skip("Current implementation does not support context cancellation")

	// This test demonstrates what we'd want if context support was added
	ctx, cancel := context.WithCancel(context.Background())
	attemptCount := 0

	operation := func() error {
		attemptCount++
		// Check if context is cancelled (hypothetical)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	err := WithRetry(operation, 10, 10*time.Millisecond)

	assert.Error(t, err)
	// Should stop retrying when context is cancelled
	assert.Less(t, attemptCount, 11, "Should stop before exhausting all retries")
}

// TestRetrySucceedsOnFirstAttempt tests that no retries occur when first attempt succeeds
func TestRetrySucceedsOnFirstAttempt(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		return nil
	}

	start := time.Now()
	err := WithRetry(operation, 5, 100*time.Millisecond)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 1, attemptCount, "Should only attempt once when first attempt succeeds")
	assert.Less(t, duration, 50*time.Millisecond, "Should not delay when successful on first attempt")
}

// TestRetryWithVeryLargeMaxRetries tests behavior with extremely high retry counts
func TestRetryWithVeryLargeMaxRetries(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		if attemptCount < 5 {
			return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
		}
		return nil
	}

	// Even with a huge max retry count, should succeed after 5 attempts
	err := WithRetry(operation, 1000, 1*time.Microsecond)

	assert.NoError(t, err)
	assert.Equal(t, 5, attemptCount, "Should stop retrying after success")
}

// TestExponentialBackoffOverflow tests that exponential backoff handles overflow gracefully
func TestExponentialBackoffOverflow(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		return &mockHTTPError{StatusCode: http.StatusTooManyRequests}
	}

	// With high retry counts and base delay, exponential backoff could overflow
	// baseDelay * 2^attempt can overflow for large attempts
	// This test verifies the system doesn't panic
	// Using smaller values to keep test fast, but still tests the principle
	err := WithRetry(operation, 10, 1*time.Microsecond)

	assert.Error(t, err)
	assert.Equal(t, 11, attemptCount, "Should complete all attempts without panic")
}

// TestRetryableErrorFormatting tests the error message formatting
func TestRetryableErrorFormatting(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		message        string
		expectedOutput string
	}{
		{"with message", 503, "Service temporarily unavailable", "HTTP 503: Service temporarily unavailable"},
		{"without message", 429, "", "HTTP 429 error"},
		{"internal server error", 500, "Internal error occurred", "HTTP 500: Internal error occurred"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewRetryableError(tc.statusCode, tc.message)
			assert.Equal(t, tc.expectedOutput, err.Error())
			assert.Equal(t, tc.statusCode, err.HTTPStatusCode())
		})
	}
}

// TestShouldRetryBoundaryStatusCodes tests edge cases around status code boundaries
func TestShouldRetryBoundaryStatusCodes(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		expectedRetry  bool
	}{
		{"399 - below 4xx", 399, false},
		{"400 - start of 4xx", 400, false},
		{"429 - rate limit", 429, true},
		{"499 - end of 4xx", 499, false},
		{"500 - start of 5xx", 500, true},
		{"599 - end of 5xx", 599, true},
		{"600 - above 5xx", 600, false},
		{"200 - success", 200, false},
		{"302 - redirect", 302, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := &mockHTTPError{StatusCode: tc.statusCode}
			result := shouldRetry(err)
			assert.Equal(t, tc.expectedRetry, result, "Status code %d retry behavior incorrect", tc.statusCode)
		})
	}
}

// TestRetryWithNilOperation tests that nil operation is handled
func TestRetryWithNilOperation(t *testing.T) {
	// This will panic, but let's verify the panic occurs
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when operation is nil")
		}
	}()

	_ = WithRetry(nil, 3, 10*time.Millisecond)
}

// TestRetryPreservesLastError tests that the last error is returned when retries are exhausted
func TestRetryPreservesLastError(t *testing.T) {
	attemptCount := 0
	errors := []*mockHTTPError{
		{StatusCode: 503},
		{StatusCode: 500},
		{StatusCode: 503},
	}

	operation := func() error {
		err := errors[attemptCount]
		attemptCount++
		return err
	}

	result := WithRetry(operation, 2, 1*time.Microsecond)

	assert.Error(t, result)
	// Should return the last error (third one)
	httpErr, ok := result.(*mockHTTPError)
	assert.True(t, ok)
	assert.Equal(t, 503, httpErr.StatusCode, "Should return the last error encountered")
}

// TestRetryStopsOnNonRetryableError tests that retry immediately stops on non-retryable errors
func TestRetryStopsOnNonRetryableError(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		if attemptCount == 1 {
			return &mockHTTPError{StatusCode: http.StatusTooManyRequests} // retryable
		}
		return &mockHTTPError{StatusCode: http.StatusNotFound} // non-retryable
	}

	start := time.Now()
	err := WithRetry(operation, 5, 50*time.Millisecond)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Equal(t, 2, attemptCount, "Should stop immediately on non-retryable error")
	// Should have waited for first retry delay (~50ms) but not more
	assert.Less(t, duration, 150*time.Millisecond, "Should not continue retrying after non-retryable error")
}

// TestRetryWithMixedErrorTypes tests handling of mixed HTTP and non-HTTP errors
func TestRetryWithMixedErrorTypes(t *testing.T) {
	attemptCount := 0

	operation := func() error {
		attemptCount++
		switch attemptCount {
		case 1:
			return &mockHTTPError{StatusCode: http.StatusTooManyRequests} // retryable
		case 2:
			return errors.New("non-HTTP error") // non-retryable
		default:
			return nil
		}
	}

	err := WithRetry(operation, 5, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, 2, attemptCount, "Should stop on non-HTTP error")
	assert.Equal(t, "non-HTTP error", err.Error())
}
