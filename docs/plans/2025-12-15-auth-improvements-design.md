# Auth Improvements Design

## Problem

1. **Refreshed tokens not persisted**: When `oauth2.TokenSource` auto-refreshes an expired access token, the new token stays in memory and never gets written back to `token.json`. Server restarts cause unnecessary re-auth cycles.

2. **No MCP tools for auth recovery**: When auth fails, there's no way to diagnose or fix it without restarting the server and going through the interactive terminal flow.

## Solution

### Part 1: PersistentTokenSource

Wrapper around `oauth2.TokenSource` that auto-saves refreshed tokens to disk.

```go
type PersistentTokenSource struct {
    source    oauth2.TokenSource
    lastToken *oauth2.Token
    saveFn    func(*oauth2.Token) error
    mu        sync.Mutex
}
```

On each `Token()` call, checks if access token changed and persists if so.

### Part 2: MCP Auth Tools

| Tool | Purpose | Parameters |
|------|---------|------------|
| `auth_status` | Quick health check (makes API call) | None |
| `auth_info` | Token diagnostics (local state) | None |
| `auth_init` | Start OAuth flow | `force` (bool, optional) |
| `auth_complete` | Finish OAuth flow | `code` (string, required) |
| `auth_revoke` | Delete cached token | None |

**Behaviors:**

- `auth_init` smart default: Returns status if auth valid, URL if invalid. `force=true` always returns URL.
- `auth_complete` is stateful: OAuth config already in memory, just needs the code.
- `auth_status` validates by making a lightweight API call (Gmail labels).

### Architecture Changes

Server stores `Authenticator` reference alongside HTTP client:

```go
type Server struct {
    mcpServer *server.MCPServer
    client    *http.Client
    auth      *auth.Authenticator
    // ...services
}
```

New Authenticator methods:
- `TokenInfo() (*TokenInfo, error)` - metadata without API calls
- `AuthURL() string` - generate URL without blocking
- `ExchangeCode(ctx, code) error` - exchange and save
- `RefreshClient(ctx) (*http.Client, error)` - new client after auth

### Error Handling

- In-flight requests during `auth_complete`: May fail, acceptable
- `auth_complete` without `auth_init`: Works (config loaded at startup)
- Invalid code: Error surfaced, token file unchanged
- Missing token file: `auth_status` returns `valid: false`

## Files

**Modify:**
- `pkg/auth/oauth.go` - PersistentTokenSource, TokenInfo, new methods
- `pkg/server/server.go` - Store authenticator, register tools

**Create:**
- `pkg/server/server_auth_test.go` - Integration tests
