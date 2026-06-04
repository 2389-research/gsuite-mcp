// ABOUTME: Tests for the per-request timeout middleware in the MCP server.
// ABOUTME: Verifies that outbound API calls are bounded by a deadline, preventing indefinite hangs.

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithRequestTimeout_InjectsDeadline verifies that the middleware adds a
// deadline to the context seen by the wrapped handler.
func TestWithRequestTimeout_InjectsDeadline(t *testing.T) {
	t.Run("active parent gains deadline", func(t *testing.T) {
		var captured context.Context

		probe := func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			captured = ctx
			return mcp.NewToolResultText("ok"), nil
		}

		wrapped := withRequestTimeout(30 * time.Second)(probe)
		_, err := wrapped(context.Background(), mcp.CallToolRequest{})
		require.NoError(t, err)

		deadline, ok := captured.Deadline()
		assert.True(t, ok, "wrapped context must have a deadline")
		assert.True(t, deadline.After(time.Now()), "deadline must be in the future")
		assert.True(t, deadline.Before(time.Now().Add(31*time.Second)), "deadline must be within the timeout window")
	})

	t.Run("already-cancelled parent propagates cancellation", func(t *testing.T) {
		var captured context.Context

		probe := func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			captured = ctx
			return mcp.NewToolResultText("ok"), nil
		}

		// Build a parent context that is already cancelled before calling the middleware.
		parent, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		wrapped := withRequestTimeout(30 * time.Second)(probe)
		_, _ = wrapped(parent, mcp.CallToolRequest{})

		assert.NotNil(t, captured.Err(), "context derived from a cancelled parent must already be done")
	})
}

// TestWithRequestTimeout_BoundsSlowCall verifies that a real handler backed by a
// slow upstream server returns promptly once the timeout fires, rather than
// hanging until the upstream eventually responds.
//
// This test uses ISH mode and httptest to simulate a slow Google API endpoint;
// it then calls the handler with a short timeout and asserts the elapsed time is
// well under the upstream's simulated delay.
func TestWithRequestTimeout_BoundsSlowCall(t *testing.T) {
	const upstreamDelay = 3 * time.Second
	const handlerTimeout = 50 * time.Millisecond
	const maxAcceptableElapsed = 1 * time.Second

	// Stand up a slow httptest server that sleeps before responding. The select
	// on r.Context().Done() lets the handler goroutine exit promptly when the
	// client cancels (deadline fired), so defer slowSrv.Close() does not block
	// for the full upstreamDelay on every run. The test's true-positive property
	// is preserved: without the timeout middleware the client never cancels, the
	// server waits the full 3s, and elapsed≈3s fails the assertion.
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(upstreamDelay):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"messages":[]}`))
		case <-r.Context().Done():
			// Client cancelled (deadline fired); return promptly so Close() doesn't block.
		}
	}))
	defer slowSrv.Close()

	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", slowSrv.URL)

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	wrapped := withRequestTimeout(handlerTimeout)(srv.handleGmailListMessages)

	start := time.Now()
	_, _ = wrapped(context.Background(), mcp.CallToolRequest{})
	elapsed := time.Since(start)

	// The call must return well under the upstream's sleep duration; the timeout
	// should have fired at ~50 ms and caused the handler to return with an error.
	assert.Less(t, elapsed, maxAcceptableElapsed,
		"handler bounded by %v must return in < %v (got %v); upstream delay is %v",
		handlerTimeout, maxAcceptableElapsed, elapsed, upstreamDelay)
}
