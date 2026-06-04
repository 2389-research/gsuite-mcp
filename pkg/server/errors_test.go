// ABOUTME: Tests for the toolError helper that sanitizes upstream Google API errors.
// ABOUTME: Verifies that sensitive fields (Body, Message) from googleapi.Error never reach the client.

package server

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
)

// toolResultText extracts the text from the first content item of a CallToolResult.
func toolResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotNil(t, result, "CallToolResult must not be nil")
	require.NotEmpty(t, result.Content, "CallToolResult must have at least one content item")
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "first content item must be a TextContent, got %T", result.Content[0])
	return tc.Text
}

// TestToolError_GoogleAPIError_SecretNotLeaked is the core spec test:
// a *googleapi.Error with Body "secret" must never surface "secret" in the
// returned tool result text.
func TestToolError_GoogleAPIError_SecretNotLeaked(t *testing.T) {
	err := &googleapi.Error{Code: 403, Message: "secret", Body: "secret"}
	result := toolError(err)

	require.NotNil(t, result)
	assert.True(t, result.IsError, "result must be marked as an error")

	text := toolResultText(t, result)
	assert.NotContains(t, text, "secret", "Body content must not appear in the tool result")
	assert.Contains(t, text, "403", "safe HTTP code should appear in the tool result")
}

// TestToolError_PlainError_PassThrough verifies that non-Google-API errors are
// returned unchanged — the caller's message is preserved exactly.
func TestToolError_PlainError_PassThrough(t *testing.T) {
	err := errors.New("plain validation error")
	result := toolError(err)

	require.NotNil(t, result)
	assert.True(t, result.IsError)

	text := toolResultText(t, result)
	assert.Equal(t, "plain validation error", text)
}

// TestToolError_WrappedGoogleAPIError_SecretNotLeaked verifies that errors.As
// unwraps through fmt.Errorf wrapping so that a wrapped *googleapi.Error is
// also sanitized. This mirrors what the service layer does:
// fmt.Errorf("unable to list messages: %w", googleErr).
func TestToolError_WrappedGoogleAPIError_SecretNotLeaked(t *testing.T) {
	wrapped := fmt.Errorf("unable to list messages: %w", &googleapi.Error{Code: 403, Body: "secret"})
	result := toolError(wrapped)

	require.NotNil(t, result)
	assert.True(t, result.IsError)

	text := toolResultText(t, result)
	assert.NotContains(t, text, "secret", "Body content must not appear even when the googleapi.Error is wrapped")
	assert.Contains(t, text, "403", "safe HTTP code should appear in the tool result")
}

// TestToolError_StatusCodeMapping verifies that each status class maps to the
// expected safe, human-readable hint.
func TestToolError_StatusCodeMapping(t *testing.T) {
	cases := []struct {
		code        int
		wantContain string
	}{
		{401, "401"},
		{403, "403"},
		{404, "404"},
		{429, "429"},
		{500, "5xx"},
		{503, "5xx"},
		{418, "418"}, // default fallthrough
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("code_%d", tc.code), func(t *testing.T) {
			result := toolError(&googleapi.Error{Code: tc.code, Body: "internal detail"})
			text := toolResultText(t, result)
			assert.NotContains(t, text, "internal detail")
			assert.Contains(t, text, tc.wantContain)
		})
	}
}
