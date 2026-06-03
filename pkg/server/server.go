// ABOUTME: MCP server implementation
// ABOUTME: Exposes Gmail, Calendar, People, and Tasks services as MCP tools

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/harper/gsuite-mcp/pkg/accounts"
	"github.com/harper/gsuite-mcp/pkg/auth"
	"github.com/harper/gsuite-mcp/pkg/calendar"
	"github.com/harper/gsuite-mcp/pkg/gmail"
	"github.com/harper/gsuite-mcp/pkg/people"
	"github.com/harper/gsuite-mcp/pkg/tasks"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	googlecalendar "google.golang.org/api/calendar/v3"
	googlepeople "google.golang.org/api/people/v1"
	googletasks "google.golang.org/api/tasks/v1"
)

// AccountServices holds all Google service instances for a single account
type AccountServices struct {
	Gmail    *gmail.Service
	Calendar *calendar.Service
	People   *people.Service
	Tasks    *tasks.Service
}

// Server is the MCP server for GSuite APIs
type Server struct {
	registry   *accounts.Registry
	services   map[string]*AccountServices
	servicesMu sync.RWMutex // protects services map
	mcp        *server.MCPServer
	auth       *auth.Authenticator // For auth management tools
}

// NewServer creates a new MCP server
func NewServer(ctx context.Context) (*Server, error) {
	var client *http.Client
	var authenticator *auth.Authenticator
	var registry *accounts.Registry

	// Check for ish mode
	if os.Getenv("ISH_MODE") == "true" {
		client = auth.NewFakeClient("")
		// Build an in-memory registry with a single "default" account
		defaultAcct := &accounts.Account{Alias: "default"}
		defaultAcct.SetClient(client)
		registry = &accounts.Registry{
			DefaultAlias: "default",
			Accounts: map[string]*accounts.Account{
				"default": defaultAcct,
			},
		}
	} else {
		// Use real OAuth
		var err error
		authenticator, err = auth.NewAuthenticator(auth.GetCredentialsPath(), auth.GetTokenPath())
		if err != nil {
			return nil, err
		}
		// Use non-interactive auth - if no token exists, client will be nil
		// and API calls will fail gracefully. User can authenticate via auth_init/auth_complete tools.
		client, err = authenticator.GetClientIfAuthenticated(ctx)
		if err != nil {
			return nil, err
		}
		// If no token yet, use a placeholder client that will fail on API calls
		if client == nil {
			client = &http.Client{}
		}

		// Load account registry from config file
		registry, err = accounts.LoadRegistry(auth.GetAccountsConfigPath())
		if err != nil {
			return nil, fmt.Errorf("failed to load accounts registry: %w", err)
		}

		// Attach the authenticated client to the default account
		defaultAcct, err := registry.GetDefault()
		if err != nil {
			return nil, fmt.Errorf("failed to get default account: %w", err)
		}
		defaultAcct.SetClient(client)
	}

	s := &Server{
		registry: registry,
		services: make(map[string]*AccountServices),
		auth:     authenticator,
	}

	// Eagerly create services for the default account
	eagerAcct, err := registry.GetDefault()
	if err != nil {
		return nil, fmt.Errorf("failed to get default account: %w", err)
	}
	defaultClient := eagerAcct.GetClient()
	if defaultClient == nil {
		defaultClient = &http.Client{}
	}
	if _, err := s.createServicesForClient(ctx, eagerAcct.Alias, defaultClient); err != nil {
		return nil, fmt.Errorf("failed to create services for default account: %w", err)
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"gsuite-mcp",
		"1.0.0",
	)

	s.mcp = mcpServer
	// Register tools, prompts and resources (schemas are kept intact for client compatibility).
	s.registerTools()
	s.registerPrompts()
	s.registerResources()

	return s, nil
}

// resolveServices returns the AccountServices for the given alias, creating them
// lazily if they don't already exist. An empty alias resolves to the default account.
func (s *Server) resolveServices(ctx context.Context, alias string) (*AccountServices, error) {
	acct, err := s.registry.Resolve(alias)
	if err != nil {
		return nil, err
	}

	// Fast path: check if services already exist
	s.servicesMu.RLock()
	if svc, ok := s.services[acct.Alias]; ok {
		s.servicesMu.RUnlock()
		return svc, nil
	}
	s.servicesMu.RUnlock()

	// Slow path: create services (need write lock)
	s.servicesMu.Lock()
	defer s.servicesMu.Unlock()

	// Double-check after acquiring write lock
	if svc, ok := s.services[acct.Alias]; ok {
		return svc, nil
	}

	client := acct.GetClient()
	if client == nil {
		// Lazy-load: create an authenticated client from the account's stored token
		tokenPath := acct.TokenPath
		if tokenPath == "" {
			tokenPath = auth.GetTokenPath()
		}
		authenticator, authErr := auth.NewAuthenticator(auth.GetCredentialsPath(), tokenPath)
		if authErr != nil {
			return nil, fmt.Errorf("account '%s' is not authenticated: %w (use auth_init tool to authenticate)", acct.Alias, authErr)
		}
		authedClient, clientErr := authenticator.GetClientIfAuthenticated(ctx)
		if clientErr != nil {
			return nil, fmt.Errorf("account '%s' authentication failed: %w (token may be expired, use auth_init to re-authenticate)", acct.Alias, clientErr)
		}
		if authedClient == nil {
			return nil, fmt.Errorf("account '%s' is not authenticated (use auth_init tool to authenticate)", acct.Alias)
		}
		acct.SetClient(authedClient)
		client = authedClient
	}

	return s.createServicesForClient(ctx, acct.Alias, client)
}

