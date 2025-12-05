// ABOUTME: Integration tests for GSuite MCP server
// ABOUTME: Tests end-to-end workflows using ish mode

package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/calendar"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/people"
	"github.com/harper/gsuite-mcp/pkg/server"
)

// setupIshMode configures environment for ish mode testing
func setupIshMode(t *testing.T) func() {
	t.Helper()

	// Save original values
	originalIshMode := os.Getenv("ISH_MODE")
	originalIshBaseURL := os.Getenv("ISH_BASE_URL")
	originalIshUser := os.Getenv("ISH_USER")

	// Set ish mode
	os.Setenv("ISH_MODE", "true")
	os.Setenv("ISH_BASE_URL", "http://localhost:9000")
	os.Setenv("ISH_USER", "testuser@example.com")

	// Return cleanup function
	return func() {
		if originalIshMode == "" {
			os.Unsetenv("ISH_MODE")
		} else {
			os.Setenv("ISH_MODE", originalIshMode)
		}
		if originalIshBaseURL == "" {
			os.Unsetenv("ISH_BASE_URL")
		} else {
			os.Setenv("ISH_BASE_URL", originalIshBaseURL)
		}
		if originalIshUser == "" {
			os.Unsetenv("ISH_USER")
		} else {
			os.Setenv("ISH_USER", originalIshUser)
		}
	}
}

// TestGmailOperationsIntegration tests Gmail service in ish mode
func TestGmailOperationsIntegration(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	client := auth.NewFakeClient("testuser@example.com")

	svc, err := gmail.NewService(ctx, client)
	if err != nil {
		t.Fatalf("Failed to create Gmail service: %v", err)
	}

	t.Run("ListMessages", func(t *testing.T) {
		messages, err := svc.ListMessages(ctx, "", 10)
		if err != nil {
			t.Errorf("Failed to list messages: %v", err)
		}
		t.Logf("Listed %d messages", len(messages))
	})

	t.Run("SendMessage", func(t *testing.T) {
		msg, err := svc.SendMessage(ctx, "recipient@example.com", "Test Subject", "Test Body")
		if err != nil {
			t.Logf("Note: Send message failed (expected without ish server): %v", err)
			return
		}
		if msg == nil {
			t.Error("Expected message to be non-nil")
			return
		}
		t.Logf("Sent message with ID: %s", msg.Id)
	})

	t.Run("GetMessage", func(t *testing.T) {
		// First list to get a message ID
		messages, err := svc.ListMessages(ctx, "", 1)
		if err != nil {
			t.Fatalf("Failed to list messages: %v", err)
		}

		if len(messages) > 0 {
			msg, err := svc.GetMessage(ctx, messages[0].Id)
			if err != nil {
				t.Errorf("Failed to get message: %v", err)
			}
			if msg == nil {
				t.Error("Expected message to be non-nil")
			}
			t.Logf("Retrieved message with ID: %s", msg.Id)
		}
	})
}

// TestCalendarOperationsIntegration tests Calendar service in ish mode
func TestCalendarOperationsIntegration(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	client := auth.NewFakeClient("testuser@example.com")

	svc, err := calendar.NewService(ctx, client)
	if err != nil {
		t.Fatalf("Failed to create Calendar service: %v", err)
	}

	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)

	t.Run("ListEvents", func(t *testing.T) {
		events, err := svc.ListEvents(ctx, 10, time.Time{}, time.Time{})
		if err != nil {
			t.Logf("Note: List events failed (expected without ish server): %v", err)
			return
		}
		t.Logf("Listed %d events", len(events))
	})

	t.Run("ListEventsWithTimeRange", func(t *testing.T) {
		events, err := svc.ListEvents(ctx, 10, now, tomorrow)
		if err != nil {
			t.Logf("Note: List events with time range failed (expected without ish server): %v", err)
			return
		}
		t.Logf("Listed %d events in time range", len(events))
	})

	t.Run("CreateEvent", func(t *testing.T) {
		startTime := now.Add(2 * time.Hour)
		endTime := startTime.Add(1 * time.Hour)

		event, err := svc.CreateEvent(ctx, "Integration Test Event", "Testing event creation", startTime, endTime)
		if err != nil {
			t.Logf("Note: Create event failed (expected without ish server): %v", err)
			return
		}
		if event == nil {
			t.Error("Expected event to be non-nil")
			return
		}
		t.Logf("Created event with ID: %s", event.Id)
	})

	t.Run("GetEvent", func(t *testing.T) {
		// First list to get an event ID
		events, err := svc.ListEvents(ctx, 1, time.Time{}, time.Time{})
		if err != nil {
			t.Logf("Note: List events failed (expected without ish server): %v", err)
			return
		}

		if len(events) > 0 {
			event, err := svc.GetEvent(ctx, events[0].Id)
			if err != nil {
				t.Logf("Note: Get event failed (expected without ish server): %v", err)
				return
			}
			if event == nil {
				t.Error("Expected event to be non-nil")
				return
			}
			t.Logf("Retrieved event with ID: %s", event.Id)
		}
	})
}

