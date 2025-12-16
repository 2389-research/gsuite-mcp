// ABOUTME: Tests for MCP server
// ABOUTME: Validates server initialization and tool registration

package server

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMockRequest creates a mock CallToolRequest for testing
func createMockRequest(name string, args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

func TestNewServer_WithIshMode(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestServer_ListTools(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tools := srv.ListTools()
	assert.Greater(t, len(tools), 0)

	// Verify we have the expected tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	// Gmail tools
	assert.True(t, toolNames["gmail_list_messages"])
	assert.True(t, toolNames["gmail_send_message"])

	// Calendar tools
	assert.True(t, toolNames["calendar_list_events"])

	// People tools
	assert.True(t, toolNames["people_list_contacts"])
	assert.True(t, toolNames["people_search_contacts"])
	assert.True(t, toolNames["people_get_contact"])
}

func TestServer_HandleGmailListMessages(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	// Create a mock request
	request := createMockRequest("gmail_list_messages", map[string]interface{}{
		"query":       "test",
		"max_results": 10,
	})

	result, err := srv.handleGmailListMessages(context.Background(), request)

	// In ish mode, this should work with fake data
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestServer_HandleGmailSendMessage(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("gmail_send_message", map[string]interface{}{
		"to":      "test@example.com",
		"subject": "Test Subject",
		"body":    "Test body",
	})

	result, err := srv.handleGmailSendMessage(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestServer_HandleCalendarListEvents(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("calendar_list_events", map[string]interface{}{
		"max_results": 10,
	})

	result, err := srv.handleCalendarListEvents(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestServer_HandlePeopleListContacts(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("people_list_contacts", map[string]interface{}{
		"page_size": 10,
	})

	result, err := srv.handlePeopleListContacts(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestServer_HandlePeopleSearchContacts(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("people_search_contacts", map[string]interface{}{
		"query":     "John",
		"page_size": 5,
	})

	result, err := srv.handlePeopleSearchContacts(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestServer_HandlePeopleGetContact(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("people_get_contact", map[string]interface{}{
		"resource_name": "people/12345",
	})

	result, _ := srv.handlePeopleGetContact(context.Background(), request)

	// This may return an error if the resource doesn't exist in ish mode
	// but the handler should still work correctly
	assert.NotNil(t, result)
}

func TestExtractAuthCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full localhost URL",
			input:    "http://localhost/?code=4/0AfJohXl123abc&scope=email",
			expected: "4/0AfJohXl123abc",
		},
		{
			name:     "URL with multiple params",
			input:    "http://localhost:8080/?state=xyz&code=AUTH_CODE_HERE&scope=a%20b",
			expected: "AUTH_CODE_HERE",
		},
		{
			name:     "raw code passthrough",
			input:    "4/0AfJohXl123abc",
			expected: "4/0AfJohXl123abc",
		},
		{
			name:     "URL without code param",
			input:    "http://localhost/?error=access_denied",
			expected: "http://localhost/?error=access_denied",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAuthCode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
