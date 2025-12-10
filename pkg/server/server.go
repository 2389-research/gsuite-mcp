// ABOUTME: MCP server implementation
// ABOUTME: Exposes Gmail, Calendar, and People services as MCP tools

package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/calendar"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/people"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	googlecalendar "google.golang.org/api/calendar/v3"
	googlepeople "google.golang.org/api/people/v1"
)

// Server is the MCP server for GSuite APIs
type Server struct {
	gmail    *gmail.Service
	calendar *calendar.Service
	people   *people.Service
	mcp      *server.MCPServer
}

// NewServer creates a new MCP server
func NewServer(ctx context.Context) (*Server, error) {
	var client *http.Client

	// Check for ish mode
	if os.Getenv("ISH_MODE") == "true" {
		client = auth.NewFakeClient("")
	} else {
		// Use real OAuth
		authenticator, err := auth.NewAuthenticator(auth.GetCredentialsPath(), auth.GetTokenPath())
		if err != nil {
			return nil, err
		}
		client, err = authenticator.GetClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Create services
	gmailSvc, err := gmail.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service: %w", err)
	}

	calendarSvc, err := calendar.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar service: %w", err)
	}

	peopleSvc, err := people.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create People service: %w", err)
	}

	s := &Server{
		gmail:    gmailSvc,
		calendar: calendarSvc,
		people:   peopleSvc,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"gsuite-mcp",
		"1.0.0",
	)

	s.mcp = mcpServer
	s.registerTools()
	s.registerPrompts()
	s.registerResources()

	return s, nil
}

