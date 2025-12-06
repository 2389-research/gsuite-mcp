# GSuite MCP Server (Go)

A Model Context Protocol (MCP) server that provides programmatic access to Google Workspace APIs including Gmail, Google Calendar, and Google Contacts. Built in Go for performance and reliability.

![GSuite MCP Server](docs/gsuite_mcp.png)

## Features

- **Gmail Integration**: Full CRUD operations - list, get, send, create drafts, modify labels, delete messages
- **Calendar Access**: Complete calendar management - view, create, update, delete events
- **Contacts Management**: Full contacts API - list, search, get, create, update, delete contacts
- **MCP Prompts**: Pre-defined workflow templates for common tasks (email triage, meeting scheduling, etc.)
- **MCP Resources**: Dynamic data endpoints exposing calendar, email, and contact information
- **OAuth 2.0 Authentication**: Secure access to your Google Workspace data
- **Ish Mode Testing**: Test without real credentials using mock API server
- **Automatic Retry Logic**: Exponential backoff for rate limits and transient errors
- **Stdio Transport**: Seamless integration with MCP clients

## Available Tools

The server exposes 19 MCP tools organized by service:

### Gmail Tools (7)
1. **gmail_list_messages** - Search and list Gmail messages
2. **gmail_get_message** - Get a specific message by ID
3. **gmail_send_message** - Send email messages
4. **gmail_create_draft** - Create a draft email
5. **gmail_send_draft** - Send an existing draft
6. **gmail_modify_labels** - Add/remove labels from messages
7. **gmail_delete_message** - Permanently delete a message

### Calendar Tools (6)
8. **calendar_list_events** - List calendar events with time filtering
9. **calendar_get_event** - Get a specific event by ID
10. **calendar_create_event** - Create a new calendar event
11. **calendar_update_event** - Update an existing event
12. **calendar_delete_event** - Delete a calendar event
13. **calendar_quick_add** - Quick add event using natural language

### People/Contacts Tools (6)
14. **people_list_contacts** - List contact information
15. **people_get_contact** - Get a specific contact by resource name
16. **people_search_contacts** - Search contacts by query
17. **people_create_contact** - Create a new contact
18. **people_update_contact** - Update an existing contact
19. **people_delete_contact** - Delete a contact

See [docs/usage.md](docs/usage.md) for detailed tool documentation and examples.

## MCP Prompts

The server provides 8 workflow prompts for common tasks:

### Email Workflows
1. **email_triage** - Help triage and organize unread emails (never deletes, only archives)
2. **compose_email** - Help compose professional emails with threading awareness and draft-first approach
3. **email_reply** - Reply to existing emails with proper threading (searches original, extracts thread_id/message_id)

### Calendar Workflows
4. **schedule_meeting** - Find available time slots and schedule meetings with timezone handling
5. **calendar_summary** - Summarize calendar events for a time period
6. **follow_up_reminder** - Set up follow-up reminders for important emails or meetings

### Contact/CRM Workflows
7. **find_contact** - Search for contact information with CRM integration guidance
8. **add_contact_from_email** - Extract and add contact information from emails with full CRM workflow (duplicate checking, company association, interaction logging)

## MCP Resources

The server exposes 8 dynamic resources:

1. **gsuite://calendar/today** - Today's calendar events
2. **gsuite://calendar/this-week** - This week's calendar events
3. **gsuite://calendar/upcoming** - Next 5 upcoming events
4. **gsuite://calendar/availability** - Free/busy status for next 7 days
5. **gsuite://gmail/unread** - Unread emails summary
6. **gsuite://gmail/unread/important** - Important unread emails
7. **gsuite://gmail/drafts** - Current draft emails
8. **gsuite://contacts/recent** - Recently added/modified contacts

## Quick Start

### Prerequisites

- Go 1.21 or later
- Google Cloud account with Gmail, Calendar, and People APIs enabled
- OAuth 2.0 credentials (see [setup guide](docs/setup.md))

### Build

```bash
go build ./cmd/gsuite-mcp
```

### Run

