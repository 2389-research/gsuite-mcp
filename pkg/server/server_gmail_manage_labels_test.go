// ABOUTME: Tests for gmail_manage_labels MCP tool handler
// ABOUTME: Validates label management operations with mock HTTP server

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMockGmailServer creates a test server that mocks Gmail API label endpoints
func setupMockGmailServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		path := r.URL.Path
		method := r.Method

		// Labels list
		if path == "/gmail/v1/users/me/labels" && method == "GET" {
			w.Write([]byte(`{
				"labels": [
					{"id": "INBOX", "name": "INBOX", "type": "system"},
					{"id": "SENT", "name": "SENT", "type": "system"},
					{"id": "TRASH", "name": "TRASH", "type": "system"},
					{"id": "Label_1", "name": "Projects", "type": "user"},
					{"id": "Label_2", "name": "Projects/Client-A", "type": "user"}
				]
			}`))
			return
		}

		// Labels create
		if path == "/gmail/v1/users/me/labels" && method == "POST" {
			w.Write([]byte(`{
				"id": "Label_new",
				"name": "Test/NewLabel",
				"type": "user",
				"labelListVisibility": "labelShow",
				"messageListVisibility": "show"
			}`))
			return
		}

		// Labels get specific
		if strings.HasPrefix(path, "/gmail/v1/users/me/labels/") && method == "GET" {
			labelID := strings.TrimPrefix(path, "/gmail/v1/users/me/labels/")
			if labelID == "Label_1" {
				w.Write([]byte(`{
					"id": "Label_1",
					"name": "Projects",
					"type": "user",
					"messagesTotal": 42,
					"messagesUnread": 5
				}`))
				return
			}
			if labelID == "INBOX" {
				w.Write([]byte(`{
					"id": "INBOX",
					"name": "INBOX",
					"type": "system",
					"messagesTotal": 1000,
					"messagesUnread": 10
				}`))
				return
			}
			w.WriteHeader(404)
			w.Write([]byte(`{"error": {"code": 404, "message": "Label not found"}}`))
			return
		}

		// Labels update (PUT or PATCH)
		if strings.HasPrefix(path, "/gmail/v1/users/me/labels/") && (method == "PUT" || method == "PATCH") {
			labelID := strings.TrimPrefix(path, "/gmail/v1/users/me/labels/")
			if labelID == "INBOX" || labelID == "SENT" || labelID == "TRASH" {
				w.WriteHeader(400)
				w.Write([]byte(`{"error": {"code": 400, "message": "systemLabelCannotBeUpdated"}}`))
				return
			}
			w.Write([]byte(`{
				"id": "` + labelID + `",
				"name": "Updated Name",
				"type": "user"
			}`))
			return
		}

		// Labels delete
		if strings.HasPrefix(path, "/gmail/v1/users/me/labels/") && method == "DELETE" {
			labelID := strings.TrimPrefix(path, "/gmail/v1/users/me/labels/")
			if labelID == "INBOX" || labelID == "SENT" || labelID == "TRASH" {
				w.WriteHeader(400)
				w.Write([]byte(`{"error": {"code": 400, "message": "systemLabelCannotBeDeleted"}}`))
				return
			}
			w.WriteHeader(204)
			return
		}

		// Default fallback
		w.WriteHeader(404)
		w.Write([]byte(`{"error": {"code": 404, "message": "not found"}}`))
	}))
}

func TestHandleGmailManageLabels_ListAction(t *testing.T) {
	server := setupMockGmailServer(t)
	defer server.Close()

	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", server.URL)

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	request := createMockRequest("gmail_manage_labels", map[string]interface{}{
		"action": "list",
	})
	result, err := srv.handleGmailManageLabels(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError, "list should succeed")

	// Parse and verify response
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			var response map[string]interface{}
			err := json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			assert.Equal(t, "list", response["action"])
			assert.NotNil(t, response["labels"])
			assert.NotNil(t, response["count"])

			labels := response["labels"].([]interface{})
			assert.GreaterOrEqual(t, len(labels), 3, "should have at least 3 labels")

			// Verify label structure
			firstLabel := labels[0].(map[string]interface{})
			assert.NotEmpty(t, firstLabel["id"])
			assert.NotEmpty(t, firstLabel["name"])
			assert.NotEmpty(t, firstLabel["type"])
		}
	}
}

func TestHandleGmailManageLabels_GetAction(t *testing.T) {
	server := setupMockGmailServer(t)
	defer server.Close()

	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", server.URL)

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	t.Run("get existing label", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":   "get",
			"label_id": "Label_1",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.False(t, result.IsError)

		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			var response map[string]interface{}
			json.Unmarshal([]byte(textContent.Text), &response)

			assert.Equal(t, "get", response["action"])
			label := response["label"].(map[string]interface{})
			assert.Equal(t, "Label_1", label["id"])
			assert.Equal(t, "Projects", label["name"])
		}
	})

	t.Run("get system label", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":   "get",
			"label_id": "INBOX",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.False(t, result.IsError)

		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			var response map[string]interface{}
			json.Unmarshal([]byte(textContent.Text), &response)

			label := response["label"].(map[string]interface{})
			assert.Equal(t, "INBOX", label["id"])
			assert.Equal(t, "system", label["type"])
		}
	})

	t.Run("get nonexistent label", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":   "get",
			"label_id": "NonExistent",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.True(t, result.IsError)

		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			assert.Contains(t, textContent.Text, "action: list")
		}
	})
}

