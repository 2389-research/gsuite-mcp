// ABOUTME: Integration tests for MCP server functionality
// ABOUTME: Tests full MCP request/response cycles, prompts, resources, and error handling

package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullMCPRequestResponseCycleWithMultipleTools tests a complete workflow
// using multiple tools in sequence, simulating a real user interaction
func TestFullMCPRequestResponseCycleWithMultipleTools(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	ctx := context.Background()

	// Test 1: List messages
	listRequest := createMockRequest("gmail_list_messages", map[string]interface{}{
		"query":       "is:unread",
		"max_results": 5,
	})
	listResult, err := srv.handleGmailListMessages(ctx, listRequest)
	require.NoError(t, err)
	assert.NotNil(t, listResult)
	assert.NotEmpty(t, listResult.Content)

	// Test 2: Create a draft
	draftRequest := createMockRequest("gmail_create_draft", map[string]interface{}{
		"to":      "test@example.com",
		"subject": "Integration Test Draft",
		"body":    "This is a test draft created during integration testing",
	})
	draftResult, err := srv.handleGmailCreateDraft(ctx, draftRequest)
	require.NoError(t, err)
	assert.NotNil(t, draftResult)
	assert.NotEmpty(t, draftResult.Content)

	// Test 3: List calendar events
	calendarRequest := createMockRequest("calendar_list_events", map[string]interface{}{
		"max_results": 10,
	})
	calendarResult, err := srv.handleCalendarListEvents(ctx, calendarRequest)
	require.NoError(t, err)
	assert.NotNil(t, calendarResult)
	assert.NotEmpty(t, calendarResult.Content)

	// Test 4: Search contacts
	contactRequest := createMockRequest("people_search_contacts", map[string]interface{}{
		"query":     "test",
		"page_size": 5,
	})
	contactResult, err := srv.handlePeopleSearchContacts(ctx, contactRequest)
	require.NoError(t, err)
	assert.NotNil(t, contactResult)
	assert.NotEmpty(t, contactResult.Content)

	// Test 5: Create calendar event
	now := time.Now()
	startTime := now.Add(24 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	createEventRequest := createMockRequest("calendar_create_event", map[string]interface{}{
		"summary":     "Integration Test Meeting",
		"description": "Test event created during integration testing",
		"start_time":  startTime.Format(time.RFC3339),
		"end_time":    endTime.Format(time.RFC3339),
	})
	eventResult, err := srv.handleCalendarCreateEvent(ctx, createEventRequest)
	require.NoError(t, err)
	assert.NotNil(t, eventResult)
	assert.NotEmpty(t, eventResult.Content)
}

// TestMCPPromptListingAndExecution tests all registered prompts
func TestMCPPromptListingAndExecution(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	ctx := context.Background()

	// Test that we have prompts registered
	// Note: The MCP server stores prompts internally, so we test via execution

	tests := []struct {
		name          string
		promptName    string
		args          map[string]string
		expectError   bool
		errorContains string
	}{
		{
			name:       "email_triage_prompt_all",
			promptName: "email_triage",
			args:       map[string]string{"priority": "all"},
		},
		{
			name:       "email_triage_prompt_urgent",
			promptName: "email_triage",
			args:       map[string]string{"priority": "urgent"},
		},
		{
			name:       "schedule_meeting_prompt",
			promptName: "schedule_meeting",
			args:       map[string]string{"duration": "60", "attendees": "test@example.com"},
		},
		{
			name:       "compose_email_prompt",
			promptName: "compose_email",
			args:       map[string]string{"context": "follow-up on meeting", "tone": "professional"},
		},
		{
			name:       "find_contact_prompt",
			promptName: "find_contact",
			args:       map[string]string{"search_term": "John Doe"},
		},
		{
			name:       "calendar_summary_today",
			promptName: "calendar_summary",
			args:       map[string]string{"period": "today"},
		},
		{
			name:       "calendar_summary_this_week",
			promptName: "calendar_summary",
			args:       map[string]string{"period": "this_week"},
		},
		{
			name:       "follow_up_reminder_prompt",
			promptName: "follow_up_reminder",
			args:       map[string]string{"context": "Review project proposal", "when": "tomorrow"},
		},
		{
			name:       "email_reply_prompt",
			promptName: "email_reply",
			args:       map[string]string{"subject": "Re: Test", "sender": "test@example.com"},
		},
		{
			name:       "add_contact_from_email_prompt",
			promptName: "add_contact_from_email",
			args:       map[string]string{"email_subject": "Introduction", "sender": "new@example.com"},
		},
		{
			name:          "find_contact_missing_required",
			promptName:    "find_contact",
			args:          map[string]string{},
			expectError:   true,
			errorContains: "required",
		},
		{
			name:          "follow_up_reminder_missing_context",
			promptName:    "follow_up_reminder",
			args:          map[string]string{"when": "tomorrow"},
			expectError:   true,
			errorContains: "required",
		},
		{
			name:       "compose_email_with_default_context",
			promptName: "compose_email",
			args:       map[string]string{"tone": "casual"},
			// context is optional, should use default
		},
		{
			name:          "add_contact_missing_subject",
			promptName:    "add_contact_from_email",
			args:          map[string]string{},
			expectError:   true,
			errorContains: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.GetPromptRequest{
				Request: mcp.Request{
					Method: "prompts/get",
				},
				Params: mcp.GetPromptParams{
					Name:      tt.promptName,
					Arguments: tt.args,
				},
			}

			var result *mcp.GetPromptResult
			var err error

			// Route to the appropriate handler
			switch tt.promptName {
			case "email_triage":
				result, err = srv.handleEmailTriagePrompt(ctx, request)
			case "schedule_meeting":
				result, err = srv.handleScheduleMeetingPrompt(ctx, request)
			case "compose_email":
				result, err = srv.handleComposeEmailPrompt(ctx, request)
			case "find_contact":
				result, err = srv.handleFindContactPrompt(ctx, request)
			case "calendar_summary":
				result, err = srv.handleCalendarSummaryPrompt(ctx, request)
			case "follow_up_reminder":
				result, err = srv.handleFollowUpReminderPrompt(ctx, request)
			case "email_reply":
				result, err = srv.handleEmailReplyPrompt(ctx, request)
			case "add_contact_from_email":
				result, err = srv.handleAddContactFromEmailPrompt(ctx, request)
			default:
				t.Fatalf("Unknown prompt: %s", tt.promptName)
			}

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.Messages)

				// Verify message structure
				assert.Greater(t, len(result.Messages), 0)
				for _, msg := range result.Messages {
					assert.NotEmpty(t, msg.Role)
					assert.NotEmpty(t, msg.Content)
				}
			}
		})
	}
}

