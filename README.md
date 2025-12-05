# GSuite MCP Server (Go)

A Model Context Protocol (MCP) server that provides programmatic access to Google Workspace APIs including Gmail, Google Calendar, and Google Contacts. Built in Go for performance and reliability.

![GSuite MCP Server](docs/gsuite_mcp.png)

## Features

- **Gmail Integration**: List and send email messages with Gmail query syntax
- **Calendar Access**: View upcoming events and schedule management
- **Contacts Management**: Access and search your Google contacts
- **OAuth 2.0 Authentication**: Secure access to your Google Workspace data
- **Ish Mode Testing**: Test without real credentials using mock API server
- **Automatic Retry Logic**: Exponential backoff for rate limits and transient errors
- **Stdio Transport**: Seamless integration with MCP clients

## Available Tools

The server exposes 4 MCP tools:

1. **gmail_list_messages** - Search and list Gmail messages
2. **gmail_send_message** - Send email messages
3. **calendar_list_events** - List calendar events with time filtering
4. **people_list_contacts** - List contact information

See [docs/usage.md](docs/usage.md) for detailed tool documentation and examples.

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
# Production mode (requires credentials.json)
./gsuite-mcp

# Testing mode (requires ish mock server)
ISH_MODE=true ISH_BASE_URL=http://localhost:9000 ./gsuite-mcp
```

### Configure MCP Client

Add to your MCP client configuration (e.g., Claude Desktop):

```json
{
  "mcpServers": {
    "gsuite": {
      "command": "/path/to/gsuite-mcp",
      "args": [],
      "env": {}
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
│       └── server.go        # MCP server implementation
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

- Additional Gmail tools (get message, modify labels, drafts)
- Calendar event creation and modification
- Contact creation and updates
- Batch operations support
- Service account authentication
- Advanced query filters

See [docs/usage.md](docs/usage.md) for the full list of planned tools.