// createServicesForClient builds AccountServices for the given alias/client and
// caches them in the services map.
func (s *Server) createServicesForClient(ctx context.Context, alias string, client *http.Client) (*AccountServices, error) {
	gmailSvc, err := gmail.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service for account '%s': %w", alias, err)
	}
	calendarSvc, err := calendar.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Calendar service for account '%s': %w", alias, err)
	}
	peopleSvc, err := people.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create People service for account '%s': %w", alias, err)
	}
	tasksSvc, err := tasks.NewService(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Tasks service for account '%s': %w", alias, err)
	}

	svc := &AccountServices{
		Gmail:    gmailSvc,
		Calendar: calendarSvc,
		People:   peopleSvc,
		Tasks:    tasksSvc,
	}
	s.services[alias] = svc
	return svc, nil
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
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":    map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"message_id"},
		},
	}, s.handleGmailGetMessage)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_send_message",
		Description: "Send an email. Use in_reply_to to reply to an existing message (auto-fetches threading headers).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"to":          map[string]string{"type": "string", "description": "Recipient email address"},
				"cc":          map[string]string{"type": "string", "description": "CC email address(es) (comma-separated)"},
				"bcc":         map[string]string{"type": "string", "description": "BCC email address(es) (comma-separated)"},
				"subject":     map[string]string{"type": "string", "description": "Email subject (auto-prefixed with Re: for replies)"},
				"body":        map[string]string{"type": "string", "description": "Email body content"},
				"in_reply_to": map[string]string{"type": "string", "description": "Message ID to reply to (auto-fetches threading headers)"},
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"to", "subject", "body"},
		},
	}, s.handleGmailSendMessage)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_create_draft",
		Description: "Create a draft email. Use in_reply_to to create a reply draft (auto-fetches threading headers).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"to":          map[string]string{"type": "string", "description": "Recipient email address"},
				"cc":          map[string]string{"type": "string", "description": "CC email address(es) (comma-separated)"},
				"bcc":         map[string]string{"type": "string", "description": "BCC email address(es) (comma-separated)"},
				"subject":     map[string]string{"type": "string", "description": "Email subject (auto-prefixed with Re: for replies)"},
				"body":        map[string]string{"type": "string", "description": "Email body content"},
				"in_reply_to": map[string]string{"type": "string", "description": "Message ID to reply to (auto-fetches threading headers)"},
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":  map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":    map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":    map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"message_id"},
		},
	}, s.handleGmailDeleteMessage)

	s.mcp.AddTool(mcp.Tool{
		Name:        "gmail_manage_labels",
		Description: "Manage Gmail labels (list, get, create, update, delete). Use gmail_modify_labels to apply labels to messages.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"action": map[string]string{
					"type":        "string",
					"description": "Action to perform: list, get, create, update, delete",
				},
				"label_id": map[string]string{
					"type":        "string",
					"description": "Label ID (required for get, update, delete)",
				},
				"name": map[string]string{
					"type":        "string",
					"description": "Label name (required for create, optional for update). Use slashes for nesting: 'Projects/Client-A'",
				},
				"label_list_visibility": map[string]string{
					"type":        "string",
					"description": "Visibility in label list: labelShow, labelShowIfUnread, labelHide",
				},
				"message_list_visibility": map[string]string{
					"type":        "string",
					"description": "Visibility in message list: show, hide",
				},
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"action"},
		},
	}, s.handleGmailManageLabels)

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
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":  map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"attendees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Email addresses of required attendees",
				},
				"optional_attendees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Email addresses of optional attendees",
				},
				"send_notifications": map[string]interface{}{
					"type":        "boolean",
					"description": "Send invite emails to attendees (default: true)",
				},
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"attendees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Full replacement - replaces ALL required attendees",
				},
				"optional_attendees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Full replacement - replaces ALL optional attendees",
				},
				"add_attendees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Incremental - add as required attendees",
				},
				"add_optional_attendees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Incremental - add as optional attendees",
				},
				"remove_attendees": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "Incremental - remove by email",
				},
				"send_notifications": map[string]interface{}{
					"type":        "boolean",
					"description": "Send update emails (default: true)",
				},
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":  map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":   map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":   map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":       map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":       map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
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
				"account":       map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"resource_name"},
		},
	}, s.handlePeopleDeleteContact)

	// Tasks tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_list_tasklists",
		Description: "List all task lists",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
		},
	}, s.handleTasksListTaskLists)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_create_tasklist",
		Description: "Create a new task list",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"title":   map[string]string{"type": "string", "description": "Title of the task list"},
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"title"},
		},
	}, s.handleTasksCreateTaskList)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_update_tasklist",
		Description: "Update a task list's title",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list"},
				"title":       map[string]string{"type": "string", "description": "New title for the task list"},
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"tasklist_id", "title"},
		},
	}, s.handleTasksUpdateTaskList)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_delete_tasklist",
		Description: "Delete a task list",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list to delete"},
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"tasklist_id"},
		},
	}, s.handleTasksDeleteTaskList)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_list_tasks",
		Description: "List tasks in a task list",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id":    map[string]string{"type": "string", "description": "ID of the task list (default: @default)"},
				"show_completed": map[string]interface{}{"type": "boolean", "description": "Include completed tasks (default: false)"},
				"show_hidden":    map[string]interface{}{"type": "boolean", "description": "Include hidden tasks (default: false)"},
				"due_min":        map[string]string{"type": "string", "description": "Minimum due date (RFC 3339, e.g., 2026-01-01T00:00:00Z)"},
				"due_max":        map[string]string{"type": "string", "description": "Maximum due date (RFC 3339, e.g., 2026-12-31T23:59:59Z)"},
				"account":        map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
		},
	}, s.handleTasksListTasks)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_create_task",
		Description: "Create a task in a task list",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list (default: @default)"},
				"title":       map[string]string{"type": "string", "description": "Title of the task"},
				"notes":       map[string]string{"type": "string", "description": "Additional notes for the task"},
				"due":         map[string]string{"type": "string", "description": "Due date (RFC 3339, e.g., 2026-03-01T00:00:00Z)"},
				"parent":      map[string]string{"type": "string", "description": "Parent task ID for creating a subtask"},
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"title"},
		},
	}, s.handleTasksCreateTask)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_update_task",
		Description: "Update a task's title, notes, due date, or status",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list (default: @default)"},
				"task_id":     map[string]string{"type": "string", "description": "ID of the task to update"},
				"title":       map[string]string{"type": "string", "description": "New title"},
				"notes":       map[string]string{"type": "string", "description": "New notes"},
				"due":         map[string]string{"type": "string", "description": "New due date (RFC 3339)"},
				"status":      map[string]string{"type": "string", "description": "Task status: 'needsAction' or 'completed'"},
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"task_id"},
		},
	}, s.handleTasksUpdateTask)

	s.mcp.AddTool(mcp.Tool{
		Name:        "tasks_delete_task",
		Description: "Delete a task",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tasklist_id": map[string]string{"type": "string", "description": "ID of the task list (default: @default)"},
				"task_id":     map[string]string{"type": "string", "description": "ID of the task to delete"},
				"account":     map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"task_id"},
		},
	}, s.handleTasksDeleteTask)

	// Auth tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "auth_status",
		Description: "Check if OAuth authentication is valid by making a test API call",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
		},
	}, s.handleAuthStatus)

	s.mcp.AddTool(mcp.Tool{
		Name:        "auth_info",
		Description: "Get OAuth token metadata (expiry, scopes) without making API calls",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
		},
	}, s.handleAuthInfo)

	s.mcp.AddTool(mcp.Tool{
		Name:        "auth_init",
		Description: "Start OAuth authentication flow. Returns an auth_url the USER must visit in their browser to authorize. After authorizing, the user receives a code to provide to auth_complete. Returns current status if already authenticated (use force=true to re-authenticate).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "Force new auth flow even if current auth is valid",
				},
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
		},
	}, s.handleAuthInit)

	s.mcp.AddTool(mcp.Tool{
		Name:        "auth_complete",
		Description: "Complete OAuth flow by exchanging authorization code for tokens. Call this after the user visits the auth_url from auth_init. The user should provide the FULL redirect URL from their browser (e.g., http://localhost/?code=4/0AfJohX...) - the code will be extracted automatically.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"code":    map[string]string{"type": "string", "description": "The full redirect URL from the browser, or just the authorization code"},
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
			Required: []string{"code"},
		},
	}, s.handleAuthComplete)

	s.mcp.AddTool(mcp.Tool{
		Name:        "auth_revoke",
		Description: "Delete cached OAuth token, forcing re-authentication on next API call",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"account": map[string]string{"type": "string", "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."},
			},
		},
	}, s.handleAuthRevoke)

	// Account management tools
	s.mcp.AddTool(mcp.Tool{
		Name:        "accounts_list",
		Description: "List all configured accounts and their authentication status",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleAccountsList)
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

// ListTaskListsResponse wraps task lists for MCP
type ListTaskListsResponse struct {
	TaskLists []*googletasks.TaskList `json:"task_lists"`
	Count     int                     `json:"count"`
}

// ListTasksResponse wraps tasks for MCP
type ListTasksResponse struct {
	Tasks []*googletasks.Task `json:"tasks"`
	Count int                 `json:"count"`
}

// Tool handlers
func (s *Server) handleGmailListMessages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	query := request.GetString("query", "")
	maxResults := int64(request.GetInt("max_results", 100))
	hydrate := request.GetBool("hydrate", false)

	messages, err := svc.Gmail.ListMessages(ctx, query, maxResults)
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
		fullMsg, err := svc.Gmail.GetMessage(ctx, msg.Id)
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
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	messageID, err := request.RequireString("message_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	msg, err := svc.Gmail.GetMessage(ctx, messageID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(msg)
}

func (s *Server) handleGmailSendMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	inReplyTo := request.GetString("in_reply_to", "")
	cc := request.GetString("cc", "")
	bcc := request.GetString("bcc", "")

	msg, err := svc.Gmail.SendMessage(ctx, to, subject, body, inReplyTo, cc, bcc)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(msg)
}