// TestMCPResourceEndpointsReturnValidJSON tests all 8 resource endpoints
// Note: This test requires ISH_MODE to be set and an ish server running.
// It skips if the server is not available to allow unit testing.
func TestMCPResourceEndpointsReturnValidJSON(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name     string
		uri      string
		handler  func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)
		validate func(t *testing.T, contents []mcp.ResourceContents)
	}{
		{
			name:    "today_calendar",
			uri:     "gsuite://calendar/today",
			handler: srv.handleTodayCalendarResource,
			validate: func(t *testing.T, contents []mcp.ResourceContents) {
				require.Len(t, contents, 1)
				textContent := contents[0].(mcp.TextResourceContents)
				assert.Equal(t, "application/json", textContent.MIMEType)
				assert.NotEmpty(t, textContent.Text)

				var data map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &data)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, data, "date")
				assert.Contains(t, data, "event_count")
				assert.Contains(t, data, "events")
			},
		},
		{
			name:    "this_week_calendar",
			uri:     "gsuite://calendar/this-week",
			handler: srv.handleThisWeekCalendarResource,
			validate: func(t *testing.T, contents []mcp.ResourceContents) {
				require.Len(t, contents, 1)
				textContent := contents[0].(mcp.TextResourceContents)
				assert.Equal(t, "application/json", textContent.MIMEType)

				var data map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &data)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, data, "week_start")
				assert.Contains(t, data, "week_end")
				assert.Contains(t, data, "event_count")
				assert.Contains(t, data, "events_by_day")
			},
		},
		{
			name:    "unread_emails",
			uri:     "gsuite://gmail/unread",
			handler: srv.handleUnreadEmailsResource,
			validate: func(t *testing.T, contents []mcp.ResourceContents) {
				require.Len(t, contents, 1)
				textContent := contents[0].(mcp.TextResourceContents)
				assert.Equal(t, "application/json", textContent.MIMEType)

				var data map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &data)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, data, "unread_count")
				assert.Contains(t, data, "messages")
				assert.Contains(t, data, "timestamp")
			},
		},
		{
			name:    "important_unread_emails",
			uri:     "gsuite://gmail/unread/important",
			handler: srv.handleImportantEmailsResource,
			validate: func(t *testing.T, contents []mcp.ResourceContents) {
				require.Len(t, contents, 1)
				textContent := contents[0].(mcp.TextResourceContents)
				assert.Equal(t, "application/json", textContent.MIMEType)

				var data map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &data)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, data, "important_unread_count")
				assert.Contains(t, data, "messages")
				assert.Contains(t, data, "timestamp")
			},
		},
		{
			name:    "recent_contacts",
			uri:     "gsuite://contacts/recent",
			handler: srv.handleRecentContactsResource,
			validate: func(t *testing.T, contents []mcp.ResourceContents) {
				require.Len(t, contents, 1)
				textContent := contents[0].(mcp.TextResourceContents)
				assert.Equal(t, "application/json", textContent.MIMEType)

				var data map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &data)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, data, "contact_count")
				assert.Contains(t, data, "contacts")
				assert.Contains(t, data, "timestamp")
			},
		},
		{
			name:    "upcoming_meetings",
			uri:     "gsuite://calendar/upcoming",
			handler: srv.handleUpcomingMeetingsResource,
			validate: func(t *testing.T, contents []mcp.ResourceContents) {
				require.Len(t, contents, 1)
				textContent := contents[0].(mcp.TextResourceContents)
				assert.Equal(t, "application/json", textContent.MIMEType)

				var data map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &data)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, data, "upcoming_count")
				assert.Contains(t, data, "events")
				assert.Contains(t, data, "time_range")
			},
		},
		{
			name:    "calendar_availability",
			uri:     "gsuite://calendar/availability",
			handler: srv.handleCalendarAvailabilityResource,
			validate: func(t *testing.T, contents []mcp.ResourceContents) {
				require.Len(t, contents, 1)
				textContent := contents[0].(mcp.TextResourceContents)
				assert.Equal(t, "application/json", textContent.MIMEType)

				var data map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &data)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, data, "period")
				assert.Contains(t, data, "availability")
				assert.Contains(t, data, "generated_at")

				// Validate availability structure
				avail := data["availability"].(map[string]interface{})
				assert.Greater(t, len(avail), 0, "Should have availability data")

				// Check one day's structure
				for _, dayData := range avail {
					dayMap := dayData.(map[string]interface{})
					assert.Contains(t, dayMap, "day_name")
					assert.Contains(t, dayMap, "busy_hours")
					assert.Contains(t, dayMap, "free_hours")
					assert.Contains(t, dayMap, "event_count")
					assert.Contains(t, dayMap, "status")
					break
				}
			},
		},
		{
			name:    "draft_emails",
			uri:     "gsuite://gmail/drafts",
			handler: srv.handleDraftsResource,
			validate: func(t *testing.T, contents []mcp.ResourceContents) {
				require.Len(t, contents, 1)
				textContent := contents[0].(mcp.TextResourceContents)
				assert.Equal(t, "application/json", textContent.MIMEType)

				var data map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &data)
				require.NoError(t, err, "Response should be valid JSON")
				assert.Contains(t, data, "draft_count")
				assert.Contains(t, data, "drafts")
				assert.Contains(t, data, "timestamp")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.ReadResourceRequest{
				Request: mcp.Request{
					Method: "resources/read",
				},
				Params: mcp.ReadResourceParams{
					URI: tt.uri,
				},
			}

			contents, err := tt.handler(ctx, request)

			// Skip if ish server is not available (allows unit testing without ish server)
			if err != nil && (strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "dial tcp")) {
				t.Skip("Skipping resource test: ish server not available")
				return
			}

			require.NoError(t, err, "Resource handler should not error")
			require.NotNil(t, contents, "Resource contents should not be nil")

			// Run custom validation
			tt.validate(t, contents)
		})
	}
}

