// ABOUTME: Tests for panic recovery in the MCP server.
// ABOUTME: Verifies that WithRecovery() converts tool-handler panics into error responses instead of crashing.

package server

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panicHandler is a tool handler that unconditionally panics.
func panicHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	panic("intentional panic for recovery test")
}

// TestPanicRecovery_WithRecoveryOption verifies that WithRecovery() converts a
// panicking tool handler into an INTERNAL_ERROR JSONRPCError response rather
// than letting the panic escape and crash the process.
//
// Expected behaviour (confirmed from mcp-go v0.43.2 upstream tests):
//   - response is a mcp.JSONRPCError (not a panicking goroutine)
//   - error code is mcp.INTERNAL_ERROR
//   - error message contains the recovered panic value
func TestPanicRecovery_WithRecoveryOption(t *testing.T) {
	// Build an MCPServer with recovery enabled — mirrors our production constructor.
	mcpServer := server.NewMCPServer(
		"test-server",
		"test",
		server.WithRecovery(),
	)

	// Register a tool whose handler will panic on every call.
	mcpServer.AddTool(
		mcp.Tool{
			Name:        "panic_tool",
			Description: "A tool that panics",
			InputSchema: mcp.ToolInputSchema{Type: "object"},
		},
		panicHandler,
	)

	// Drive the tool call through HandleMessage. With WithRecovery() this must
	// NOT panic — the server should catch the panic and return an error response.
	resp := mcpServer.HandleMessage(context.Background(), []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "panic_tool"}
	}`))
	require.NotNil(t, resp)

	// The panic is caught by the recovery middleware and returned as an error
	// from the handler; mcp-go converts that into a JSONRPCError (INTERNAL_ERROR).
	errResp, ok := resp.(mcp.JSONRPCError)
	require.True(t, ok, "expected a JSONRPCError response; got %T — panic may have escaped", resp)
	assert.Equal(t, mcp.INTERNAL_ERROR, errResp.Error.Code)
	assert.Contains(t, errResp.Error.Message, "panic recovered")
	assert.Contains(t, errResp.Error.Message, "intentional panic for recovery test")
}

// TestPanicRecovery_WithoutRecoveryOption_PanicEscapes documents the behaviour
// WITHOUT WithRecovery(): the panic from the tool handler escapes HandleMessage
// and would crash a real server process. We catch it with a deferred recover to
// prove it escapes, then the sibling test above proves WithRecovery() stops it.
func TestPanicRecovery_WithoutRecoveryOption_PanicEscapes(t *testing.T) {
	// Build an MCPServer WITHOUT WithRecovery — the baseline we are fixing.
	mcpServer := server.NewMCPServer(
		"test-server-no-recovery",
		"test",
	)

	mcpServer.AddTool(
		mcp.Tool{
			Name:        "panic_tool",
			Description: "A tool that panics",
			InputSchema: mcp.ToolInputSchema{Type: "object"},
		},
		panicHandler,
	)

	// Invoke in a sub-goroutine so the panic does not kill the test binary.
	// If the panic escapes HandleMessage it is caught by our recover — that
	// is the RED condition this test documents.
	panicEscaped := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Panic escaped HandleMessage — expected in the WITHOUT-recovery case.
				panicEscaped <- true
			} else {
				panicEscaped <- false
			}
		}()
		mcpServer.HandleMessage(context.Background(), []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "tools/call",
			"params": {"name": "panic_tool"}
		}`))
		// If we reach here without panicking the server handled it internally.
		panicEscaped <- false
	}()

	escaped := <-panicEscaped
	// Version-coupled: true as of mcp-go v0.43.2. If this flips to false,
	// mcp-go added its own unconditional recovery and WithRecovery() is now
	// a documented no-op — adjust or remove this test accordingly.
	assert.True(t, escaped, "without WithRecovery() the panic should escape HandleMessage")
}

// TestNewServer_WithRecoveryWired verifies that the production NewServer
// constructor includes recovery: a call to HandleMessage with a panicking tool
// must return a JSONRPCError instead of letting the panic escape.
func TestNewServer_WithRecoveryWired(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	// Inject a panicking tool into the already-constructed MCPServer.
	srv.mcp.AddTool(
		mcp.Tool{
			Name:        "panic_tool_production",
			Description: "A tool that panics",
			InputSchema: mcp.ToolInputSchema{Type: "object"},
		},
		panicHandler,
	)

	resp := srv.mcp.HandleMessage(context.Background(), []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "panic_tool_production"}
	}`))
	require.NotNil(t, resp)

	// Production server with recovery: panic must become a JSONRPCError.
	errResp, ok := resp.(mcp.JSONRPCError)
	require.True(t, ok, "production server: expected a JSONRPCError; got %T — panic may have escaped", resp)
	assert.Equal(t, mcp.INTERNAL_ERROR, errResp.Error.Code)
	assert.Contains(t, errResp.Error.Message, "panic recovered")
}
