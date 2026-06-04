// ABOUTME: Error helpers for the MCP server's tool-result boundary.
// ABOUTME: Sanitizes upstream Google API errors so internal detail never reaches the client.

package server

import (
	"errors"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/api/googleapi"
)

// toolError converts err into a safe *mcp.CallToolResult for the client.
//
// If err wraps a *googleapi.Error the full error (including Body) is logged to
// stderr for the operator, and only a code-class hint is returned to the client.
// All other errors pass through unchanged so pre-validation messages reach the
// client as-is.
func toolError(err error) *mcp.CallToolResult {
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		// Log the complete upstream error (including Body) to stderr — operator-only.
		log.Printf("upstream Google API error: %v", gErr)
		return mcp.NewToolResultError(safeMessageForCode(gErr.Code))
	}
	return mcp.NewToolResultError(err.Error())
}

// safeMessageForCode maps an HTTP status code to a fixed, safe client message.
// Only the numeric Code is used — never gErr.Message, gErr.Body, or err.Error().
func safeMessageForCode(code int) string {
	switch {
	case code == 401 || code == 403:
		return fmt.Sprintf("permission denied (HTTP %d); the account may lack access or need re-authentication", code)
	case code == 404:
		return fmt.Sprintf("resource not found (HTTP %d)", code)
	case code == 429:
		return fmt.Sprintf("rate limited by Google (HTTP %d); retry later", code)
	// 5xx is grouped into one client message: the client can't act differently on
	// 500 vs 503, and the operator still gets the exact code from the stderr log above.
	case code >= 500 && code < 600:
		return "Google API is temporarily unavailable (HTTP 5xx); retry later"
	default:
		return fmt.Sprintf("Google API error (HTTP %d)", code)
	}
}
