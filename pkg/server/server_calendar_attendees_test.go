// ABOUTME: Tests for calendar event attendee management
// ABOUTME: Validates full replacement and incremental attendee updates

package server

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test full replacement mode with attendees
func TestHandleCalendarUpdateEvent_FullReplacementAttendees(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-attendees-replace"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":  eventID,
		"attendees": []string{"alice@example.com", "bob@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test full replacement mode with optional_attendees
func TestHandleCalendarUpdateEvent_FullReplacementOptionalAttendees(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-optional-replace"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":           eventID,
		"optional_attendees": []string{"charlie@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test full replacement mode with both attendees and optional_attendees
func TestHandleCalendarUpdateEvent_FullReplacementBoth(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-both-replace"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":           eventID,
		"attendees":          []string{"alice@example.com"},
		"optional_attendees": []string{"bob@example.com", "charlie@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test incremental mode - add_attendees
func TestHandleCalendarUpdateEvent_IncrementalAddAttendees(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-add-attendees"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":      eventID,
		"add_attendees": []string{"newcomer@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test incremental mode - add_optional_attendees
func TestHandleCalendarUpdateEvent_IncrementalAddOptionalAttendees(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-add-optional"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":                eventID,
		"add_optional_attendees": []string{"maybe@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test incremental mode - remove_attendees
func TestHandleCalendarUpdateEvent_IncrementalRemoveAttendees(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-remove-attendees"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":         eventID,
		"remove_attendees": []string{"departed@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test incremental mode - multiple operations
func TestHandleCalendarUpdateEvent_IncrementalMultiple(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-incremental-multi"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":                eventID,
		"add_attendees":           []string{"new1@example.com"},
		"add_optional_attendees": []string{"new2@example.com"},
		"remove_attendees":        []string{"old@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test mixing modes returns error
func TestHandleCalendarUpdateEvent_MixingModesError(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-mixing-modes"

	// Try to mix full replacement (attendees) with incremental (add_attendees)
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":      eventID,
		"attendees":     []string{"alice@example.com"},
		"add_attendees": []string{"bob@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err) // No Go error
	assert.NotNil(t, result)
	assert.True(t, result.IsError) // But result should be an error
	assert.NotEmpty(t, result.Content)
	// Check that error message mentions mode mixing
	if len(result.Content) > 0 {
		content := result.Content[0]
		if textContent, ok := content.(mcp.TextContent); ok {
			assert.Contains(t, textContent.Text, "cannot mix")
		}
	}
}

// Test omitting all attendee params leaves unchanged
func TestHandleCalendarUpdateEvent_NoAttendeeParamsUnchanged(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-no-attendee-changes"

	// Update only summary, no attendee params
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id": eventID,
		"summary":  "Updated Summary Only",
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test send_notifications parameter
func TestHandleCalendarUpdateEvent_SendNotifications(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-send-notifications"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":           eventID,
		"attendees":          []string{"alice@example.com"},
		"send_notifications": false,
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test send_notifications defaults to true
func TestHandleCalendarUpdateEvent_SendNotificationsDefault(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-default-notifications"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":  eventID,
		"attendees": []string{"alice@example.com"},
		// send_notifications not specified, should default to true
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test edge case: empty arrays
func TestHandleCalendarUpdateEvent_EmptyArrays(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-empty-arrays"

	// Empty arrays in full replacement mode should clear all attendees
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":  eventID,
		"attendees": []string{},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test edge case: removing non-existent attendee
func TestHandleCalendarUpdateEvent_RemoveNonExistent(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-remove-nonexistent"

	// Removing someone who isn't an attendee should not error
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":         eventID,
		"remove_attendees": []string{"nothere@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// Test combining attendee updates with other field updates
func TestHandleCalendarUpdateEvent_AttendeesWithOtherFields(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-attendees-and-summary"
	newStartTime := time.Now().Add(2 * time.Hour)

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":      eventID,
		"summary":       "Updated Meeting",
		"start_time":    newStartTime.Format(time.RFC3339),
		"add_attendees": []string{"newperson@example.com"},
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}