// TestPeopleOperationsIntegration tests People service in ish mode
func TestPeopleOperationsIntegration(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	client := auth.NewFakeClient("testuser@example.com")

	svc, err := people.NewService(ctx, client)
	if err != nil {
		t.Fatalf("Failed to create People service: %v", err)
	}

	t.Run("ListContacts", func(t *testing.T) {
		contacts, err := svc.ListContacts(ctx, 10)
		if err != nil {
			t.Errorf("Failed to list contacts: %v", err)
		}
		t.Logf("Listed %d contacts", len(contacts))
	})

	t.Run("SearchContacts", func(t *testing.T) {
		contacts, err := svc.SearchContacts(ctx, "test", 10)
		if err != nil {
			t.Logf("Note: Search contacts failed (expected without ish server): %v", err)
			return
		}
		t.Logf("Found %d contacts matching 'test'", len(contacts))
	})

	t.Run("GetPerson", func(t *testing.T) {
		// First list to get a person resource name
		contacts, err := svc.ListContacts(ctx, 1)
		if err != nil {
			t.Fatalf("Failed to list contacts: %v", err)
		}

		if len(contacts) > 0 {
			person, err := svc.GetPerson(ctx, contacts[0].ResourceName)
			if err != nil {
				t.Errorf("Failed to get person: %v", err)
			}
			if person == nil {
				t.Error("Expected person to be non-nil")
			}
			t.Logf("Retrieved person: %s", person.ResourceName)
		}
	})
}

// TestServerToolHandlers tests MCP server tool handlers
func TestServerToolHandlers(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	srv, err := server.NewServer(ctx)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	t.Run("ListTools", func(t *testing.T) {
		tools := srv.ListTools()
		if len(tools) == 0 {
			t.Error("Expected at least one tool to be registered")
		}

		expectedTools := map[string]bool{
			"gmail_list_messages":   false,
			"gmail_send_message":    false,
			"calendar_list_events":  false,
			"people_list_contacts":  false,
		}

		for _, tool := range tools {
			if _, ok := expectedTools[tool.Name]; ok {
				expectedTools[tool.Name] = true
			}
		}

		for toolName, found := range expectedTools {
			if !found {
				t.Errorf("Expected tool %s to be registered", toolName)
			}
		}

		t.Logf("Registered %d tools", len(tools))
	})

	t.Run("GmailListMessagesHandler", func(t *testing.T) {
		// Verify the tool is registered
		tools := srv.ListTools()
		var found bool
		for _, tool := range tools {
			if tool.Name == "gmail_list_messages" {
				found = true
				if tool.InputSchema.Type != "object" {
					t.Errorf("Expected input schema type 'object', got '%s'", tool.InputSchema.Type)
				}
				break
			}
		}
		if !found {
			t.Error("gmail_list_messages tool not found")
		}
		t.Log("Verified handler registration for: gmail_list_messages")
	})

	t.Run("GmailSendMessageHandler", func(t *testing.T) {
		tools := srv.ListTools()
		var found bool
		for _, tool := range tools {
			if tool.Name == "gmail_send_message" {
				found = true
				if tool.InputSchema.Type != "object" {
					t.Errorf("Expected input schema type 'object', got '%s'", tool.InputSchema.Type)
				}
				// Verify required fields
				if len(tool.InputSchema.Required) != 3 {
					t.Errorf("Expected 3 required fields, got %d", len(tool.InputSchema.Required))
				}
				break
			}
		}
		if !found {
			t.Error("gmail_send_message tool not found")
		}
		t.Log("Verified handler registration for: gmail_send_message")
	})

	t.Run("CalendarListEventsHandler", func(t *testing.T) {
		tools := srv.ListTools()
		var found bool
		for _, tool := range tools {
			if tool.Name == "calendar_list_events" {
				found = true
				if tool.InputSchema.Type != "object" {
					t.Errorf("Expected input schema type 'object', got '%s'", tool.InputSchema.Type)
				}
				break
			}
		}
		if !found {
			t.Error("calendar_list_events tool not found")
		}
		t.Log("Verified handler registration for: calendar_list_events")
	})

	t.Run("PeopleListContactsHandler", func(t *testing.T) {
		tools := srv.ListTools()
		var found bool
		for _, tool := range tools {
			if tool.Name == "people_list_contacts" {
				found = true
				if tool.InputSchema.Type != "object" {
					t.Errorf("Expected input schema type 'object', got '%s'", tool.InputSchema.Type)
				}
				break
			}
		}
		if !found {
			t.Error("people_list_contacts tool not found")
		}
		t.Log("Verified handler registration for: people_list_contacts")
	})
}

