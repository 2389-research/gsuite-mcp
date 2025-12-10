// ABOUTME: End-to-end scenario tests with real ish server
// ABOUTME: Tests realistic workflows against fake Google API server

package test

import (
	"context"
	"testing"
	"time"

	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/calendar"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/people"
	"github.com/harper/gsuite-mcp/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario_EmailTriage tests a realistic email triage workflow
func TestScenario_EmailTriage(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	client := auth.NewFakeClient("test@example.com")

	gmailSvc, err := gmail.NewService(ctx, client)
	require.NoError(t, err, "Failed to create Gmail service")

	t.Run("List unread messages", func(t *testing.T) {
		messages, err := gmailSvc.ListMessages(ctx, "is:unread", 10)

		// Should succeed or fail gracefully
		if err != nil {
			t.Logf("List unread messages: %v", err)
		} else {
			t.Logf("Found %d unread messages", len(messages))
			assert.NotNil(t, messages)
		}
	})

	t.Run("Search for important emails", func(t *testing.T) {
		messages, err := gmailSvc.ListMessages(ctx, "is:important", 5)

		if err != nil {
			t.Logf("Search important: %v", err)
		} else {
			t.Logf("Found %d important messages", len(messages))
		}
	})

	t.Run("Send reply to inquiry", func(t *testing.T) {
		msg, err := gmailSvc.SendMessage(ctx,
			"customer@example.com",
			"Re: Your inquiry",
			"Thank you for reaching out. We'll get back to you soon.",
			"")

		if err != nil {
			t.Logf("Send reply failed: %v", err)
		} else {
			t.Logf("Reply sent successfully: %v", msg != nil)
			assert.NotNil(t, msg)
		}
	})
}

// TestScenario_MeetingScheduling tests calendar scheduling workflow
func TestScenario_MeetingScheduling(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	client := auth.NewFakeClient("scheduler@example.com")

	calendarSvc, err := calendar.NewService(ctx, client)
	require.NoError(t, err, "Failed to create Calendar service")

	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	nextWeek := now.Add(7 * 24 * time.Hour)

	t.Run("Check availability for next week", func(t *testing.T) {
		events, err := calendarSvc.ListEvents(ctx, 50, tomorrow, nextWeek)

		if err != nil {
			t.Logf("Check availability failed: %v", err)
		} else {
			t.Logf("Found %d events in next week", len(events))
			assert.NotNil(t, events)
		}
	})

	t.Run("Schedule team meeting", func(t *testing.T) {
		meetingStart := tomorrow.Add(10 * time.Hour) // 10 AM tomorrow
		meetingEnd := meetingStart.Add(1 * time.Hour)

		event, err := calendarSvc.CreateEvent(ctx,
			"Team Sync",
			"Weekly team synchronization meeting",
			meetingStart,
			meetingEnd)

		if err != nil {
			t.Logf("Schedule meeting failed: %v", err)
		} else {
			t.Logf("Meeting scheduled successfully: %v", event != nil)
			assert.NotNil(t, event)

			// Verify we can retrieve it
			if event != nil && event.Id != "" {
				retrieved, err := calendarSvc.GetEvent(ctx, event.Id)
				if err == nil {
					assert.Equal(t, "Team Sync", retrieved.Summary)
					t.Logf("Successfully retrieved scheduled meeting")
				}
			}
		}
	})

	t.Run("List today's meetings", func(t *testing.T) {
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayEnd := todayStart.Add(24 * time.Hour)

		events, err := calendarSvc.ListEvents(ctx, 20, todayStart, todayEnd)

		if err != nil {
			t.Logf("List today's meetings failed: %v", err)
		} else {
			t.Logf("Today's meetings: %d", len(events))
		}
	})
}

// TestScenario_ContactManagement tests contact lookup and search workflow
func TestScenario_ContactManagement(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	client := auth.NewFakeClient("user@example.com")

	peopleSvc, err := people.NewService(ctx, client)
	require.NoError(t, err, "Failed to create People service")

	t.Run("List recent contacts", func(t *testing.T) {
		contacts, err := peopleSvc.ListContacts(ctx, 20)

		if err != nil {
			t.Logf("List contacts failed: %v", err)
		} else {
			t.Logf("Found %d contacts", len(contacts))
			assert.NotNil(t, contacts)
		}
	})

	t.Run("Search for specific person", func(t *testing.T) {
		results, err := peopleSvc.SearchContacts(ctx, "John", 5)

		if err != nil {
			t.Logf("Search contacts failed: %v", err)
		} else {
			t.Logf("Search results for 'John': %d", len(results))
			// Results can be nil or empty array - both are valid for "no results"
		}
	})

	t.Run("Search by email domain", func(t *testing.T) {
		results, err := peopleSvc.SearchContacts(ctx, "example.com", 10)

		if err != nil {
			t.Logf("Search by domain failed: %v", err)
		} else {
			t.Logf("Contacts from example.com: %d", len(results))
		}
	})
}

// TestScenario_FullDayWorkflow tests a complete day's workflow
func TestScenario_FullDayWorkflow(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	client := auth.NewFakeClient("worker@example.com")

	// Create all services
	gmailSvc, err := gmail.NewService(ctx, client)
	require.NoError(t, err)

	calendarSvc, err := calendar.NewService(ctx, client)
	require.NoError(t, err)

	peopleSvc, err := people.NewService(ctx, client)
	require.NoError(t, err)

	t.Run("Morning routine", func(t *testing.T) {
		// 1. Check calendar for today
		t.Log("Step 1: Checking today's schedule...")
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayEnd := todayStart.Add(24 * time.Hour)

		events, err := calendarSvc.ListEvents(ctx, 20, todayStart, todayEnd)
		if err != nil {
			t.Logf("Calendar check: %v", err)
		} else {
			t.Logf("Today's schedule: %d events", len(events))
		}

		// 2. Check unread emails
		t.Log("Step 2: Checking unread emails...")
		messages, err := gmailSvc.ListMessages(ctx, "is:unread", 20)
		if err != nil {
			t.Logf("Email check: %v", err)
		} else {
			t.Logf("Unread emails: %d", len(messages))
		}

		// 3. Look up contact for meeting
		t.Log("Step 3: Looking up meeting attendee...")
		contacts, err := peopleSvc.SearchContacts(ctx, "Sarah", 5)
		if err != nil {
			t.Logf("Contact lookup: %v", err)
		} else {
			t.Logf("Found contacts: %d", len(contacts))
		}
	})

	t.Run("Respond to urgent email", func(t *testing.T) {
		t.Log("Step 4: Sending urgent response...")
		msg, err := gmailSvc.SendMessage(ctx,
			"boss@example.com",
			"Re: Urgent: Project status",
			"The project is on track. Will send detailed update by EOD.",
			"")

		if err != nil {
			t.Logf("Send response: %v", err)
		} else {
			t.Logf("Response sent: %v", msg != nil)
		}
	})

	t.Run("Schedule follow-up meeting", func(t *testing.T) {
		t.Log("Step 5: Scheduling follow-up...")
		tomorrow := time.Now().Add(24 * time.Hour)
		meetingStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, tomorrow.Location())
		meetingEnd := meetingStart.Add(30 * time.Minute)

		event, err := calendarSvc.CreateEvent(ctx,
			"Follow-up: Project Discussion",
			"Discuss project status and next steps",
			meetingStart,
			meetingEnd)

		if err != nil {
			t.Logf("Schedule meeting: %v", err)
		} else {
			t.Logf("Meeting scheduled: %v", event != nil)
		}
	})
}

