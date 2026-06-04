// ABOUTME: Tests for MCP server
// ABOUTME: Validates server initialization and tool registration

package server

import (
	"context"
	"encoding/json"
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

func TestServer_HasTasksTools(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tools := srv.ListTools()
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["tasks_list_tasklists"])
	assert.True(t, toolNames["tasks_create_tasklist"])
	assert.True(t, toolNames["tasks_update_tasklist"])
	assert.True(t, toolNames["tasks_delete_tasklist"])
	assert.True(t, toolNames["tasks_list_tasks"])
	assert.True(t, toolNames["tasks_create_task"])
	assert.True(t, toolNames["tasks_update_task"])
	assert.True(t, toolNames["tasks_delete_task"])
}

func TestServer_ResolveServices_Default(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	svc, err := srv.resolveServices(context.Background(), "")
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.Gmail)
	assert.NotNil(t, svc.Calendar)
	assert.NotNil(t, svc.People)
	assert.NotNil(t, svc.Tasks)
}

func TestServer_ResolveServices_UnknownAccount(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	_, err = srv.resolveServices(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown account")
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

func TestServer_ToolsAcceptAccountParam(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tools := srv.ListTools()
	for _, tool := range tools {
		// accounts_list is the only tool that doesn't take an account param
		if tool.Name == "accounts_list" {
			continue
		}
		props := tool.InputSchema.Properties
		_, hasAccount := props["account"]
		assert.True(t, hasAccount, "tool %s should have 'account' property", tool.Name)
	}
}

func TestServer_AccountsList_Registered(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tools := srv.ListTools()
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["accounts_list"], "accounts_list tool should be registered")
}

func TestServer_HandleAccountsList_ISHMode(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("accounts_list", map[string]interface{}{})

	result, err := srv.handleAccountsList(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
	// In ISH mode, there should be at least 1 account (default)
	assert.False(t, result.IsError, "accounts_list should not return an error")
}

func TestHandleAccountsList_DoesNotExposeTokenPath(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("accounts_list", map[string]interface{}{})

	result, err := srv.handleAccountsList(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent")

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &parsed))

	accounts, ok := parsed["accounts"].([]interface{})
	require.True(t, ok, "expected accounts to be a list")
	require.NotEmpty(t, accounts, "expected at least one account")

	for i, item := range accounts {
		entry, ok := item.(map[string]interface{})
		require.True(t, ok, "account entry %d should be a map", i)

		_, hasTokenPath := entry["token_path"]
		assert.False(t, hasTokenPath, "account entry %d must not contain token_path", i)

		_, hasAlias := entry["alias"]
		assert.True(t, hasAlias, "account entry %d must contain alias", i)

		_, hasAuthenticated := entry["authenticated"]
		assert.True(t, hasAuthenticated, "account entry %d must contain authenticated", i)
	}
}

func TestServer_AuthToolsAcceptAccountParam(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	authTools := []string{"auth_status", "auth_info", "auth_init", "auth_complete", "auth_revoke"}
	tools := srv.ListTools()
	toolMap := make(map[string]mcp.Tool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	for _, name := range authTools {
		tool, exists := toolMap[name]
		require.True(t, exists, "tool %s should exist", name)
		_, hasAccount := tool.InputSchema.Properties["account"]
		assert.True(t, hasAccount, "auth tool %s should have 'account' property", name)
	}
}

func TestHandleAuthInfo_DoesNotExposeAccessToken(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("auth_info", map[string]interface{}{})

	result, err := srv.handleAuthInfo(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)

	// Extract the text content and verify no access_token fragment is present
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent")

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &parsed))
	_, hasAccessToken := parsed["access_token"]
	assert.False(t, hasAccessToken, "auth_info response must not contain access_token")
}