```bash
# Show help
./gsuite-mcp help

# Start the MCP server (production mode, requires credentials.json)
./gsuite-mcp mcp

# Start in testing mode (requires ish mock server)
ISH_MODE=true ISH_BASE_URL=http://localhost:9000 ./gsuite-mcp mcp

# Show version
./gsuite-mcp version
```

### Configure MCP Client

Add to your MCP client configuration (e.g., Claude Desktop):

```json
{
  "mcpServers": {
    "gsuite": {
      "command": "/path/to/gsuite-mcp",
      "args": ["mcp"]
    }
  }
}
```

## Documentation

- **[Setup Guide](docs/setup.md)** - OAuth configuration and ish mode setup
- **[Usage Guide](docs/usage.md)** - Tool reference, examples, and workflows
- **[Ish Mode](docs/ISH_MODE.md)** - Testing without real Google credentials

## Project Structure

```
gsuite-mcp/
├── cmd/
│   └── gsuite-mcp/
│       └── main.go          # Entry point
├── pkg/
│   ├── auth/
│   │   ├── oauth.go         # OAuth 2.0 authentication
│   │   └── fake.go          # Ish mode fake credentials
│   ├── gmail/
│   │   └── service.go       # Gmail API wrapper
│   ├── calendar/
│   │   └── service.go       # Calendar API wrapper
│   ├── people/
│   │   └── service.go       # People API wrapper
│   ├── retry/
│   │   └── retry.go         # Exponential backoff logic
│   └── server/
│       ├── server.go        # MCP server implementation
│       ├── prompts.go       # MCP prompt templates
│       └── resources.go     # MCP dynamic resources
├── docs/                    # Documentation
└── go.mod                   # Go module dependencies
```

## Testing

Run all tests:

```bash
go test ./...
```

Run specific package tests:

```bash
go test ./pkg/gmail -v
go test ./pkg/calendar -v
go test ./pkg/people -v
go test ./pkg/server -v
```

All tests use ish mode by default and don't require real Google credentials.

### Test Coverage

```bash
go test ./... -cover
```

### Running with Ish Mode

Tests automatically configure ish mode. To test the server manually with ish:

```bash
# Set environment variables
export ISH_MODE=true
export ISH_BASE_URL=http://localhost:9000
export ISH_USER=testuser

# Start ish mock server (in another terminal)
ish server start --port 9000

# Run the server
./gsuite-mcp
```

## Technology Stack

- **Language**: Go 1.21+
- **Google APIs**:
  - `google.golang.org/api/gmail/v1`
  - `google.golang.org/api/calendar/v3`
  - `google.golang.org/api/people/v1`
- **OAuth**: `golang.org/x/oauth2`
- **MCP SDK**: `github.com/mark3labs/mcp-go`
- **Testing**: `github.com/stretchr/testify`

## Authentication Modes

### OAuth 2.0 (Production)

For production use with real Google accounts:

1. Download `credentials.json` from Google Cloud Console
2. Place in project root
3. Run server and complete OAuth flow
4. Token saved to `token.json` for future use

See [setup guide](docs/setup.md) for detailed instructions.

### Ish Mode (Testing)

For development and testing without real credentials:

1. Set `ISH_MODE=true` environment variable
2. Start mock API server on `ISH_BASE_URL`
3. Run server normally

All API requests use fake Bearer token authentication and connect to mock server.

## Security

- **Credentials**: Never commit `credentials.json` or `token.json` to version control
- **Scopes**: Server requests minimal required OAuth scopes
- **Token Storage**: OAuth tokens stored locally in `token.json`
- **Ish Mode**: For testing only, provides no real security

## Contributing

This is a reference implementation of the GSuite MCP server in Go. Contributions welcome!

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Support

- Documentation: See [docs/](docs/) directory
- Issues: File on GitHub
- Google API Help: [Google Workspace API Documentation](https://developers.google.com/workspace)

## Roadmap

Future enhancements planned:

- Batch operations support for bulk actions
- Service account authentication for organizational access
- Advanced query filters and search capabilities
- Gmail threads and conversation management
- Calendar recurring events support
- Contact groups and organization management
- Drive integration (files and folders)
- Meet integration (video calls)

See [docs/usage.md](docs/usage.md) for more details on planned features.
