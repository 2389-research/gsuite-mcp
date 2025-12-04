# GSuite MCP Server (Go)

MCP server for Google Workspace APIs (Gmail, Calendar, People).

## Features

- Gmail API (messages, drafts, labels, attachments)
- Calendar API (events, recurring events)
- People API (contacts, groups)
- OAuth 2.0 authentication
- Ish mode for testing

## Setup

```bash
go build ./cmd/gsuite-mcp
./gsuite-mcp
```

## Testing

```bash
go test ./...
```