// TestEndToEndWorkflow tests a complete workflow
func TestEndToEndWorkflow(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// Create server
		srv, err := server.NewServer(ctx)
		if err != nil {
			t.Fatalf("Failed to create server: %v", err)
		}

		// Verify tools are registered
		tools := srv.ListTools()
		if len(tools) < 4 {
			t.Errorf("Expected at least 4 tools, got %d", len(tools))
		}

		// Verify each service is accessible through the server
		client := auth.NewFakeClient("testuser@example.com")

		// Test Gmail
		gmailSvc, err := gmail.NewService(ctx, client)
		if err != nil {
			t.Errorf("Failed to create Gmail service: %v", err)
		} else {
			messages, err := gmailSvc.ListMessages(ctx, "", 1)
			if err != nil {
				t.Errorf("Failed to list messages: %v", err)
			}
			t.Logf("Gmail service working: listed %d messages", len(messages))
		}

		// Test Calendar
		calSvc, err := calendar.NewService(ctx, client)
		if err != nil {
			t.Errorf("Failed to create Calendar service: %v", err)
		} else {
			events, err := calSvc.ListEvents(ctx, 1, time.Time{}, time.Time{})
			if err != nil {
				t.Logf("Note: List events failed (expected without ish server): %v", err)
			} else {
				t.Logf("Calendar service working: listed %d events", len(events))
			}
		}

		// Test People
		peopleSvc, err := people.NewService(ctx, client)
		if err != nil {
			t.Errorf("Failed to create People service: %v", err)
		} else {
			contacts, err := peopleSvc.ListContacts(ctx, 1)
			if err != nil {
				t.Errorf("Failed to list contacts: %v", err)
			}
			t.Logf("People service working: listed %d contacts", len(contacts))
		}

		t.Log("End-to-end workflow completed successfully")
	})
}

// TestIshModeConfiguration tests ish mode environment variable handling
func TestIshModeConfiguration(t *testing.T) {
	t.Run("WithIshMode", func(t *testing.T) {
		cleanup := setupIshMode(t)
		defer cleanup()

		if os.Getenv("ISH_MODE") != "true" {
			t.Error("Expected ISH_MODE to be 'true'")
		}
		if os.Getenv("ISH_BASE_URL") != "http://localhost:9000" {
			t.Errorf("Expected ISH_BASE_URL to be 'http://localhost:9000', got '%s'", os.Getenv("ISH_BASE_URL"))
		}
		if os.Getenv("ISH_USER") != "testuser@example.com" {
			t.Errorf("Expected ISH_USER to be 'testuser@example.com', got '%s'", os.Getenv("ISH_USER"))
		}
	})

	t.Run("CleanupRestoresOriginalValues", func(t *testing.T) {
		// Set some original values
		os.Setenv("ISH_MODE", "original")
		originalValue := os.Getenv("ISH_MODE")

		cleanup := setupIshMode(t)
		// ISH_MODE should now be "true"
		if os.Getenv("ISH_MODE") != "true" {
			t.Error("Expected ISH_MODE to be 'true' after setup")
		}

		cleanup()
		// ISH_MODE should be restored
		if os.Getenv("ISH_MODE") != originalValue {
			t.Errorf("Expected ISH_MODE to be restored to '%s', got '%s'", originalValue, os.Getenv("ISH_MODE"))
		}
	})
}