func (s *Server) handleGmailCreateDraft(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	inReplyTo := request.GetString("in_reply_to", "")
	cc := request.GetString("cc", "")
	bcc := request.GetString("bcc", "")

	draft, err := svc.Gmail.CreateDraft(ctx, to, subject, body, inReplyTo, cc, bcc)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(draft)
}

func (s *Server) handleGmailSendDraft(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	draftID, err := request.RequireString("draft_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	msg, err := svc.Gmail.SendDraft(ctx, draftID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(msg)
}

func (s *Server) handleGmailModifyLabels(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	modified, err := svc.Gmail.ModifyLabels(ctx, messageID, addLabels, removeLabels)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(modified)
}

func (s *Server) handleGmailTrashMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	messageID, err := request.RequireString("message_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	trashed, err := svc.Gmail.TrashMessage(ctx, messageID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(trashed)
}

func (s *Server) handleGmailDeleteMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	messageID, err := request.RequireString("message_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = svc.Gmail.DeleteMessage(ctx, messageID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Message %s deleted successfully", messageID)), nil
}

// LabelSummary is a compact representation of a label for list results
type LabelSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ManageLabelsResponse wraps label management results
type ManageLabelsResponse struct {
	Action  string         `json:"action"`
	Labels  []LabelSummary `json:"labels,omitempty"`
	Label   *LabelSummary  `json:"label,omitempty"`
	Count   int            `json:"count,omitempty"`
	Message string         `json:"message,omitempty"`
}