func TestHandleGmailManageLabels_CreateAction(t *testing.T) {
	server := setupMockGmailServer(t)
	defer server.Close()

	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", server.URL)

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	t.Run("create simple label", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action": "create",
			"name":   "Test/NewLabel",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.False(t, result.IsError)

		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			var response map[string]interface{}
			json.Unmarshal([]byte(textContent.Text), &response)

			assert.Equal(t, "create", response["action"])
			assert.NotEmpty(t, response["message"])
			label := response["label"].(map[string]interface{})
			assert.NotEmpty(t, label["id"])
		}
	})

	t.Run("create with visibility options", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":                   "create",
			"name":                     "Visible Label",
			"label_list_visibility":   "labelShow",
			"message_list_visibility": "show",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.False(t, result.IsError)
	})
}

func TestHandleGmailManageLabels_UpdateAction(t *testing.T) {
	server := setupMockGmailServer(t)
	defer server.Close()

	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", server.URL)

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	t.Run("update user label name", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":   "update",
			"label_id": "Label_1",
			"name":     "Projects Renamed",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.False(t, result.IsError)

		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			var response map[string]interface{}
			json.Unmarshal([]byte(textContent.Text), &response)

			assert.Equal(t, "update", response["action"])
			assert.Contains(t, response["message"], "updated")
		}
	})

	t.Run("update label visibility", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":                 "update",
			"label_id":               "Label_1",
			"label_list_visibility":  "labelHide",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("update system label fails with helpful error", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":   "update",
			"label_id": "INBOX",
			"name":     "My Inbox",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.True(t, result.IsError)

		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			assert.Contains(t, textContent.Text, "system labels")
		}
	})
}

func TestHandleGmailManageLabels_DeleteAction(t *testing.T) {
	server := setupMockGmailServer(t)
	defer server.Close()

	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", server.URL)

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	t.Run("delete user label", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":   "delete",
			"label_id": "Label_1",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.False(t, result.IsError)

		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			var response map[string]interface{}
			json.Unmarshal([]byte(textContent.Text), &response)

			assert.Equal(t, "delete", response["action"])
			assert.Contains(t, response["message"], "deleted")
		}
	})

	t.Run("delete system label fails with helpful error", func(t *testing.T) {
		request := createMockRequest("gmail_manage_labels", map[string]interface{}{
			"action":   "delete",
			"label_id": "INBOX",
		})
		result, err := srv.handleGmailManageLabels(context.Background(), request)

		require.NoError(t, err)
		assert.True(t, result.IsError)

		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			assert.Contains(t, textContent.Text, "system labels")
		}
	})
}

func TestHandleGmailManageLabels_ValidationErrors(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           map[string]interface{}
		errorContains  string
	}{
		{
			name:          "missing action",
			args:          map[string]interface{}{},
			errorContains: "action is required",
		},
		{
			name: "invalid action",
			args: map[string]interface{}{
				"action": "invalid",
			},
			errorContains: "unknown action",
		},
		{
			name: "get without label_id",
			args: map[string]interface{}{
				"action": "get",
			},
			errorContains: "label_id is required",
		},
		{
			name: "create without name",
			args: map[string]interface{}{
				"action": "create",
			},
			errorContains: "name is required",
		},
		{
			name: "update without label_id",
			args: map[string]interface{}{
				"action": "update",
				"name":   "New Name",
			},
			errorContains: "label_id is required",
		},
		{
			name: "update without fields",
			args: map[string]interface{}{
				"action":   "update",
				"label_id": "Label_1",
			},
			errorContains: "at least one of",
		},
		{
			name: "delete without label_id",
			args: map[string]interface{}{
				"action": "delete",
			},
			errorContains: "label_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createMockRequest("gmail_manage_labels", tt.args)
			result, err := srv.handleGmailManageLabels(context.Background(), request)

			require.NoError(t, err)
			assert.True(t, result.IsError)

			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				assert.Contains(t, textContent.Text, tt.errorContains)
			}
		})
	}
}

func TestHandleGmailManageLabels_HelpfulErrorMessages(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           map[string]interface{}
		expectedSubstr string
	}{
		{
			name:           "missing action lists valid actions",
			args:           map[string]interface{}{},
			expectedSubstr: "list, get, create, update, delete",
		},
		{
			name: "get without label_id suggests list",
			args: map[string]interface{}{
				"action": "get",
			},
			expectedSubstr: "action: list",
		},
		{
			name: "create without name explains nesting",
			args: map[string]interface{}{
				"action": "create",
			},
			expectedSubstr: "slashes",
		},
		{
			name: "invalid action shows valid options",
			args: map[string]interface{}{
				"action": "foobar",
			},
			expectedSubstr: "list, get, create, update, delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createMockRequest("gmail_manage_labels", tt.args)
			result, err := srv.handleGmailManageLabels(context.Background(), request)

			require.NoError(t, err)
			assert.True(t, result.IsError)

			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				assert.Contains(t, textContent.Text, tt.expectedSubstr)
			}
		})
	}
}
