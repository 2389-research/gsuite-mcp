# GSuite MCP Server Setup Guide

This guide covers setting up the GSuite MCP server with both OAuth 2.0 authentication (for production) and ish mode (for testing).

## Prerequisites

- Go 1.21 or later
- Google Cloud account (for OAuth setup)
- Access to Google Workspace APIs

## Quick Start

### Build the Server

```bash
go build ./cmd/gsuite-mcp
```

This creates the `gsuite-mcp` binary in the current directory.

## OAuth 2.0 Setup (Production)

OAuth 2.0 is the primary authentication method for production use. It allows the server to access your Google Workspace data securely.

### Step 1: Create Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Note your project ID

### Step 2: Enable Required APIs

Enable the following APIs for your project:

1. **Gmail API**
   - Navigate to "APIs & Services" > "Library"
   - Search for "Gmail API"
   - Click "Enable"

2. **Google Calendar API**
   - Search for "Google Calendar API"
   - Click "Enable"

3. **People API**
   - Search for "People API"
   - Click "Enable"

### Step 3: Configure OAuth Consent Screen

1. Go to "APIs & Services" > "OAuth consent screen"
2. Choose "Internal" (for Google Workspace) or "External" (for personal use)
3. Fill in required information:
   - App name: "GSuite MCP Server"
   - User support email: Your email
   - Developer contact: Your email
4. Click "Save and Continue"
5. Add scopes (click "Add or Remove Scopes"):
   - `.../auth/gmail.modify`
   - `.../auth/gmail.labels`
   - `.../auth/calendar`
   - `.../auth/contacts`
6. Click "Save and Continue"
7. Add test users if using external mode

### Step 4: Create OAuth Credentials

1. Go to "APIs & Services" > "Credentials"
2. Click "Create Credentials" > "OAuth client ID"
3. Application type: "Desktop app"
4. Name: "GSuite MCP Client"
5. Click "Create"
6. Download the JSON file
7. Save as `credentials.json` in the same directory as the binary

### Step 5: First Run and Authorization

On first run, the server will prompt for authorization:

```bash
./gsuite-mcp
```

You'll see:
```
Go to the following link in your browser:
https://accounts.google.com/o/oauth2/auth?...

Enter authorization code:
```

1. Open the URL in your browser
2. Sign in with your Google account
3. Grant permissions
4. Copy the authorization code from the browser
5. Paste into the terminal and press Enter

The server will save the token to `token.json` for future use. You won't need to authorize again unless you revoke the token.

### Step 6: Verify Setup

The server is now running and ready to accept MCP requests via stdio.

## Ish Mode Setup (Testing)

Ish mode allows you to test the server without real OAuth credentials. It's perfect for development, CI/CD, and automated testing.

### What is Ish Mode?

Ish mode uses fake Bearer token authentication instead of OAuth. All API requests are redirected to a mock server (typically running locally) that simulates Google Workspace APIs.

### Step 1: Set Environment Variables

Create a `.env` file or set these environment variables:

```bash
export ISH_MODE=true
export ISH_BASE_URL=http://localhost:9000
export ISH_USER=testuser
```

Parameters:
- `ISH_MODE`: Set to `true` to enable ish mode
- `ISH_BASE_URL`: URL of your mock API server (default: `http://localhost:9000`)
- `ISH_USER`: Username for Bearer token authentication (default: `testuser`)

### Step 2: Start Mock Server

You'll need a mock server that implements the Google Workspace API endpoints. The server should:

1. Listen on the port specified in `ISH_BASE_URL`
2. Accept Bearer token authentication with format `Bearer user:USERNAME`
3. Implement endpoints for Gmail, Calendar, and People APIs

Example using the ish framework:
```bash
# Start ish mock server
ish server start --port 9000
```

### Step 3: Run the Server

```bash
./gsuite-mcp
```

The server will use fake authentication and connect to your mock server instead of real Google APIs.

### Step 4: Run Tests

All tests use ish mode by default:

```bash
go test ./...
```

Tests automatically set `ISH_MODE=true` and don't require credentials.json.

## MCP Client Configuration

To use the server with an MCP client (like Claude Desktop), add it to your MCP settings:

### Claude Desktop Configuration

Edit your Claude Desktop config file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

Add the server:

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

For ish mode testing:
```json
{
  "mcpServers": {
    "gsuite-test": {
      "command": "/path/to/gsuite-mcp",
      "args": [],
      "env": {
        "ISH_MODE": "true",
        "ISH_BASE_URL": "http://localhost:9000",
        "ISH_USER": "testuser"
      }
    }
  }
}
```

## Troubleshooting

### "credentials.json not found"

Make sure `credentials.json` is in the same directory as the binary, or set the path:

```bash
export GOOGLE_CREDENTIALS_PATH=/path/to/credentials.json
```

### "Failed to create Gmail service"

Check that all required APIs are enabled in Google Cloud Console:
- Gmail API
- Google Calendar API
- People API

### OAuth Token Expired

Delete `token.json` and re-run the server. You'll be prompted to re-authorize.

```bash
rm token.json
./gsuite-mcp
```

### Ish Mode Not Working

Verify environment variables are set:
```bash
echo $ISH_MODE
echo $ISH_BASE_URL
echo $ISH_USER
```

Make sure your mock server is running and accessible at `ISH_BASE_URL`.

## Security Considerations

1. **Never commit credentials.json or token.json** - They are listed in `.gitignore` by default
2. **Rotate OAuth tokens regularly** - Delete and regenerate if compromised
3. **Use minimal scopes** - Only enable the APIs you need
4. **Restrict OAuth consent screen** - Use "Internal" mode for Google Workspace organizations
5. **Ish mode is for testing only** - Never use in production environments

## Advanced Configuration

### Custom Token Path

```bash
export GOOGLE_TOKEN_PATH=/custom/path/token.json
./gsuite-mcp
```

### Custom Credentials Path

```bash
export GOOGLE_CREDENTIALS_PATH=/custom/path/credentials.json
./gsuite-mcp
```

### Multiple Accounts

Run separate instances with different token files:

```bash
# Account 1
GOOGLE_TOKEN_PATH=token-account1.json ./gsuite-mcp

# Account 2
GOOGLE_TOKEN_PATH=token-account2.json ./gsuite-mcp
```

## Next Steps

- See [Usage Guide](usage.md) for tool reference and examples
- See [README](../README.md) for project overview and testing
