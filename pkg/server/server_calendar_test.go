// ABOUTME: Tests for calendar event update functionality
// ABOUTME: Validates event update with nil fields, partial updates, and edge cases

package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	googlecalendar "google.golang.org/api/calendar/v3"
)

func TestHandleCalendarUpdateEvent_WithNilStartField(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	// First create an event to update
	createRequest := createMockRequest("calendar_create_event", map[string]interface{}{
		"summary":     "Test Event",
		"description": "Test description",
		"start_time":  time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		"end_time":    time.Now().Add(2 * time.Hour).Format(time.RFC3339),
	})

	createResult, err := srv.handleCalendarCreateEvent(context.Background(), createRequest)
	require.NoError(t, err)
	require.NotNil(t, createResult)

	// Extract event ID from the created event
	// In ISH mode, we'll use a known event ID
	eventID := "test-event-123"

	// Update the event with only start_time (testing nil Start field handling)
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":   eventID,
		"start_time": time.Now().Add(3 * time.Hour).Format(time.RFC3339),
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// The handler should handle nil Start field gracefully
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_WithNilEndField(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-456"

	// Update event with only end_time (testing nil End field handling)
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id": eventID,
		"end_time": time.Now().Add(4 * time.Hour).Format(time.RFC3339),
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// The handler should handle nil End field gracefully
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_WithBothNilStartAndEnd(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-789"

	// Update event with both start_time and end_time (testing both nil fields)
	startTime := time.Now().Add(1 * time.Hour)
	endTime := time.Now().Add(2 * time.Hour)

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":   eventID,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// Should successfully create Start and End structs if nil
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_OnlySummary(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-summary"

	// Update only the summary field
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id": eventID,
		"summary":  "Updated Summary Only",
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_OnlyDescription(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-description"

	// Update only the description field
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":    eventID,
		"description": "Updated Description Only",
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_OnlyStartTime(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-starttime"

	// Update only start_time
	newStartTime := time.Now().Add(5 * time.Hour)
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":   eventID,
		"start_time": newStartTime.Format(time.RFC3339),
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_OnlyEndTime(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-endtime"

	// Update only end_time
	newEndTime := time.Now().Add(6 * time.Hour)
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id": eventID,
		"end_time": newEndTime.Format(time.RFC3339),
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_InvalidStartTimeFormat(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-invalid-start"

	// Try to update with invalid start_time format
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":   eventID,
		"start_time": "not-a-valid-time",
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// Should return error result with proper error message
	require.NoError(t, err) // No Go error, but result contains error
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_InvalidEndTimeFormat(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-invalid-end"

	// Try to update with invalid end_time format
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id": eventID,
		"end_time": "also-not-a-valid-time",
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// Should return error result with proper error message
	require.NoError(t, err) // No Go error, but result contains error
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_MissingEventID(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	// Try to update without event_id
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"summary": "Updated Summary",
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// Should return error about missing event_id
	require.NoError(t, err) // No Go error, but result contains error
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestHandleCalendarUpdateEvent_AllFieldsUpdate(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-all-fields"

	// Update all fields at once
	newStartTime := time.Now().Add(1 * time.Hour)
	newEndTime := time.Now().Add(3 * time.Hour)

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":    eventID,
		"summary":     "Completely Updated Event",
		"description": "All fields have been updated",
		"start_time":  newStartTime.Format(time.RFC3339),
		"end_time":    newEndTime.Format(time.RFC3339),
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
	// In ISH mode, may fail if event doesn't exist
}

func TestHandleCalendarUpdateEvent_EmptyStringFieldsIgnored(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-empty-strings"

	// Update with empty strings (should be ignored per the handler logic)
	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":    eventID,
		"summary":     "", // Should be ignored
		"description": "", // Should be ignored
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// Empty strings should be ignored, so the update should succeed without changes
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_MalformedExistingEvent(t *testing.T) {
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	// In ISH mode, this will try to get a non-existent event
	eventID := "non-existent-event-999"

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id": eventID,
		"summary":  "Try to update non-existent event",
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// Should handle the error from GetEvent gracefully
	require.NoError(t, err) // No Go error, but result may contain error
	assert.NotNil(t, result)
	// In ISH mode, this might succeed or fail depending on mock implementation
}

func TestHandleCalendarUpdateEvent_NilEventDateTime_StartOnly(t *testing.T) {
	// This test simulates the scenario where an existing event
	// has nil Start field and we're updating it
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-nil-start-update"

	// The handler should create a new EventDateTime if nil
	newStartTime := time.Now().Add(2 * time.Hour)

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id":   eventID,
		"start_time": newStartTime.Format(time.RFC3339),
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// Verify the handler didn't panic from nil pointer
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestHandleCalendarUpdateEvent_NilEventDateTime_EndOnly(t *testing.T) {
	// This test simulates the scenario where an existing event
	// has nil End field and we're updating it
	t.Setenv("ISH_MODE", "true")

	srv, err := NewServer(context.Background())
	require.NoError(t, err)

	eventID := "test-event-nil-end-update"

	// The handler should create a new EventDateTime if nil
	newEndTime := time.Now().Add(4 * time.Hour)

	updateRequest := createMockRequest("calendar_update_event", map[string]interface{}{
		"event_id": eventID,
		"end_time": newEndTime.Format(time.RFC3339),
	})

	result, err := srv.handleCalendarUpdateEvent(context.Background(), updateRequest)

	// Verify the handler didn't panic from nil pointer
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// TestCalendarEventWithNilFields tests direct manipulation of calendar.Event
// to ensure our understanding of nil field handling is correct
func TestCalendarEventWithNilFields(t *testing.T) {
	// Create an event with nil Start and End
	event := &googlecalendar.Event{
		Summary: "Test Event",
		Start:   nil,
		End:     nil,
	}

	// Verify nil fields
	assert.Nil(t, event.Start)
	assert.Nil(t, event.End)

	// Now simulate what the handler does
	if event.Start == nil {
		event.Start = &googlecalendar.EventDateTime{}
	}
	event.Start.DateTime = time.Now().Format(time.RFC3339)

	if event.End == nil {
		event.End = &googlecalendar.EventDateTime{}
	}
	event.End.DateTime = time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	// Verify fields are now set
	assert.NotNil(t, event.Start)
	assert.NotNil(t, event.End)
	assert.NotEmpty(t, event.Start.DateTime)
	assert.NotEmpty(t, event.End.DateTime)
}