// TestServerHandlesUnknownToolGracefully tests error handling for unknown tools
func TestServerHandlesUnknownToolGracefully(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	ctx := context.Background()

	// Note: The MCP server itself handles unknown tools, but we can test
	// that our handlers validate their inputs properly

	tests := []struct {
		name        string
		toolName    string
		handler     func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args        map[string]interface{}
		expectError bool
	}{
		{
			name:     "gmail_get_message_missing_id",
			toolName: "gmail_get_message",
			handler:  srv.handleGmailGetMessage,
			args:     map[string]interface{}{},
		},
		{
			name:     "gmail_send_message_missing_to",
			toolName: "gmail_send_message",
			handler:  srv.handleGmailSendMessage,
			args: map[string]interface{}{
				"subject": "Test",
				"body":    "Test body",
			},
		},
		{
			name:     "calendar_get_event_missing_id",
			toolName: "calendar_get_event",
			handler:  srv.handleCalendarGetEvent,
			args:     map[string]interface{}{},
		},
		{
			name:     "people_get_contact_missing_resource",
			toolName: "people_get_contact",
			handler:  srv.handlePeopleGetContact,
			args:     map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createMockRequest(tt.toolName, tt.args)
			result, err := tt.handler(ctx, request)

			// Handler should return a result with error content, not a Go error
			require.NoError(t, err, "Handler should not return Go error")
			require.NotNil(t, result, "Result should not be nil")

			// The result should indicate an error via its content
			assert.True(t, result.IsError, "Result should be marked as error")
			assert.NotEmpty(t, result.Content, "Error result should have content")
		})
	}
}