func (s *Server) handleGmailManageLabels(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	action, err := request.RequireString("action")
	if err != nil {
		return mcp.NewToolResultError("action is required. Valid actions: list, get, create, update, delete"), nil
	}

	switch action {
	case "list":
		labels, err := svc.Gmail.ListLabels(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		summaries := make([]LabelSummary, len(labels))
		for i, label := range labels {
			summaries[i] = LabelSummary{
				ID:   label.Id,
				Name: label.Name,
				Type: label.Type,
			}
		}

		return mcp.NewToolResultJSON(ManageLabelsResponse{
			Action: "list",
			Labels: summaries,
			Count:  len(summaries),
		})

	case "get":
		labelID := request.GetString("label_id", "")
		if labelID == "" {
			return mcp.NewToolResultError("label_id is required for get action. Use action: list to see available labels."), nil
		}

		label, err := svc.Gmail.GetLabel(ctx, labelID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("label not found: %v. Use action: list to see available labels.", err)), nil
		}

		return mcp.NewToolResultJSON(ManageLabelsResponse{
			Action: "get",
			Label: &LabelSummary{
				ID:   label.Id,
				Name: label.Name,
				Type: label.Type,
			},
		})

	case "create":
		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("name is required for create action. Use slashes for nested labels: 'Projects/Client-A'"), nil
		}

		labelListVisibility := request.GetString("label_list_visibility", "")
		messageListVisibility := request.GetString("message_list_visibility", "")

		label, err := svc.Gmail.CreateLabel(ctx, name, labelListVisibility, messageListVisibility)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create label: %v", err)), nil
		}

		return mcp.NewToolResultJSON(ManageLabelsResponse{
			Action:  "create",
			Label:   &LabelSummary{ID: label.Id, Name: label.Name, Type: label.Type},
			Message: fmt.Sprintf("Label '%s' created with ID: %s", label.Name, label.Id),
		})

	case "update":
		labelID := request.GetString("label_id", "")
		if labelID == "" {
			return mcp.NewToolResultError("label_id is required for update action. Use action: list to see available labels."), nil
		}

		name := request.GetString("name", "")
		labelListVisibility := request.GetString("label_list_visibility", "")
		messageListVisibility := request.GetString("message_list_visibility", "")

		if name == "" && labelListVisibility == "" && messageListVisibility == "" {
			return mcp.NewToolResultError("at least one of name, label_list_visibility, or message_list_visibility must be provided for update"), nil
		}

		label, err := svc.Gmail.UpdateLabel(ctx, labelID, name, labelListVisibility, messageListVisibility)
		if err != nil {
			if strings.Contains(err.Error(), "systemLabelCannotBeUpdated") {
				return mcp.NewToolResultError("system labels (INBOX, SENT, etc.) cannot be updated. Only user-created labels can be modified."), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("failed to update label: %v", err)), nil
		}

		return mcp.NewToolResultJSON(ManageLabelsResponse{
			Action:  "update",
			Label:   &LabelSummary{ID: label.Id, Name: label.Name, Type: label.Type},
			Message: fmt.Sprintf("Label '%s' updated successfully", label.Name),
		})

	case "delete":
		labelID := request.GetString("label_id", "")
		if labelID == "" {
			return mcp.NewToolResultError("label_id is required for delete action. Use action: list to see available labels."), nil
		}

		err := svc.Gmail.DeleteLabel(ctx, labelID)
		if err != nil {
			if strings.Contains(err.Error(), "systemLabelCannotBeDeleted") {
				return mcp.NewToolResultError("system labels (INBOX, SENT, etc.) cannot be deleted. Only user-created labels can be deleted."), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("failed to delete label: %v", err)), nil
		}

		return mcp.NewToolResultJSON(ManageLabelsResponse{
			Action:  "delete",
			Message: fmt.Sprintf("Label %s deleted successfully", labelID),
		})

	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown action '%s'. Valid actions: list, get, create, update, delete", action)), nil
	}
}

