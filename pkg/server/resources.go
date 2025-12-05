// ABOUTME: MCP resources exposing GSuite data
// ABOUTME: Dynamic resources that provide access to calendar events, emails, and contacts

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerResources registers all MCP resources
func (s *Server) registerResources() {
	// Today's calendar events
	s.mcp.AddResource(
		mcp.NewResource(
			"gsuite://calendar/today",
			"Today's Calendar",
			mcp.WithResourceDescription("Calendar events for today"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleTodayCalendarResource,
	)

	// This week's calendar events
	s.mcp.AddResource(
		mcp.NewResource(
			"gsuite://calendar/this-week",
			"This Week's Calendar",
			mcp.WithResourceDescription("Calendar events for this week"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleThisWeekCalendarResource,
	)

	// Unread emails summary
	s.mcp.AddResource(
		mcp.NewResource(
			"gsuite://gmail/unread",
			"Unread Emails",
			mcp.WithResourceDescription("Summary of unread emails"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleUnreadEmailsResource,
	)

	// Important unread emails
	s.mcp.AddResource(
		mcp.NewResource(
			"gsuite://gmail/unread/important",
			"Important Unread Emails",
			mcp.WithResourceDescription("Unread emails marked as important"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleImportantEmailsResource,
	)

	// Recent contacts
	s.mcp.AddResource(
		mcp.NewResource(
			"gsuite://contacts/recent",
			"Recent Contacts",
			mcp.WithResourceDescription("Recently added or modified contacts"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleRecentContactsResource,
	)

	// Upcoming meetings
	s.mcp.AddResource(
		mcp.NewResource(
			"gsuite://calendar/upcoming",
			"Upcoming Meetings",
			mcp.WithResourceDescription("Next 5 upcoming calendar events"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleUpcomingMeetingsResource,
	)

	// Calendar availability
	s.mcp.AddResource(
		mcp.NewResource(
			"gsuite://calendar/availability",
			"Calendar Availability",
			mcp.WithResourceDescription("Free/busy status for the next 7 days"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleCalendarAvailabilityResource,
	)

	// Draft emails
	s.mcp.AddResource(
		mcp.NewResource(
			"gsuite://gmail/drafts",
			"Draft Emails",
			mcp.WithResourceDescription("Current draft emails"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleDraftsResource,
	)
}

// Resource handlers

func (s *Server) handleTodayCalendarResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	events, err := s.calendar.ListEvents(ctx, 50, startOfDay, endOfDay)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch today's events: %w", err)
	}

	// Format as JSON
	data, err := json.MarshalIndent(map[string]interface{}{
		"date":        startOfDay.Format("2006-01-02"),
		"event_count": len(events),
		"events":      events,
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleThisWeekCalendarResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	now := time.Now()
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// Adjust to Monday if not already
	for startOfWeek.Weekday() != time.Monday {
		startOfWeek = startOfWeek.Add(-24 * time.Hour)
	}
	endOfWeek := startOfWeek.Add(7 * 24 * time.Hour)

	events, err := s.calendar.ListEvents(ctx, 100, startOfWeek, endOfWeek)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch this week's events: %w", err)
	}

	// Group by day
	eventsByDay := make(map[string]interface{})
	for _, event := range events {
		// Extract date from event start time
		var eventDate string
		if event.Start != nil {
			if event.Start.DateTime != "" {
				t, err := time.Parse(time.RFC3339, event.Start.DateTime)
				if err == nil {
					eventDate = t.Format("2006-01-02")
				}
			} else if event.Start.Date != "" {
				// All-day event
				eventDate = event.Start.Date
			}
		}

		if eventDate != "" {
			if eventsByDay[eventDate] == nil {
				eventsByDay[eventDate] = []interface{}{}
			}
			dayEvents := eventsByDay[eventDate].([]interface{})
			eventsByDay[eventDate] = append(dayEvents, event)
		}
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"week_start":  startOfWeek.Format("2006-01-02"),
		"week_end":    endOfWeek.Format("2006-01-02"),
		"event_count": len(events),
		"events_by_day": eventsByDay,
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleUnreadEmailsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	messages, err := s.gmail.ListMessages(ctx, "is:unread", 20)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unread emails: %w", err)
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"unread_count": len(messages),
		"messages":     messages,
		"timestamp":    time.Now().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleImportantEmailsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	messages, err := s.gmail.ListMessages(ctx, "is:unread is:important", 10)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch important emails: %w", err)
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"important_unread_count": len(messages),
		"messages":               messages,
		"timestamp":              time.Now().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleRecentContactsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	contacts, err := s.people.ListContacts(ctx, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent contacts: %w", err)
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"contact_count": len(contacts),
		"contacts":      contacts,
		"timestamp":     time.Now().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleUpcomingMeetingsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	now := time.Now()
	// Get events for next 7 days
	endTime := now.Add(7 * 24 * time.Hour)

	events, err := s.calendar.ListEvents(ctx, 5, now, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch upcoming meetings: %w", err)
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"upcoming_count": len(events),
		"events":         events,
		"time_range": map[string]string{
			"from": now.Format(time.RFC3339),
			"to":   endTime.Format(time.RFC3339),
		},
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleCalendarAvailabilityResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	now := time.Now()
	endTime := now.Add(7 * 24 * time.Hour)

	events, err := s.calendar.ListEvents(ctx, 100, now, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendar for availability: %w", err)
	}

	// Calculate free/busy slots by day
	availability := make(map[string]interface{})

	for day := 0; day < 7; day++ {
		currentDay := now.Add(time.Duration(day) * 24 * time.Hour)
		dayKey := currentDay.Format("2006-01-02")
		dayName := currentDay.Format("Monday")

		busyHours := 0.0
		dayEvents := 0

		// Count busy hours for this day
		for _, event := range events {
			if event.Start == nil || event.End == nil {
				continue
			}

			// Parse start time
			var startTime time.Time
			var err error
			if event.Start.DateTime != "" {
				startTime, err = time.Parse(time.RFC3339, event.Start.DateTime)
				if err != nil {
					continue
				}
			} else {
				// Skip all-day events for busy hours calculation
				continue
			}

			// Check if event is on this day
			if startTime.Format("2006-01-02") == dayKey {
				// Parse end time
				var endTime time.Time
				if event.End.DateTime != "" {
					endTime, err = time.Parse(time.RFC3339, event.End.DateTime)
					if err != nil {
						continue
					}
					duration := endTime.Sub(startTime).Hours()
					busyHours += duration
					dayEvents++
				}
			}
		}

		// Business hours: 8 AM - 6 PM = 10 hours
		freeHours := 10.0 - busyHours
		if freeHours < 0 {
			freeHours = 0
		}

		availability[dayKey] = map[string]interface{}{
			"day_name":    dayName,
			"busy_hours":  busyHours,
			"free_hours":  freeHours,
			"event_count": dayEvents,
			"status":      getAvailabilityStatus(busyHours),
		}
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"period": "next 7 days",
		"availability": availability,
		"generated_at": now.Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleDraftsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// List draft emails
	drafts, err := s.gmail.ListDrafts(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch drafts: %w", err)
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"draft_count": len(drafts),
		"drafts":      drafts,
		"timestamp":   time.Now().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

// Helper functions

func getAvailabilityStatus(busyHours float64) string {
	if busyHours < 3 {
		return "available"
	} else if busyHours < 6 {
		return "moderate"
	} else if busyHours < 8 {
		return "busy"
	}
	return "very_busy"
}