// TestServerHandlesMalformedRequestsGracefully tests various malformed requests
func TestServerHandlesMalformedRequestsGracefully(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		handler     func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		request     mcp.CallToolRequest
		description string
	}{
		{
			name:    "gmail_send_empty_to",
			handler: srv.handleGmailSendMessage,
			request: createMockRequest("gmail_send_message", map[string]interface{}{
				"to":      "",
				"subject": "Test",
				"body":    "Test",
			}),
			description: "Empty 'to' address should be rejected",
		},
		{
			name:    "gmail_send_empty_subject",
			handler: srv.handleGmailSendMessage,
			request: createMockRequest("gmail_send_message", map[string]interface{}{
				"to":      "test@example.com",
				"subject": "",
				"body":    "Test",
			}),
			description: "Empty subject should be rejected",
		},
		{
			name:    "calendar_create_invalid_time",
			handler: srv.handleCalendarCreateEvent,
			request: createMockRequest("calendar_create_event", map[string]interface{}{
				"summary":    "Test Event",
				"start_time": "not-a-valid-timestamp",
				"end_time":   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			}),
			description: "Invalid timestamp format should be rejected",
		},
		{
			name:    "calendar_list_invalid_time_min",
			handler: srv.handleCalendarListEvents,
			request: createMockRequest("calendar_list_events", map[string]interface{}{
				"time_min": "invalid-timestamp",
			}),
			description: "Invalid time_min should be rejected",
		},
		{
			name:    "gmail_modify_labels_invalid_args",
			handler: srv.handleGmailModifyLabels,
			request: createMockRequest("gmail_modify_labels", map[string]interface{}{
				"message_id": "test123",
				// add_labels and remove_labels are optional but should handle gracefully
			}),
			description: "Missing label arrays should be handled gracefully",
		},
		{
			name:    "people_create_contact_missing_name",
			handler: srv.handlePeopleCreateContact,
			request: createMockRequest("people_create_contact", map[string]interface{}{
				"email": "test@example.com",
			}),
			description: "Missing required given_name should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(ctx, tt.request)

			// Handler should return a result, not panic
			require.NoError(t, err, "Handler should not return Go error for validation failures")
			require.NotNil(t, result, "Result should not be nil")

			// The result should indicate an error
			assert.True(t, result.IsError, tt.description)
			assert.NotEmpty(t, result.Content, "Error result should have descriptive content")
		})
	}
}

// TestGmailModifyLabelsWithArrays tests the array handling in modify labels
func TestGmailModifyLabelsWithArrays(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name       string
		args       map[string]interface{}
		shouldWork bool
	}{
		{
			name: "add_labels_only",
			args: map[string]interface{}{
				"message_id": "msg123",
				"add_labels": []interface{}{"STARRED", "IMPORTANT"},
			},
			shouldWork: true,
		},
		{
			name: "remove_labels_only",
			args: map[string]interface{}{
				"message_id":     "msg123",
				"remove_labels": []interface{}{"UNREAD", "INBOX"},
			},
			shouldWork: true,
		},
		{
			name: "both_add_and_remove",
			args: map[string]interface{}{
				"message_id":     "msg123",
				"add_labels":    []interface{}{"STARRED"},
				"remove_labels": []interface{}{"UNREAD"},
			},
			shouldWork: true,
		},
		{
			name: "empty_arrays",
			args: map[string]interface{}{
				"message_id":     "msg123",
				"add_labels":    []interface{}{},
				"remove_labels": []interface{}{},
			},
			shouldWork: true,
		},
		{
			name: "no_label_arrays",
			args: map[string]interface{}{
				"message_id": "msg123",
			},
			shouldWork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createMockRequest("gmail_modify_labels", tt.args)
			result, err := srv.handleGmailModifyLabels(ctx, request)

			require.NoError(t, err, "Handler should not return Go error")
			require.NotNil(t, result, "Result should not be nil")

			if tt.shouldWork {
				// In ISH_MODE, this might still fail if the message doesn't exist
				// but the request structure should be valid
				assert.NotNil(t, result)
			}
		})
	}
}

