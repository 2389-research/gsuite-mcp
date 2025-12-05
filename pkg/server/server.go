// ABOUTME: MCP server implementation
// ABOUTME: Exposes Gmail, Calendar, and People services as MCP tools

package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/calendar"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/people"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
		authenticator, err := auth.NewAuthenticator("credentials.json", "token.json")
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
				"query":       map[string]string{"type": "string"},
				"max_results": map[string]string{"type": "integer"},
			},
		},
	}, s.handleGmailListMessages)

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
}

// Tool handlers
func (s *Server) handleGmailListMessages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.GetString("query", "")
	maxResults := int64(request.GetInt("max_results", 100))

	messages, err := s.gmail.ListMessages(ctx, query, maxResults)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(messages)
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

	return mcp.NewToolResultJSON(events)
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
		event.Start.DateTime = startTime.Format(time.RFC3339)
	}

	if endTimeStr := request.GetString("end_time", ""); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid end_time format: %v", err)), nil
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

	return mcp.NewToolResultJSON(contacts)
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

	return mcp.NewToolResultJSON(contacts)
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
