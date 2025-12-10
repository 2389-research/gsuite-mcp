// ABOUTME: Debug test to capture actual HTTP requests made
// ABOUTME: Shows exact URLs being called by Google API client

package test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/calendar"
	"github.com/harper/gsuite-mcp/pkg/people"
	"github.com/stretchr/testify/require"
)

// TestDebugRequests captures and prints actual HTTP requests
func TestDebugRequests(t *testing.T) {
	// Create a test server that logs all requests
	var capturedRequests []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		if r.URL.RawQuery != "" {
			url += "?" + r.URL.RawQuery
		}
		capturedRequests = append(capturedRequests, url)
		t.Logf("Request: %s %s", r.Method, r.URL.String())

		// Return empty valid responses
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/gmail/v1/users/me/messages" {
			_, _ = w.Write([]byte(`{"messages":[],"resultSizeEstimate":0}`))
		} else if r.URL.Path == "/gmail/v1/users/me/messages/send" {
			_, _ = w.Write([]byte(`{"id":"msg123","threadId":"thread123"}`))
		} else if r.URL.Path == "/calendar/v3/calendars/primary/events" && r.Method == "GET" {
			_, _ = w.Write([]byte(`{"items":[]}`))
		} else if r.URL.Path == "/calendar/v3/calendars/primary/events" && r.Method == "POST" {
			_, _ = w.Write([]byte(`{"id":"event123","summary":"Test"}`))
		} else if r.URL.Path == "/v1/people/me/connections" {
			_, _ = w.Write([]byte(`{"connections":[]}`))
		} else if r.URL.Path == "/v1/people:searchContacts" {
			_, _ = w.Write([]byte(`{"results":[]}`))
		} else {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer server.Close()

	ctx := context.Background()
	client := auth.NewFakeClient("debug@example.com")

	t.Run("Gmail ListMessages", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		t.Setenv("ISH_BASE_URL", server.URL)

		svc, err := gmail.NewService(ctx, client)
		require.NoError(t, err)

		_, _ = svc.ListMessages(ctx, "is:unread", 10)
	})

	t.Run("Gmail SendMessage", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		t.Setenv("ISH_BASE_URL", server.URL)

		svc, err := gmail.NewService(ctx, client)
		require.NoError(t, err)

		_, _ = svc.SendMessage(ctx, "test@example.com", "Subject", "Body", "")
	})

	t.Run("Calendar ListEvents", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		t.Setenv("ISH_BASE_URL", server.URL)

		svc, err := calendar.NewService(ctx, client)
		require.NoError(t, err)

		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)
		_, _ = svc.ListEvents(ctx, 10, now, tomorrow)
	})

	t.Run("Calendar CreateEvent", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		t.Setenv("ISH_BASE_URL", server.URL)

		svc, err := calendar.NewService(ctx, client)
		require.NoError(t, err)

		start := time.Now().Add(24 * time.Hour)
		end := start.Add(1 * time.Hour)
		_, _ = svc.CreateEvent(ctx, "Meeting", "Description", start, end)
	})

	t.Run("People ListContacts", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		t.Setenv("ISH_BASE_URL", server.URL)

		svc, err := people.NewService(ctx, client)
		require.NoError(t, err)

		_, _ = svc.ListContacts(ctx, 10)
	})

	t.Run("People SearchContacts", func(t *testing.T) {
		t.Setenv("ISH_MODE", "true")
		t.Setenv("ISH_BASE_URL", server.URL)

		svc, err := people.NewService(ctx, client)
		require.NoError(t, err)

		_, _ = svc.SearchContacts(ctx, "John", 5)
	})

	t.Log("\n=== CAPTURED REQUESTS ===")
	for _, req := range capturedRequests {
		t.Logf("  %s", req)
	}
}
