# GSuite MCP Server

A comprehensive Model Context Protocol (MCP) server for Google Workspace APIs, providing seamless access to Gmail, Google Calendar, and Google People (Contacts) APIs.

## Features

### Gmail (9 tools)
- ✅ List and search messages with Gmail query syntax
- ✅ Read full message content including headers
- ✅ Send emails (plain text and HTML)
- ✅ Reply to emails (maintains threading)
- ✅ Draft management (create, update, send, delete)
- ✅ Label operations (list, create, apply, remove)
- ✅ Message operations (mark read/unread, star, archive, delete)
- ✅ Attachment handling (download, send with attachments)
- ✅ Batch operations (bulk modify, bulk delete)

### Calendar (4 tools)
- ✅ List calendars and events
- ✅ Create events with attendees
- ✅ Update and delete events
- ✅ Recurring events support
- ✅ Multiple calendar support
- ✅ Event search
- ✅ Meeting responses (accept/decline)

### People API (4 tools)
- ✅ List all contacts
- ✅ Search contacts by name/email
- ✅ Create new contacts
- ✅ Update contact information
- ✅ Delete contacts
- ✅ Contact groups (list, create, add members)

### Infrastructure
- ✅ OAuth 2.0 authentication with token caching
- ✅ Automatic token refresh
- ✅ Smart retry logic with exponential backoff
- ✅ Rate limit handling
- ✅ Comprehensive error handling
- ✅ Full test coverage
- ✅ Ish mode for testing without real Google credentials

## Quick Start

### 1. Setup Google Cloud

See [Setup Guide](docs/setup.md) for detailed instructions:

1. Create Google Cloud project
2. Enable Gmail, Calendar, and People APIs
3. Create OAuth 2.0 credentials
4. Download `credentials.json`

Or use [Ish Mode](docs/ISH_MODE.md) for testing without Google credentials.

### 2. Install

```bash
uv sync
```

### 3. Configure

**Production Mode:**
Place `credentials.json` in project root.

**Ish Mode (Testing):**
```bash
export ISH_MODE=true
export ISH_BASE_URL=http://localhost:9000
```

### 4. Run

```bash
uv run python -m gsuite_mcp
```

First run will open browser for OAuth consent (production mode only).

## Documentation

- [Setup Guide](docs/setup.md) - Google Cloud setup, OAuth configuration, installation
- [Usage Guide](docs/usage.md) - Complete tool reference with examples for all 17 tools
- [Ish Mode](docs/ISH_MODE.md) - Testing mode with fake Google API server

## Available Tools

### Gmail
1. `gmail_list_messages` - List/search messages
2. `gmail_get_message` - Get message details
3. `gmail_send_message` - Send email
4. `gmail_reply_to_message` - Reply to thread
5. `gmail_list_labels` - List labels
6. `gmail_create_label` - Create label
7. `gmail_modify_message` - Modify labels
8. `gmail_create_draft` - Create draft
9. `gmail_send_draft` - Send draft

### Calendar
10. `calendar_list_events` - List events
11. `calendar_create_event` - Create event
12. `calendar_update_event` - Update event
13. `calendar_delete_event` - Delete event

### People
14. `people_list_contacts` - List contacts
15. `people_search_contacts` - Search contacts
16. `people_create_contact` - Create contact
17. `people_delete_contact` - Delete contact

## Example Usage

### List Unread Emails
```json
{
  "tool": "gmail_list_messages",
  "arguments": {
    "query": "is:unread",
    "max_results": 10
  }
}
```

### Send an Email
```json
{
  "tool": "gmail_send_message",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "Hello from MCP",
    "body": "This email was sent via the GSuite MCP server!"
  }
}
```

### Create Calendar Event
```json
{
  "tool": "calendar_create_event",
  "arguments": {
    "summary": "Team Meeting",
    "start_time": "2025-12-05T10:00:00Z",
    "end_time": "2025-12-05T11:00:00Z",
    "attendees": ["alice@example.com", "bob@example.com"]
  }
}
```

### Search Contacts
```json
{
  "tool": "people_search_contacts",
  "arguments": {
    "query": "John Doe"
  }
}
```

## MCP Client Configuration

Add to your MCP client (e.g., Claude Desktop):

**Production Mode:**
```json
{
  "mcpServers": {
    "gsuite": {
      "command": "uv",
      "args": ["run", "python", "-m", "gsuite_mcp"],
      "cwd": "/path/to/gsuite-mcp"
    }
  }
}
```

**Ish Mode (Testing):**
```json
{
  "mcpServers": {
    "gsuite": {
      "command": "uv",
      "args": ["run", "python", "-m", "gsuite_mcp"],
      "cwd": "/path/to/gsuite-mcp",
      "env": {
        "ISH_MODE": "true",
        "ISH_BASE_URL": "http://localhost:9000",
        "ISH_USER": "testuser"
      }
    }
  }
}
```

## Development

### Run Tests

```bash
uv run pytest -v
```

### Run with Coverage

```bash
uv run pytest --cov=src/gsuite_mcp --cov-report=html
```

### Code Structure

```
src/gsuite_mcp/
├── auth/
│   ├── oauth.py               # OAuth 2.0 authentication
│   └── fake_credentials.py    # Fake credentials for ish mode
├── services/
│   ├── gmail.py               # Gmail API operations
│   ├── calendar.py            # Calendar API operations
│   └── people.py              # People API operations
├── utils/
│   └── retry.py               # Retry logic with backoff
├── server.py                  # MCP server implementation
└── __main__.py                # Entry point

tests/                          # Comprehensive test suite
docs/                           # Documentation
```

## Testing Modes

### Production Mode
Uses real Google OAuth credentials and APIs. Suitable for production use with real Gmail, Calendar, and Contacts data.

### Ish Mode
Uses a fake Google API server for testing. No OAuth required, no real data affected. Perfect for development and automated testing.

See [ISH_MODE.md](docs/ISH_MODE.md) for details.

## Features in Detail

### Authentication
- OAuth 2.0 flow with browser-based consent
- Automatic token refresh
- Secure credential caching
- Support for multiple scopes (Gmail, Calendar, People)
- Fake credentials for ish mode testing

### Resilience
- Exponential backoff retry logic
- Automatic rate limit handling
- Comprehensive error messages
- Graceful degradation

### Gmail Capabilities
- Full Gmail search query syntax support
- Thread-aware replies
- HTML and plain text emails
- Label management (system and custom labels)
- Draft workflow support
- Batch operations for efficiency

### Calendar Capabilities
- Multi-calendar support
- Recurring events
- Attendee management
- Free/busy time lookup
- Event search

### People API Capabilities
- Contact CRUD operations
- Advanced search
- Contact groups
- Field-level updates

## Requirements

- Python 3.12 or higher
- Google Cloud Project with APIs enabled (production mode)
- OR ish server running (testing mode)
- OAuth 2.0 credentials (production mode)

## Security

- Credentials never committed to version control
- Token files excluded via `.gitignore`
- OAuth 2.0 best practices
- Minimal scope requests
- Secure credential storage

For production deployments, consider using service accounts with domain-wide delegation.

## Troubleshooting

See the [Setup Guide](docs/setup.md#troubleshooting) for common issues:

- OAuth authentication errors
- Rate limiting
- API quota management
- Ish mode connection issues

## License

MIT

## Contributing

Contributions welcome! Please ensure:
- Tests pass: `uv run pytest`
- Code follows existing style
- Documentation updated for new features
- No breaking changes without discussion

## Support

- Documentation: See [docs/](docs/) directory
- Issues: Open a GitHub issue
- Examples: See [Usage Guide](docs/usage.md)