func (s *Server) handleCalendarListEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	events, err := svc.Calendar.ListEvents(ctx, maxResults, timeMin, timeMax)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(ListEventsResponse{
		Events: events,
		Count:  len(events),
	})
}

func (s *Server) handleCalendarGetEvent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	eventID, err := request.RequireString("event_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	event, err := svc.Calendar.GetEvent(ctx, eventID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(event)
}

func (s *Server) handleCalendarCreateEvent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	// Get optional attendee parameters
	attendees := request.GetStringSlice("attendees", []string{})
	optionalAttendees := request.GetStringSlice("optional_attendees", []string{})
	sendNotifications := request.GetBool("send_notifications", true)

	event, err := svc.Calendar.CreateEvent(ctx, summary, description, startTime, endTime, attendees, optionalAttendees, sendNotifications)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(event)
}

func (s *Server) handleCalendarUpdateEvent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	eventID, err := request.RequireString("event_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Validate attendee parameters before fetching event
	attendees := request.GetStringSlice("attendees", nil)
	optionalAttendees := request.GetStringSlice("optional_attendees", nil)
	addAttendees := request.GetStringSlice("add_attendees", nil)
	addOptionalAttendees := request.GetStringSlice("add_optional_attendees", nil)
	removeAttendees := request.GetStringSlice("remove_attendees", nil)

	// Detect which mode is being used
	hasFullReplacement := attendees != nil || optionalAttendees != nil
	hasIncremental := addAttendees != nil || addOptionalAttendees != nil || removeAttendees != nil

	// Error if mixing modes
	if hasFullReplacement && hasIncremental {
		return mcp.NewToolResultError("cannot mix full replacement (attendees/optional_attendees) with incremental updates (add_attendees/add_optional_attendees/remove_attendees)"), nil
	}

	// Get existing event
	event, err := svc.Calendar.GetEvent(ctx, eventID)
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

	// Handle attendee updates

	// Apply attendee updates
	if hasFullReplacement {
		// Full replacement mode - rebuild attendee list with deduplication
		// Use map to deduplicate by email (case-insensitive)
		// If same email in both lists, optional_attendees wins (processed second)
		seen := make(map[string]*googlecalendar.EventAttendee)

		// Add required attendees
		for _, email := range attendees {
			if email == "" {
				continue
			}
			emailLower := strings.ToLower(email)
			seen[emailLower] = &googlecalendar.EventAttendee{
				Email:    email,
				Optional: false,
			}
		}

		// Add optional attendees (overwrites if duplicate)
		for _, email := range optionalAttendees {
			if email == "" {
				continue
			}
			emailLower := strings.ToLower(email)
			seen[emailLower] = &googlecalendar.EventAttendee{
				Email:    email,
				Optional: true,
			}
		}

		// Convert map to slice with deterministic order
		newAttendees := make([]*googlecalendar.EventAttendee, 0, len(seen))
		for _, att := range seen {
			newAttendees = append(newAttendees, att)
		}
		sort.Slice(newAttendees, func(i, j int) bool {
			return newAttendees[i].Email < newAttendees[j].Email
		})

		event.Attendees = newAttendees
	} else if hasIncremental {
		// Incremental mode - modify existing attendee list
		existingAttendees := event.Attendees
		if existingAttendees == nil {
			existingAttendees = []*googlecalendar.EventAttendee{}
		}

		// Build a map for quick lookup
		attendeeMap := make(map[string]*googlecalendar.EventAttendee)
		for _, att := range existingAttendees {
			attendeeMap[strings.ToLower(att.Email)] = att
		}

		// Add required attendees
		for _, email := range addAttendees {
			emailLower := strings.ToLower(email)
			if _, exists := attendeeMap[emailLower]; !exists {
				attendeeMap[emailLower] = &googlecalendar.EventAttendee{
					Email:    email,
					Optional: false,
				}
			}
		}

		// Add optional attendees
		for _, email := range addOptionalAttendees {
			emailLower := strings.ToLower(email)
			if _, exists := attendeeMap[emailLower]; !exists {
				attendeeMap[emailLower] = &googlecalendar.EventAttendee{
					Email:    email,
					Optional: true,
				}
			}
		}

		// Remove attendees
		for _, email := range removeAttendees {
			emailLower := strings.ToLower(email)
			delete(attendeeMap, emailLower)
		}

		// Convert map back to slice with deterministic order
		finalAttendees := make([]*googlecalendar.EventAttendee, 0, len(attendeeMap))
		for _, att := range attendeeMap {
			finalAttendees = append(finalAttendees, att)
		}
		sort.Slice(finalAttendees, func(i, j int) bool {
			return finalAttendees[i].Email < finalAttendees[j].Email
		})

		event.Attendees = finalAttendees
	}

	// Get send_notifications parameter (defaults to true)
	sendNotifications := request.GetBool("send_notifications", true)

	updated, err := svc.Calendar.UpdateEvent(ctx, eventID, event, sendNotifications)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(updated)
}