// registerTools registers all available tools
func (s *Server) registerTools() {
	// Gmail tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_list_messages",
		Description: "List Gmail messages",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query":       map[string]string{"type": "string", "description": "Gmail search query (e.g., 'from:me is:unread')"},
				"max_results": map[string]string{"type": "integer", "description": "Maximum number of messages to return (default: 100)"},
				"hydrate": map[string]interface{}{
					"type":        "boolean",
					"description": "When true, fetches full message details (from, subject, snippet, date). When false/omitted, returns only message IDs.",
				},
			},
		},
	}, s.handleGmailListMessages)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_get_message",
		Description: "Get a specific email message by ID",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"message_id": map[string]string{"type": "string", "description": "The message ID to retrieve"},
			},
			Required: []string{"message_id"},
		},
	}, s.handleGmailGetMessage)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_send_message",
		Description: "Send an email",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"to":      map[string]string{"type": "string"},
				"subject": map[string]string{"type": "string"},
				"body":    map[string]string{"type": "string"},
			},
			Required: []string{"to", "subject", "body"},
		},
	}, s.handleGmailSendMessage)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_create_draft",
		Description: "Create a draft email",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"to":      map[string]string{"type": "string"},
				"subject": map[string]string{"type": "string"},
				"body":    map[string]string{"type": "string"},
			},
			Required: []string{"to", "subject", "body"},
		},
	}, s.handleGmailCreateDraft)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_send_draft",
		Description: "Send an existing draft",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"draft_id": map[string]string{"type": "string", "description": "The draft ID to send"},
			},
			Required: []string{"draft_id"},
		},
	}, s.handleGmailSendDraft)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_modify_labels",
		Description: "Add or remove labels from a message (archive, star, mark as read, etc.)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"message_id": map[string]string{"type": "string", "description": "The message ID to modify"},
				"add_labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Label IDs to add (e.g., STARRED, IMPORTANT)",
				},
				"remove_labels": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Label IDs to remove (e.g., UNREAD, INBOX)",
				},
			},
			Required: []string{"message_id"},
		},
	}, s.handleGmailModifyLabels)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_trash_message",
		Description: "Move a message to trash",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"message_id": map[string]string{"type": "string", "description": "The message ID to trash"},
			},
			Required: []string{"message_id"},
		},
	}, s.handleGmailTrashMessage)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_delete_message",
		Description: "Permanently delete a message",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"message_id": map[string]string{"type": "string", "description": "The message ID to delete permanently"},
			},
			Required: []string{"message_id"},
		},
	}, s.handleGmailDeleteMessage)

	// Calendar tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "calendar_list_events",
		Description: "List calendar events",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"max_results": map[string]string{"type": "integer"},
				"time_min":    map[string]string{"type": "string", "description": "RFC3339 timestamp for earliest event"},
				"time_max":    map[string]string{"type": "string", "description": "RFC3339 timestamp for latest event"},
			},
		},
	}, s.handleCalendarListEvents)

	s.mcp.AddTool(mcp.Tool{
		Name:        "calendar_get_event",
		Description: "Get a specific calendar event by ID",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"event_id": map[string]string{"type": "string", "description": "The event ID to retrieve"},
			},
			Required: []string{"event_id"},
		},
	}, s.handleCalendarGetEvent)

	s.mcp.AddTool(mcp.Tool{
		Name:        "calendar_create_event",
		Description: "Create a new calendar event",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"summary":     map[string]string{"type": "string", "description": "Event title/summary"},
				"description": map[string]string{"type": "string", "description": "Event description"},
				"start_time":  map[string]string{"type": "string", "description": "Start time in RFC3339 format"},
				"end_time":    map[string]string{"type": "string", "description": "End time in RFC3339 format"},
			},
			Required: []string{"summary", "start_time", "end_time"},
		},
	}, s.handleCalendarCreateEvent)

	s.mcp.AddTool(mcp.Tool{
		Name:        "calendar_update_event",
		Description: "Update an existing calendar event",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"event_id":    map[string]string{"type": "string", "description": "The event ID to update"},
				"summary":     map[string]string{"type": "string", "description": "New event title/summary"},
				"description": map[string]string{"type": "string", "description": "New event description"},
				"start_time":  map[string]string{"type": "string", "description": "New start time in RFC3339 format"},
				"end_time":    map[string]string{"type": "string", "description": "New end time in RFC3339 format"},
			},
			Required: []string{"event_id"},
		},
	}, s.handleCalendarUpdateEvent)

	s.mcp.AddTool(mcp.Tool{
		Name:        "calendar_delete_event",
		Description: "Delete a calendar event",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"event_id": map[string]string{"type": "string", "description": "The event ID to delete"},
			},
			Required: []string{"event_id"},
		},
	}, s.handleCalendarDeleteEvent)

	// People tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "people_list_contacts",
		Description: "List contacts",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"page_size": map[string]string{"type": "integer"},
			},
		},
	}, s.handlePeopleListContacts)

	s.mcp.AddTool(mcp.Tool{
		Name:        "people_search_contacts",
		Description: "Search contacts by name, email, or phone number",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query":     map[string]string{"type": "string", "description": "Search query (name, email, phone, etc)"},
				"page_size": map[string]string{"type": "integer"},
			},
			Required: []string{"query"},
		},
	}, s.handlePeopleSearchContacts)

	s.mcp.AddTool(mcp.Tool{
		Name:        "people_get_contact",
		Description: "Get detailed information about a specific contact",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"resource_name": map[string]string{"type": "string", "description": "Resource name of the person (e.g., people/12345)"},
			},
			Required: []string{"resource_name"},
		},
	}, s.handlePeopleGetContact)

	s.mcp.AddTool(mcp.Tool{
		Name:        "people_create_contact",
		Description: "Create a new contact",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"given_name":  map[string]string{"type": "string", "description": "First name"},
				"family_name": map[string]string{"type": "string", "description": "Last name"},
				"email":       map[string]string{"type": "string", "description": "Email address"},
				"phone":       map[string]string{"type": "string", "description": "Phone number"},
			},
			Required: []string{"given_name"},
		},
	}, s.handlePeopleCreateContact)

	s.mcp.AddTool(mcp.Tool{
		Name:        "people_update_contact",
		Description: "Update an existing contact",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"resource_name": map[string]string{"type": "string", "description": "Resource name of the person (e.g., people/12345)"},
				"given_name":    map[string]string{"type": "string", "description": "First name"},
				"family_name":   map[string]string{"type": "string", "description": "Last name"},
				"email":         map[string]string{"type": "string", "description": "Email address"},
				"phone":         map[string]string{"type": "string", "description": "Phone number"},
			},
			Required: []string{"resource_name"},
		},
	}, s.handlePeopleUpdateContact)

	s.mcp.AddTool(mcp.Tool{
		Name:        "people_delete_contact",
		Description: "Delete a contact",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"resource_name": map[string]string{"type": "string", "description": "Resource name of the person (e.g., people/12345)"},
			},
			Required: []string{"resource_name"},
		},
	}, s.handlePeopleDeleteContact)
}