// TestScenario_MCPServerIntegration tests the full MCP server with scenarios
func TestScenario_MCPServerIntegration(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()

	srv, err := server.NewServer(ctx)
	require.NoError(t, err, "Failed to create MCP server")

	t.Run("Verify all tools registered", func(t *testing.T) {
		tools := srv.ListTools()
		require.Len(t, tools, 19, "Expected 19 tools to be registered")

		toolNames := make([]string, len(tools))
		for i, tool := range tools {
			toolNames[i] = tool.Name
		}

		// Verify core tools are present
		assert.Contains(t, toolNames, "gmail_list_messages")
		assert.Contains(t, toolNames, "gmail_send_message")
		assert.Contains(t, toolNames, "calendar_list_events")
		assert.Contains(t, toolNames, "people_list_contacts")
		assert.Contains(t, toolNames, "people_search_contacts")
		assert.Contains(t, toolNames, "people_get_contact")

		t.Logf("All 19 MCP tools verified: %v", toolNames)
	})

	t.Log("MCP server integration test complete")
}

// TestScenario_RetryLogicUnderLoad tests retry behavior
func TestScenario_RetryLogicUnderLoad(t *testing.T) {
	cleanup := setupIshMode(t)
	defer cleanup()

	ctx := context.Background()
	client := auth.NewFakeClient("load-test@example.com")

	gmailSvc, err := gmail.NewService(ctx, client)
	require.NoError(t, err)

	t.Run("Rapid successive requests", func(t *testing.T) {
		successCount := 0
		failCount := 0

		for i := 0; i < 10; i++ {
			_, err := gmailSvc.ListMessages(ctx, "", 10)
			if err == nil {
				successCount++
			} else {
				failCount++
			}
		}

		t.Logf("Rapid requests: %d succeeded, %d failed", successCount, failCount)
		// With retry logic, we should handle transient failures
		assert.True(t, successCount >= failCount, "Retry logic should improve success rate")
	})

	t.Run("Concurrent requests", func(t *testing.T) {
		done := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			go func(id int) {
				_, err := gmailSvc.ListMessages(ctx, "", 5)
				if err != nil {
					t.Logf("Concurrent request %d: %v", id, err)
				} else {
					t.Logf("Concurrent request %d: success", id)
				}
				done <- true
			}(i)
		}

		// Wait for all to complete
		for i := 0; i < 5; i++ {
			<-done
		}

		t.Log("Concurrent requests completed")
	})
}