func (s *Server) handleCalendarDeleteEvent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	eventID, err := request.RequireString("event_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = svc.Calendar.DeleteEvent(ctx, eventID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Event %s deleted successfully", eventID)), nil
}

func (s *Server) handlePeopleListContacts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pageSize := int64(request.GetInt("page_size", 100))

	contacts, err := svc.People.ListContacts(ctx, pageSize)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(ListContactsResponse{
		Contacts: contacts,
		Count:    len(contacts),
	})
}

func (s *Server) handlePeopleSearchContacts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pageSize := int64(request.GetInt("page_size", 10))

	contacts, err := svc.People.SearchContacts(ctx, query, pageSize)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(ListContactsResponse{
		Contacts: contacts,
		Count:    len(contacts),
	})
}

func (s *Server) handlePeopleGetContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resourceName, err := request.RequireString("resource_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	person, err := svc.People.GetPerson(ctx, resourceName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(person)
}

func (s *Server) handlePeopleCreateContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	created, err := svc.People.CreateContact(ctx, person)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(created)
}

func (s *Server) handlePeopleUpdateContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resourceName, err := request.RequireString("resource_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get existing contact first
	person, err := svc.People.GetPerson(ctx, resourceName)
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

	updated, err := svc.People.UpdateContact(ctx, resourceName, person, updateMask)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(updated)
}

func (s *Server) handlePeopleDeleteContact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resourceName, err := request.RequireString("resource_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = svc.People.DeleteContact(ctx, resourceName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Contact %s deleted successfully", resourceName)), nil
}

// Tasks tool handlers

func (s *Server) handleTasksListTaskLists(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	taskLists, err := svc.Tasks.ListTaskLists(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(ListTaskListsResponse{
		TaskLists: taskLists,
		Count:     len(taskLists),
	})
}

func (s *Server) handleTasksCreateTaskList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	title, err := request.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	taskList, err := svc.Tasks.CreateTaskList(ctx, title)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(taskList)
}

func (s *Server) handleTasksUpdateTaskList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tasklistID, err := request.RequireString("tasklist_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	title, err := request.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	taskList, err := svc.Tasks.UpdateTaskList(ctx, tasklistID, title)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(taskList)
}

func (s *Server) handleTasksDeleteTaskList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tasklistID, err := request.RequireString("tasklist_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = svc.Tasks.DeleteTaskList(ctx, tasklistID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(map[string]string{
		"status":      "deleted",
		"tasklist_id": tasklistID,
	})
}

func (s *Server) handleTasksListTasks(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tasklistID := request.GetString("tasklist_id", "@default")
	showCompleted := request.GetBool("show_completed", false)
	showHidden := request.GetBool("show_hidden", false)
	dueMin := request.GetString("due_min", "")
	dueMax := request.GetString("due_max", "")

	tasks, err := svc.Tasks.ListTasks(ctx, tasklistID, showCompleted, showHidden, dueMin, dueMax)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(ListTasksResponse{
		Tasks: tasks,
		Count: len(tasks),
	})
}

func (s *Server) handleTasksCreateTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	title, err := request.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tasklistID := request.GetString("tasklist_id", "@default")
	notes := request.GetString("notes", "")
	due := request.GetString("due", "")
	parent := request.GetString("parent", "")

	task := &googletasks.Task{
		Title: title,
	}
	if notes != "" {
		task.Notes = notes
	}
	if due != "" {
		task.Due = due
	}

	created, err := svc.Tasks.CreateTask(ctx, tasklistID, task, parent)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(created)
}

func (s *Server) handleTasksUpdateTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	taskID, err := request.RequireString("task_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tasklistID := request.GetString("tasklist_id", "@default")
	title := request.GetString("title", "")
	notes := request.GetString("notes", "")
	due := request.GetString("due", "")
	status := request.GetString("status", "")

	task := &googletasks.Task{}
	if title != "" {
		task.Title = title
	}
	if notes != "" {
		task.Notes = notes
	}
	if due != "" {
		task.Due = due
	}
	if status != "" {
		task.Status = status
	}

	updated, err := svc.Tasks.UpdateTask(ctx, tasklistID, taskID, task)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(updated)
}