// HydratedMessage is a summary of a Gmail message with common fields extracted
type HydratedMessage struct {
	ID       string   `json:"id"`
	ThreadID string   `json:"threadId"`
	From     string   `json:"from,omitempty"`
	To       string   `json:"to,omitempty"`
	Subject  string   `json:"subject,omitempty"`
	Snippet  string   `json:"snippet,omitempty"`
	Date     string   `json:"date,omitempty"`
	LabelIDs []string `json:"labelIds,omitempty"`
}

// ListMessagesResponse wraps message list results for MCP structuredContent
type ListMessagesResponse struct {
	Messages []HydratedMessage `json:"messages"`
	Count    int               `json:"count"`
}

// ListEventsResponse wraps calendar event list results for MCP structuredContent
type ListEventsResponse struct {
	Events any `json:"events"`
	Count  int `json:"count"`
}

// ListContactsResponse wraps contact list results for MCP structuredContent
type ListContactsResponse struct {
	Contacts any `json:"contacts"`
	Count    int `json:"count"`
}

// Tool handlers
func (s *Server) handleGmailListMessages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.GetString("query", "")
	maxResults := int64(request.GetInt("max_results", 100))
	hydrate := request.GetBool("hydrate", false)

	messages, err := s.gmail.ListMessages(ctx, query, maxResults)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if !hydrate {
		// Wrap in object for MCP structuredContent compatibility
		result := make([]HydratedMessage, len(messages))
		for i, msg := range messages {
			result[i] = HydratedMessage{
				ID:       msg.Id,
				ThreadID: msg.ThreadId,
			}
		}
		return mcp.NewToolResultJSON(ListMessagesResponse{
			Messages: result,
			Count:    len(result),
		})
	}

	// Hydrate: fetch full details for each message
	hydrated := make([]HydratedMessage, 0, len(messages))
	for _, msg := range messages {
		fullMsg, err := s.gmail.GetMessage(ctx, msg.Id)
		if err != nil {
			// If we can't get one message, include basic info and continue
			hydrated = append(hydrated, HydratedMessage{
				ID:       msg.Id,
				ThreadID: msg.ThreadId,
			})
			continue
		}

		hm := HydratedMessage{
			ID:       fullMsg.Id,
			ThreadID: fullMsg.ThreadId,
			Snippet:  fullMsg.Snippet,
			LabelIDs: fullMsg.LabelIds,
		}

		// Extract headers
		if fullMsg.Payload != nil {
			for _, header := range fullMsg.Payload.Headers {
				switch strings.ToLower(header.Name) {
				case "from":
					hm.From = header.Value
				case "to":
					hm.To = header.Value
				case "subject":
					hm.Subject = header.Value
				case "date":
					hm.Date = header.Value
				}
			}
		}

		hydrated = append(hydrated, hm)
	}

	return mcp.NewToolResultJSON(ListMessagesResponse{
		Messages: hydrated,
		Count:    len(hydrated),
	})
}

func (s *Server) handleGmailGetMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	messageID, err := request.RequireString("message_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	msg, err := s.gmail.GetMessage(ctx, messageID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(msg)
}

func (s *Server) handleGmailSendMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	to, err := request.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subject, err := request.RequireString("subject")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	body, err := request.RequireString("body")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	msg, err := s.gmail.SendMessage(ctx, to, subject, body)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(msg)
}