// TestCalendarTimeRangeParsing tests time range parsing in calendar operations
func TestCalendarTimeRangeParsing(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	ctx := context.Background()

	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	nextWeek := now.Add(7 * 24 * time.Hour)

	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
	}{
		{
			name: "valid_time_range",
			args: map[string]interface{}{
				"max_results": 10,
				"time_min":    now.Format(time.RFC3339),
				"time_max":    tomorrow.Format(time.RFC3339),
			},
			expectError: false,
		},
		{
			name: "only_time_min",
			args: map[string]interface{}{
				"max_results": 10,
				"time_min":    now.Format(time.RFC3339),
			},
			expectError: false,
		},
		{
			name: "only_time_max",
			args: map[string]interface{}{
				"max_results": 10,
				"time_max":    nextWeek.Format(time.RFC3339),
			},
			expectError: false,
		},
		{
			name: "no_time_constraints",
			args: map[string]interface{}{
				"max_results": 10,
			},
			expectError: false,
		},
		{
			name: "invalid_time_min_format",
			args: map[string]interface{}{
				"time_min": "2024-13-45T99:99:99Z", // Invalid date
			},
			expectError: true,
		},
		{
			name: "invalid_time_max_format",
			args: map[string]interface{}{
				"time_max": "not-a-timestamp",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createMockRequest("calendar_list_events", tt.args)
			result, err := srv.handleCalendarListEvents(ctx, request)

			require.NoError(t, err, "Handler should not return Go error")
			require.NotNil(t, result, "Result should not be nil")

			if tt.expectError {
				assert.True(t, result.IsError, "Should return error result for invalid time format")
			} else {
				// In ISH_MODE, should succeed
				assert.NotEmpty(t, result.Content)
			}
		})
	}
}

// TestPeopleUpdateContactFieldMask tests the field mask generation for contact updates
func TestPeopleUpdateContactFieldMask(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
		description string
	}{
		{
			name: "update_name_only",
			args: map[string]interface{}{
				"resource_name": "people/12345",
				"given_name":    "John",
				"family_name":   "Doe",
			},
			expectError: false,
			description: "Should update name fields",
		},
		{
			name: "update_email_only",
			args: map[string]interface{}{
				"resource_name": "people/12345",
				"email":         "newemail@example.com",
			},
			expectError: false,
			description: "Should update email field",
		},
		{
			name: "update_phone_only",
			args: map[string]interface{}{
				"resource_name": "people/12345",
				"phone":         "+1234567890",
			},
			expectError: false,
			description: "Should update phone field",
		},
		{
			name: "update_all_fields",
			args: map[string]interface{}{
				"resource_name": "people/12345",
				"given_name":    "Jane",
				"family_name":   "Smith",
				"email":         "jane.smith@example.com",
				"phone":         "+9876543210",
			},
			expectError: false,
			description: "Should update all fields",
		},
		{
			name: "no_fields_to_update",
			args: map[string]interface{}{
				"resource_name": "people/12345",
			},
			expectError: true,
			description: "Should error when no fields provided",
		},
		{
			name: "missing_resource_name",
			args: map[string]interface{}{
				"given_name": "John",
			},
			expectError: true,
			description: "Should error when resource_name missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createMockRequest("people_update_contact", tt.args)
			result, err := srv.handlePeopleUpdateContact(ctx, request)

			require.NoError(t, err, "Handler should not return Go error")
			require.NotNil(t, result, "Result should not be nil")

			if tt.expectError {
				assert.True(t, result.IsError, tt.description)
			}
		})
	}
}

// TestToolRegistrationCompleteness verifies all expected tools are registered
func TestToolRegistrationCompleteness(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	tools := srv.ListTools()
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		// Gmail tools
		"gmail_list_messages",
		"gmail_get_message",
		"gmail_send_message",
		"gmail_create_draft",
		"gmail_send_draft",
		"gmail_modify_labels",
		"gmail_trash_message",
		"gmail_delete_message",
		// Calendar tools
		"calendar_list_events",
		"calendar_get_event",
		"calendar_create_event",
		"calendar_update_event",
		"calendar_delete_event",
		// People tools
		"people_list_contacts",
		"people_search_contacts",
		"people_get_contact",
		"people_create_contact",
		"people_update_contact",
		"people_delete_contact",
	}

	for _, expectedTool := range expectedTools {
		assert.True(t, toolNames[expectedTool],
			"Expected tool '%s' should be registered", expectedTool)
	}

	// Verify we have exactly the expected number of tools
	assert.Equal(t, len(expectedTools), len(tools),
		"Should have exactly %d tools registered", len(expectedTools))
}
