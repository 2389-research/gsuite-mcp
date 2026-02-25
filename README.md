# GSuite MCP Server

An MCP server that connects your AI to Gmail, Google Calendar, Contacts, and Tasks.

![GSuite MCP Server](docs/gsuite_mcp.png)

## Install

```bash
brew install 2389-research/tap/gsuite-mcp
```

Or build from source:

```bash
go build ./cmd/gsuite-mcp
```

## Setup

### 1. Create Google OAuth credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Create a project (or pick an existing one)
3. Click **+ CREATE CREDENTIALS** > **OAuth client ID** > **Desktop app**
4. Download the JSON and save it to `~/.config/gsuite-mcp/credentials.json`

### 2. Enable the APIs

Click "Enable" on each of these:

- [Gmail API](https://console.cloud.google.com/apis/library/gmail.googleapis.com)
- [Calendar API](https://console.cloud.google.com/apis/library/calendar-json.googleapis.com)
- [People API](https://console.cloud.google.com/apis/library/people.googleapis.com)
- [Tasks API](https://console.cloud.google.com/apis/library/tasks.googleapis.com)

### 3. Authenticate

```bash
gsuite-mcp setup
```

This opens a browser to sign in with Google. The token is saved locally so you only do this once.

### 4. Connect to your AI client

For Claude Code:

```bash
claude mcp add gsuite gsuite-mcp -- mcp    # project scope (default)
claude mcp add gsuite gsuite-mcp --scope user -- mcp   # user scope (all projects)
```

For Claude Desktop or other MCP clients, add this to your config:

```json
{
  "mcpServers": {
    "gsuite": {
      "command": "gsuite-mcp",
      "args": ["mcp"]
    }
  }
}
```

Done. Your AI can now work with your Google account.

## Multiple accounts

You can connect more than one Google account:

```bash
gsuite-mcp setup --account work
gsuite-mcp setup --account personal
```

Then just tell your AI which one:

> "Check my **work** email for anything urgent"
> "What's on my **personal** calendar this week?"

Every tool accepts an optional `account` parameter. Without one, it uses the default.

```bash
$ gsuite-mcp whoami --account work
Account:  work
Email:    harper@2389.ai
Messages: 4210 total
Threads:  1847 total
```

## What it can do

33 tools across four Google APIs, plus prompts and live data.

**Gmail** -- read, search, send, draft, label, trash, delete. CC/BCC supported.

**Calendar** -- list, view, create, update, and delete events.

**Contacts** -- search, browse, create, update, delete.

**Tasks** -- manage task lists and tasks (create, update, complete, delete).

The server also ships with prompts for common workflows: triaging email, composing replies with proper threading, scheduling meetings around your availability, reviewing pending tasks, breaking goals into task lists, and looking up or adding contacts. Your AI picks these up automatically.

For quick context, the server exposes live data as MCP resources: today's calendar, upcoming events, unread mail, current drafts, tasks due today, overdue tasks, and so on.

## Security

Tokens live on your machine in `~/.local/share/gsuite-mcp/`. Nothing leaves your computer. The server only requests the OAuth scopes it needs. Keep `credentials.json` and token files out of git.

## CLI reference

```
gsuite-mcp setup                   # Interactive setup wizard
gsuite-mcp setup --account work    # Set up a named account
gsuite-mcp test                    # Test your API connection
gsuite-mcp whoami                  # Show authenticated user
gsuite-mcp mcp                     # Start the MCP server
gsuite-mcp version                 # Show version
gsuite-mcp help                    # Show help
```

## Documentation

See [docs/](docs/) for detailed guides:

- [Setup](docs/setup.md) -- step-by-step setup with screenshots
- [Usage](docs/usage.md) -- full tool reference and examples
- [ISH Mode](docs/ISH_MODE.md) -- testing with a local mock server

## Contributing

1. Fork the repo
2. Create a branch
3. Write tests
4. `go test ./...`
5. Open a PR

## License

MIT