func (s *Server) handleGmailCreateDraft(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	to, err := request.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subject, err := request.RequireString("subject")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	body, err := request.RequireString("body")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	draft, err := s.gmail.CreateDraft(ctx, to, subject, body)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(draft)
}

func (s *Server) handleGmailSendDraft(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	draftID, err := request.RequireString("draft_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	msg, err := s.gmail.SendDraft(ctx, draftID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(msg)
}

func (s *Server) handleGmailModifyLabels(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	messageID, err := request.RequireString("message_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get array parameters - these come as []interface{} from MCP
	// Need to cast Arguments to map first
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}

	addLabelsRaw := args["add_labels"]
	removeLabelsRaw := args["remove_labels"]

	var addLabels, removeLabels []string

	if addLabelsRaw != nil {
		if arr, ok := addLabelsRaw.([]interface{}); ok {
			for _, v := range arr {
				if str, ok := v.(string); ok {
					addLabels = append(addLabels, str)
				}
			}
		}
	}

	if removeLabelsRaw != nil {
		if arr, ok := removeLabelsRaw.([]interface{}); ok {
			for _, v := range arr {
				if str, ok := v.(string); ok {
					removeLabels = append(removeLabels, str)
				}
			}
		}
	}

	modified, err := s.gmail.ModifyLabels(ctx, messageID, addLabels, removeLabels)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(modified)
}

func (s *Server) handleGmailTrashMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	messageID, err := request.RequireString("message_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	trashed, err := s.gmail.TrashMessage(ctx, messageID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(trashed)
}

func (s *Server) handleGmailDeleteMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	messageID, err := request.RequireString("message_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.gmail.DeleteMessage(ctx, messageID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Message %s deleted successfully", messageID)), nil
}

func (s *Server) handleCalendarListEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	maxResults := int64(request.GetInt("max_results", 100))

	var timeMin, timeMax time.Time
	if tm := request.GetString("time_min", ""); tm != "" {
		parsed, err := time.Parse(time.RFC3339, tm)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid time_min format: %v", err)), nil
		}
		timeMin = parsed
	}

	if tm := request.GetString("time_max", ""); tm != "" {
		parsed, err := time.Parse(time.RFC3339, tm)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid time_max format: %v", err)), nil
		}
		timeMax = parsed
	}

	events, err := s.calendar.ListEvents(ctx, maxResults, timeMin, timeMax)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(ListEventsResponse{
		Events: events,
		Count:  len(events),
	})
}

func (s *Server) handleCalendarGetEvent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	eventID, err := request.RequireString("event_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	event, err := s.calendar.GetEvent(ctx, eventID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(event)
}

func (s *Server) handleCalendarCreateEvent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	summary, err := request.RequireString("summary")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	description := request.GetString("description", "")

	startTimeStr, err := request.RequireString("start_time")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	endTimeStr, err := request.RequireString("end_time")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid start_time format: %v", err)), nil
	}

	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid end_time format: %v", err)), nil
	}

	event, err := s.calendar.CreateEvent(ctx, summary, description, startTime, endTime)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(event)
}

func (s *Server) handleCalendarUpdateEvent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	eventID, err := request.RequireString("event_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get existing event first
	event, err := s.calendar.GetEvent(ctx, eventID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Update fields if provided
	if summary := request.GetString("summary", ""); summary != "" {
		event.Summary = summary
	}

	if description := request.GetString("description", ""); description != "" {
		event.Description = description
	}

	if startTimeStr := request.GetString("start_time", ""); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid start_time format: %v", err)), nil
		}
		if event.Start == nil {
			event.Start = &googlecalendar.EventDateTime{}
		}
		event.Start.DateTime = startTime.Format(time.RFC3339)
	}

	if endTimeStr := request.GetString("end_time", ""); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid end_time format: %v", err)), nil
		}
		if event.End == nil {
			event.End = &googlecalendar.EventDateTime{}
		}
		event.End.DateTime = endTime.Format(time.RFC3339)
	}

	updated, err := s.calendar.UpdateEvent(ctx, eventID, event)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(updated)
}

