// ABOUTME: Calendar API service for event management
// ABOUTME: Handles calendar events, creation, and listing

package calendar

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/harper/gsuite-mcp/pkg/retry"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Service wraps Calendar API operations
type Service struct {
	svc *calendar.Service
}

// NewService creates a new Calendar service
func NewService(ctx context.Context, client *http.Client) (*Service, error) {
	opts := []option.ClientOption{}

	// Check for ish mode
	if os.Getenv("ISH_MODE") == "true" {
		baseURL := os.Getenv("ISH_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:9000"
		}
		opts = append(opts, option.WithEndpoint(baseURL))
		opts = append(opts, option.WithoutAuthentication())
	}

	if client != nil {
		opts = append(opts, option.WithHTTPClient(client))
	}

	svc, err := calendar.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create Calendar service: %w", err)
	}

	return &Service{svc: svc}, nil
}

// ListEvents lists events from the primary calendar
func (s *Service) ListEvents(ctx context.Context, maxResults int64, timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	var events *calendar.Events

	err := retry.WithRetry(func() error {
		call := s.svc.Events.List("primary").
			Context(ctx).
			MaxResults(maxResults).
			SingleEvents(true).
			OrderBy("startTime")

		if !timeMin.IsZero() {
			call = call.TimeMin(timeMin.Format(time.RFC3339))
		}

		if !timeMax.IsZero() {
			call = call.TimeMax(timeMax.Format(time.RFC3339))
		}

		var err error
		events, err = call.Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to list events: %w", err)
	}

	return events.Items, nil
}

// CreateEvent creates a new calendar event
func (s *Service) CreateEvent(ctx context.Context, summary, description string, startTime, endTime time.Time, attendees []string, optionalAttendees []string, sendNotifications bool) (*calendar.Event, error) {
	event := &calendar.Event{
		Summary:     summary,
		Description: description,
		Start: &calendar.EventDateTime{
			DateTime: startTime.Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: endTime.Format(time.RFC3339),
		},
	}

	// Build attendee list
	var eventAttendees []*calendar.EventAttendee

	// Add required attendees
	for _, email := range attendees {
		eventAttendees = append(eventAttendees, &calendar.EventAttendee{
			Email:    email,
			Optional: false,
		})
	}

	// Add optional attendees
	for _, email := range optionalAttendees {
		eventAttendees = append(eventAttendees, &calendar.EventAttendee{
			Email:    email,
			Optional: true,
		})
	}

	// Only set attendees if we have any
	if len(eventAttendees) > 0 {
		event.Attendees = eventAttendees
	}

	var created *calendar.Event
	err := retry.WithRetry(func() error {
		var err error
		created, err = s.svc.Events.Insert("primary", event).
			Context(ctx).
			SendNotifications(sendNotifications).
			Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to create event: %w", err)
	}

	return created, nil
}

// GetEvent retrieves a specific event
func (s *Service) GetEvent(ctx context.Context, eventID string) (*calendar.Event, error) {
	var event *calendar.Event

	err := retry.WithRetry(func() error {
		var err error
		event, err = s.svc.Events.Get("primary", eventID).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to get event: %w", err)
	}
	return event, nil
}

// UpdateEvent updates an existing event
func (s *Service) UpdateEvent(ctx context.Context, eventID string, event *calendar.Event) (*calendar.Event, error) {
	var updated *calendar.Event

	err := retry.WithRetry(func() error {
		var err error
		updated, err = s.svc.Events.Update("primary", eventID, event).Context(ctx).Do()
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("unable to update event: %w", err)
	}
	return updated, nil
}

// DeleteEvent deletes an event
func (s *Service) DeleteEvent(ctx context.Context, eventID string) error {
	err := retry.WithRetry(func() error {
		return s.svc.Events.Delete("primary", eventID).Context(ctx).Do()
	}, 3, time.Second)

	if err != nil {
		return fmt.Errorf("unable to delete event: %w", err)
	}
	return nil
}
