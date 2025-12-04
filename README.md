# GSuite MCP Server

A comprehensive MCP server for Google Workspace APIs (Gmail, Calendar, People).

## Features

- **Gmail**: Full email management (read, send, reply, drafts, labels, attachments, batch operations)
- **Calendar**: Complete calendar operations (events, recurring events, multiple calendars)
- **People**: Contact management (CRUD operations, search, groups)
- **Authentication**: OAuth 2.0 with credential caching
- **Resilience**: Smart retry logic with exponential backoff

## Setup

1. Install dependencies: `uv sync`
2. Configure Google OAuth credentials (see docs/setup.md)
3. Run the server: `uv run python -m gsuite_mcp`

## Configuration

Place your `credentials.json` from Google Cloud Console in the project root.
The server will guide you through OAuth flow on first run.