func (s *Server) handleTasksDeleteTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	taskID, err := request.RequireString("task_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tasklistID := request.GetString("tasklist_id", "@default")

	err = svc.Tasks.DeleteTask(ctx, tasklistID, taskID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultJSON(map[string]string{
		"status":  "deleted",
		"task_id": taskID,
	})
}

// Auth tool handlers

// authenticatorForAccount creates an Authenticator for a specific account alias.
// It resolves the account via the registry to find its token path, then creates
// a fresh Authenticator using the shared credentials and that token path.
// If the account doesn't exist in the registry, it returns an error.
func (s *Server) authenticatorForAccount(alias string) (*auth.Authenticator, error) {
	acct, err := s.registry.Resolve(alias)
	if err != nil {
		return nil, err
	}

	tokenPath := acct.TokenPath
	if tokenPath == "" {
		// Fallback for the default account that may not have a token path set
		tokenPath = auth.GetTokenPath()
	}

	return auth.NewAuthenticator(auth.GetCredentialsPath(), tokenPath)
}

// extractAuthCode extracts the authorization code from a URL or returns the input as-is.
// Handles Google's redirect URL format: http://localhost/?code=4/0AfJohX...&scope=...
func extractAuthCode(codeOrURL string) string {
	// If it looks like a URL, try to parse it
	if strings.HasPrefix(codeOrURL, "http://") || strings.HasPrefix(codeOrURL, "https://") {
		if u, err := url.Parse(codeOrURL); err == nil {
			if code := u.Query().Get("code"); code != "" {
				return code
			}
		}
	}
	// Return as-is (already a code, or unparseable)
	return codeOrURL
}

// AuthStatusResponse is the response for auth_status tool
type AuthStatusResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

func (s *Server) handleAuthStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")

	// In ISH mode, always return valid
	if os.Getenv("ISH_MODE") == "true" {
		return mcp.NewToolResultJSON(AuthStatusResponse{
			Valid:   true,
			Message: "ISH mode - auth is simulated",
		})
	}

	// Try a lightweight API call to verify auth works
	svc, err := s.resolveServices(ctx, account)
	if err != nil {
		return mcp.NewToolResultJSON(AuthStatusResponse{
			Valid:   false,
			Message: fmt.Sprintf("failed to resolve services: %v", err),
		})
	}
	_, err = svc.Gmail.ListMessages(ctx, "", 1)
	if err != nil {
		return mcp.NewToolResultJSON(AuthStatusResponse{
			Valid:   false,
			Message: fmt.Sprintf("auth check failed: %v", err),
		})
	}

	return mcp.NewToolResultJSON(AuthStatusResponse{
		Valid:   true,
		Message: "authentication is valid",
	})
}

// AuthInfoResponse is the response for auth_info tool
type AuthInfoResponse struct {
	Valid       bool   `json:"valid"`
	AccessToken string `json:"access_token,omitempty"`
	Expiry      string `json:"expiry,omitempty"`
	ExpiresIn   string `json:"expires_in,omitempty"`
	HasRefresh  bool   `json:"has_refresh"`
	Message     string `json:"message,omitempty"`
}

func (s *Server) handleAuthInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")

	// In ISH mode, return fake info
	if os.Getenv("ISH_MODE") == "true" {
		return mcp.NewToolResultJSON(AuthInfoResponse{
			Valid:      true,
			HasRefresh: true,
			Message:    "ISH mode - token info is simulated",
		})
	}

	authenticator, err := s.authenticatorForAccount(account)
	if err != nil {
		return mcp.NewToolResultJSON(AuthInfoResponse{
			Valid:   false,
			Message: fmt.Sprintf("failed to resolve account: %v", err),
		})
	}

	info, err := authenticator.TokenInfo()
	if err != nil {
		return mcp.NewToolResultJSON(AuthInfoResponse{
			Valid:   false,
			Message: fmt.Sprintf("failed to get token info: %v", err),
		})
	}

	resp := AuthInfoResponse{
		Valid:       info.Valid,
		AccessToken: info.AccessToken,
		HasRefresh:  info.HasRefresh,
	}

	if !info.Expiry.IsZero() {
		resp.Expiry = info.Expiry.Format(time.RFC3339)
		resp.ExpiresIn = info.ExpiresIn.Round(time.Second).String()
	}

	return mcp.NewToolResultJSON(resp)
}

// AuthInitResponse is the response for auth_init tool
type AuthInitResponse struct {
	Status  string `json:"status"`
	AuthURL string `json:"auth_url,omitempty"`
	Message string `json:"message"`
}