func (s *Server) handleCalendarDeleteEvent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	eventID, err := request.RequireString("event_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.calendar.DeleteEvent(ctx, eventID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Event %s deleted successfully", eventID)), nil
}

func (s *Server) handlePeopleListContacts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pageSize := int64(request.GetInt("page_size", 100))

	contacts, err := s.people.ListContacts(ctx, pageSize)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(ListContactsResponse{
		Contacts: contacts,
		Count:    len(contacts),
	})
}

func (s *Server) handlePeopleSearchContacts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pageSize := int64(request.GetInt("page_size", 10))

	contacts, err := s.people.SearchContacts(ctx, query, pageSize)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(ListContactsResponse{
		Contacts: contacts,
		Count:    len(contacts),
	})
}

func (s *Server) handlePeopleGetContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceName, err := request.RequireString("resource_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	person, err := s.people.GetPerson(ctx, resourceName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(person)
}

func (s *Server) handlePeopleCreateContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	givenName, err := request.RequireString("given_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	familyName := request.GetString("family_name", "")
	email := request.GetString("email", "")
	phone := request.GetString("phone", "")

	// Build Person object
	person := &googlepeople.Person{
		Names: []*googlepeople.Name{
			{
				GivenName:  givenName,
				FamilyName: familyName,
			},
		},
	}

	if email != "" {
		person.EmailAddresses = []*googlepeople.EmailAddress{
			{Value: email},
		}
	}

	if phone != "" {
		person.PhoneNumbers = []*googlepeople.PhoneNumber{
			{Value: phone},
		}
	}

	created, err := s.people.CreateContact(ctx, person)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(created)
}

func (s *Server) handlePeopleUpdateContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceName, err := request.RequireString("resource_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get existing contact first
	person, err := s.people.GetPerson(ctx, resourceName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var updateFields []string
	var namesUpdated bool

	// Update fields if provided
	if givenName := request.GetString("given_name", ""); givenName != "" {
		if len(person.Names) == 0 {
			person.Names = []*googlepeople.Name{{}}
		}
		person.Names[0].GivenName = givenName
		namesUpdated = true
	}

	if familyName := request.GetString("family_name", ""); familyName != "" {
		if len(person.Names) == 0 {
			person.Names = []*googlepeople.Name{{}}
		}
		person.Names[0].FamilyName = familyName
		namesUpdated = true
	}

	if namesUpdated {
		updateFields = append(updateFields, "names")
	}

	if email := request.GetString("email", ""); email != "" {
		if len(person.EmailAddresses) == 0 {
			person.EmailAddresses = []*googlepeople.EmailAddress{{}}
		}
		person.EmailAddresses[0].Value = email
		updateFields = append(updateFields, "emailAddresses")
	}

	if phone := request.GetString("phone", ""); phone != "" {
		if len(person.PhoneNumbers) == 0 {
			person.PhoneNumbers = []*googlepeople.PhoneNumber{{}}
		}
		person.PhoneNumbers[0].Value = phone
		updateFields = append(updateFields, "phoneNumbers")
	}

	if len(updateFields) == 0 {
		return mcp.NewToolResultError("no fields to update"), nil
	}

	// Build update mask
	updateMask := strings.Join(updateFields, ",")

	updated, err := s.people.UpdateContact(ctx, resourceName, person, updateMask)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(updated)
}

func (s *Server) handlePeopleDeleteContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceName, err := request.RequireString("resource_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.people.DeleteContact(ctx, resourceName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Contact %s deleted successfully", resourceName)), nil
}

// ListTools returns all registered tools
func (s *Server) ListTools() []mcp.Tool {
	serverTools := s.mcp.ListTools()
	tools := make([]mcp.Tool, 0, len(serverTools))
	for _, st := range serverTools {
		tools = append(tools, st.Tool)
	}
	return tools
}

// Serve starts the MCP server with stdio transport
func (s *Server) Serve(ctx context.Context) error {
	return server.ServeStdio(s.mcp)
}
