# GSuite MCP Server Setup Guide

## Prerequisites

- Python 3.12 or higher
- Google Cloud Project with Gmail, Calendar, and People APIs enabled
- OAuth 2.0 credentials from Google Cloud Console (for production mode)
- OR access to an ish server (for testing mode)

## Google Cloud Setup (Production Mode)

### 1. Create a Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Note your project ID

### 2. Enable Required APIs

Enable these APIs in your project:

```bash
gcloud services enable gmail.googleapis.com
gcloud services enable calendar-json.googleapis.com
gcloud services enable people.googleapis.com
```

Or enable via Console:
- Gmail API
- Google Calendar API
- People API

### 3. Create OAuth 2.0 Credentials

1. Go to **APIs & Services > Credentials**
2. Click **Create Credentials > OAuth client ID**
3. Choose **Desktop app** as application type
4. Name it "GSuite MCP Server"
5. Download the JSON file
6. Save as `credentials.json` in project root

### 4. Configure OAuth Consent Screen

1. Go to **APIs & Services > OAuth consent screen**
2. Choose **External** user type
3. Fill in app name: "GSuite MCP Server"
4. Add your email as developer contact
5. Add scopes:
   - Gmail API: `.../auth/gmail.modify`
   - Calendar API: `.../auth/calendar`
   - People API: `.../auth/contacts`
6. Add test users (your email) if not publishing app

## Installation

### 1. Clone and Install

```bash
cd gsuite-mcp
uv sync
```

### 2. Place Credentials (Production Mode)

Copy your `credentials.json` to the project root:

```bash
cp ~/Downloads/credentials.json ./credentials.json
```

### 3. First Run - OAuth Flow (Production Mode)

On first run, the server will:
1. Open browser for OAuth consent
2. Ask you to sign in with Google account
3. Request permission for scopes
4. Save token to `token.json`

Test the authentication:

```bash
uv run python -c "from gsuite_mcp.auth.oauth import OAuth2Authenticator; OAuth2Authenticator().get_credentials()"
```

## Ish Mode Setup (Testing/Development)

For testing without real Google credentials, you can use ish mode with a fake Google API server.

### 1. Start the Ish Server

```bash
# Start ish on default port 9000
ish serve
```

### 2. Configure Ish Mode

Set environment variables:

```bash
export ISH_MODE=true
export ISH_BASE_URL=http://localhost:9000  # Optional, defaults to this
export ISH_USER=testuser                    # Optional, defaults to testuser
```

### 3. Run the Server

```bash
uv run python -m gsuite_mcp
```

No OAuth flow required! The server will automatically use fake credentials.

See [ISH_MODE.md](ISH_MODE.md) for detailed ish mode documentation.

## Running the Server

### Standalone

```bash
uv run python -m gsuite_mcp
```

### With MCP Client

Add to your MCP client configuration:

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

**Ish Mode:**
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

## Troubleshooting

### "credentials.json not found"

**Production Mode:** Make sure `credentials.json` is in the project root directory.

**Ish Mode:** Either set `ISH_MODE=true` environment variable or provide `api_base_url` parameter when creating services.

### "Access denied" errors

1. Check OAuth consent screen has correct scopes
2. Ensure your account is added as test user
3. Delete `token.json` and re-authenticate

### Rate limit errors

Google APIs have quota limits. The server uses automatic retry with exponential backoff, but if you hit daily quotas, you'll need to wait or request quota increases in Cloud Console.

### Connection Refused (Ish Mode)

Make sure the ish server is running:

```bash
curl http://localhost:9000/discovery/v1/apis/gmail/v1/rest
```

If ish is on a different port, set `ISH_BASE_URL`:

```bash
export ISH_BASE_URL=http://localhost:8888
```

## Security Notes

- **Never commit `credentials.json` or `token.json` to git**
- `.gitignore` includes these files by default
- Token file contains access tokens - keep it secure
- For production use, consider service accounts with domain-wide delegation
- Ish mode credentials are for testing only and provide no real security