func (s *Server) handleAuthInit(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")

	// In ISH mode, return simulated response
	if os.Getenv("ISH_MODE") == "true" {
		return mcp.NewToolResultJSON(AuthInitResponse{
			Status:  "valid",
			Message: "ISH mode - auth is simulated, no action needed",
		})
	}

	// If the account doesn't exist yet, auto-create it in the registry
	resolvedAlias := account
	if resolvedAlias == "" {
		resolvedAlias = s.registry.DefaultAlias
	}
	if _, err := s.registry.Resolve(resolvedAlias); err != nil {
		// Validate alias before creating
		if valErr := accounts.ValidateAlias(resolvedAlias); valErr != nil {
			return mcp.NewToolResultJSON(AuthInitResponse{
				Status:  "error",
				Message: fmt.Sprintf("invalid account alias: %v", valErr),
			})
		}
		// Auto-create with a generated token path
		tokenPath := auth.GenerateAccountTokenPath(resolvedAlias)
		if addErr := s.registry.AddAccount(resolvedAlias, tokenPath); addErr != nil {
			return mcp.NewToolResultJSON(AuthInitResponse{
				Status:  "error",
				Message: fmt.Sprintf("failed to create account '%s': %v", resolvedAlias, addErr),
			})
		}
	}

	authenticator, err := s.authenticatorForAccount(account)
	if err != nil {
		return mcp.NewToolResultJSON(AuthInitResponse{
			Status:  "error",
			Message: fmt.Sprintf("failed to resolve account: %v", err),
		})
	}

	force := request.GetBool("force", false)

	// Check current auth status if not forcing
	if !force {
		info, err := authenticator.TokenInfo()
		if err == nil && info.Valid {
			return mcp.NewToolResultJSON(AuthInitResponse{
				Status:  "valid",
				Message: "current authentication is valid - use force=true to re-authenticate",
			})
		}
	}

	// Return auth URL for user to visit
	authURL := authenticator.AuthURL()
	return mcp.NewToolResultJSON(AuthInitResponse{
		Status:  "auth_required",
		AuthURL: authURL,
		Message: "visit the auth_url in a browser and authorize the app. After authorizing, copy the FULL URL from your browser (it will look like http://localhost/?code=...) and provide it to auth_complete",
	})
}

// AuthCompleteResponse is the response for auth_complete tool
type AuthCompleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *Server) handleAuthComplete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")

	// In ISH mode, return simulated response
	if os.Getenv("ISH_MODE") == "true" {
		return mcp.NewToolResultJSON(AuthCompleteResponse{
			Success: true,
			Message: "ISH mode - auth completion simulated",
		})
	}

	authenticator, err := s.authenticatorForAccount(account)
	if err != nil {
		return mcp.NewToolResultJSON(AuthCompleteResponse{
			Success: false,
			Message: fmt.Sprintf("failed to resolve account: %v", err),
		})
	}

	codeOrURL, err := request.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Extract code from URL if user provided the full redirect URL
	code := extractAuthCode(codeOrURL)

	err = authenticator.ExchangeCode(ctx, code)
	if err != nil {
		return mcp.NewToolResultJSON(AuthCompleteResponse{
			Success: false,
			Message: fmt.Sprintf("token exchange failed: %v", err),
		})
	}

	// Create fresh services for this account using the newly authenticated client
	acct, resolveErr := s.registry.Resolve(account)
	if resolveErr == nil {
		client, clientErr := authenticator.GetClientIfAuthenticated(ctx)
		if clientErr == nil && client != nil {
			acct.SetClient(client)
			// Evict cached services so they'll be recreated with the new client
			s.servicesMu.Lock()
			delete(s.services, acct.Alias)
			s.servicesMu.Unlock()
		}
	}

	return mcp.NewToolResultJSON(AuthCompleteResponse{
		Success: true,
		Message: "authentication completed successfully - token saved",
	})
}

// AuthRevokeResponse is the response for auth_revoke tool
type AuthRevokeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (s *Server) handleAuthRevoke(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account := request.GetString("account", "")

	// In ISH mode, return simulated response
	if os.Getenv("ISH_MODE") == "true" {
		return mcp.NewToolResultJSON(AuthRevokeResponse{
			Success: true,
			Message: "ISH mode - auth revocation simulated",
		})
	}

	authenticator, err := s.authenticatorForAccount(account)
	if err != nil {
		return mcp.NewToolResultJSON(AuthRevokeResponse{
			Success: false,
			Message: fmt.Sprintf("failed to resolve account: %v", err),
		})
	}

	err = authenticator.RevokeToken()
	if err != nil {
		return mcp.NewToolResultJSON(AuthRevokeResponse{
			Success: false,
			Message: fmt.Sprintf("failed to revoke token: %v", err),
		})
	}

	// Evict cached services for this account
	acct, resolveErr := s.registry.Resolve(account)
	if resolveErr == nil {
		acct.SetClient(nil)
		s.servicesMu.Lock()
		delete(s.services, acct.Alias)
		s.servicesMu.Unlock()
	}

	return mcp.NewToolResultJSON(AuthRevokeResponse{
		Success: true,
		Message: "token revoked - use auth_init to start new authentication flow",
	})
}

// AccountInfo describes a single account's metadata and auth status
type AccountInfo struct {
	Alias         string `json:"alias"`
	IsDefault     bool   `json:"is_default"`
	Authenticated bool   `json:"authenticated"`
	TokenPath     string `json:"token_path"`
}

func (s *Server) handleAccountsList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var accountInfos []AccountInfo
	for _, alias := range s.registry.ListAccounts() {
		acct, _ := s.registry.Resolve(alias)
		authenticated := false
		if acct.TokenPath != "" {
			if _, err := os.Stat(acct.TokenPath); err == nil {
				authenticated = true
			}
		}
		accountInfos = append(accountInfos, AccountInfo{
			Alias:         alias,
			IsDefault:     alias == s.registry.DefaultAlias,
			Authenticated: authenticated,
			TokenPath:     acct.TokenPath,
		})
	}

	return mcp.NewToolResultJSON(map[string]interface{}{
		"accounts": accountInfos,
		"count":    len(accountInfos),
	})
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
