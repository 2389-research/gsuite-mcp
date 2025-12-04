# Ish Mode - Fake Google API Server Integration

This document describes how to use the Google Workspace MCP server with the "ish" fake Google API server for testing.

## What is Ish?

Ish is a fake Google API server that emulates Gmail, Calendar, and People APIs on `http://localhost:9000`. It uses simple Bearer token authentication instead of OAuth, making it ideal for testing and development.

## Quick Start

### Using Environment Variables

The easiest way to enable ish mode is via environment variables:

```bash
export ISH_MODE=true
export ISH_BASE_URL=http://localhost:9000  # Optional, defaults to this
export ISH_USER=testuser                    # Optional, defaults to testuser

# Now use GmailService normally
python your_script.py
```

### Programmatic Configuration

You can also configure ish mode programmatically:

```python
from gsuite_mcp.services.gmail import GmailService

# Option 1: Using api_base_url with auto-created credentials
service = GmailService(api_base_url="http://localhost:9000")

# Option 2: Using api_base_url with explicit auth token
service = GmailService(
    api_base_url="http://localhost:9000",
    auth_token="user:alice"
)

# Option 3: Using FakeCredentials directly
from gsuite_mcp.auth.fake_credentials import FakeCredentials

creds = FakeCredentials(user="bob")
service = GmailService(
    credentials=creds,
    api_base_url="http://localhost:9000"
)
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ISH_MODE` | Set to "true" to enable ish mode | `false` |
| `ISH_BASE_URL` | Base URL for ish server | `http://localhost:9000` |
| `ISH_USER` | Username for authentication | `testuser` |

## Authentication

Ish uses simple Bearer token authentication with the format:

```
Authorization: Bearer user:username
```

The `FakeCredentials` class handles this automatically. You can provide:

1. A full token: `FakeCredentials(token="user:alice")`
2. Just a username: `FakeCredentials(user="alice")`
3. Nothing (uses `ISH_USER` env var or defaults to "testuser")

## How It Works

When ish mode is enabled:

1. The `GmailService` uses a custom discovery URL pointing to ish
2. FakeCredentials are created automatically if not provided
3. All API calls go to the ish server instead of Google's APIs
4. No OAuth flow is required

The discovery URL format is:
```
{api_base_url}/discovery/v1/apis/gmail/v1/rest
```

## Production Mode vs Ish Mode

### Production Mode (Default)

```python
from google.oauth2.credentials import Credentials
from gsuite_mcp.services.gmail import GmailService

# Requires real OAuth credentials
creds = Credentials.from_authorized_user_file('token.json')
service = GmailService(credentials=creds)
# Uses: https://gmail.googleapis.com
```

### Ish Mode

```python
from gsuite_mcp.services.gmail import GmailService

# No OAuth required
service = GmailService(api_base_url="http://localhost:9000")
# Uses: http://localhost:9000/gmail/v1/...
```

## Testing Example

```python
import pytest
from gsuite_mcp.services.gmail import GmailService

def test_with_ish():
    """Test Gmail operations using ish server."""
    # Assumes ish is running on localhost:9000
    service = GmailService(
        api_base_url="http://localhost:9000",
        auth_token="user:testuser"
    )

    # Send a test message
    result = service.send_message(
        to="recipient@example.com",
        subject="Test Email",
        body="This is a test from ish mode"
    )

    assert result['id']

    # List messages
    messages = service.list_messages(query="is:unread")
    assert isinstance(messages, list)
```

## Running Ish Server

To use ish mode, you need to have the ish server running. Refer to the ish documentation for setup instructions.

Typical ish startup:

```bash
# Start ish on default port 9000
ish serve
```

## Backward Compatibility

The ish mode changes are fully backward compatible:

- Existing code using OAuth credentials continues to work unchanged
- Production mode is the default when `ISH_MODE` is not set
- No breaking changes to the `GmailService` API

## Implementation Details

### Files Modified

1. `src/gsuite_mcp/auth/fake_credentials.py` - New file implementing FakeCredentials
2. `src/gsuite_mcp/services/gmail.py` - Updated to support ish mode
3. `tests/test_ish_mode.py` - Comprehensive tests for ish integration

### Key Features

- Automatic detection of ish mode via `ISH_MODE` environment variable
- Custom discovery URL support for ish server
- FakeCredentials class that mimics OAuth credentials interface
- Full test coverage (15 tests covering all ish scenarios)
- Zero impact on production mode

## Troubleshooting

### "credentials are required" Error

This means you're not in ish mode and haven't provided OAuth credentials. Either:

1. Set `ISH_MODE=true` environment variable, or
2. Provide `api_base_url` parameter, or
3. Provide valid OAuth `credentials`

### Connection Refused

Make sure the ish server is running:

```bash
curl http://localhost:9000/discovery/v1/apis/gmail/v1/rest
```

If ish is on a different port, set `ISH_BASE_URL`:

```bash
export ISH_BASE_URL=http://localhost:8888
```

## Future Enhancements

- Support for Calendar and People services in ish mode
- Helper scripts to start/stop ish server
- Docker compose configuration with ish included
- Integration tests that actually call ish server
