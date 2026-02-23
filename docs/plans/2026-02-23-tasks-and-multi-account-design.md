# Google Tasks & Multi-Account Support Design

## Overview

Two features delivered sequentially:

1. **Google Tasks** - Full CRUD for tasks and task lists (~8 tools, 3 resources, 2 prompts)
2. **Multi-Account** - Personal + work account support via named aliases with explicit per-call account selection

Tasks ships first on the existing single-account model. Multi-account retrofits all services afterward.

---

## Feature 1: Google Tasks

### Package: `pkg/tasks/`

Follows the existing service pattern (`pkg/gmail/`, `pkg/calendar/`, `pkg/people/`). Wraps `google.golang.org/api/tasks/v1`.

**OAuth Scope**: `tasks.TasksScope` (`https://www.googleapis.com/auth/tasks`)

Existing users will need to re-auth once to grant the Tasks scope.

### Tools (8)

| Tool | Description | Key Params |
|------|-------------|------------|
| `tasks_list_tasklists` | List all task lists | (none required) |
| `tasks_create_tasklist` | Create a new task list | `title` (required) |
| `tasks_update_tasklist` | Rename a task list | `tasklist_id`, `title` |
| `tasks_delete_tasklist` | Delete a task list | `tasklist_id` |
| `tasks_list_tasks` | List tasks in a list | `tasklist_id` (default: `@default`), `show_completed`, `show_hidden`, `due_min`, `due_max` |
| `tasks_create_task` | Create a task | `tasklist_id`, `title` (required), `notes`, `due`, `parent` (for subtasks) |
| `tasks_update_task` | Update a task | `tasklist_id`, `task_id`, `title`, `notes`, `due`, `status` |
| `tasks_delete_task` | Delete a task | `tasklist_id`, `task_id` |

### Resources (3)

| URI | Description |
|-----|-------------|
| `gsuite://tasks/today` | Tasks due today |
| `gsuite://tasks/upcoming` | Tasks due in the next 7 days |
| `gsuite://tasks/overdue` | Overdue incomplete tasks |

### Prompts (2)

| Prompt | Description |
|--------|-------------|
| `task_review` | Review and organize pending tasks |
| `plan_tasks` | Break down a goal into actionable tasks |

### Response Wrapping

Arrays wrapped in objects, consistent with other services:

```json
{
  "task_lists": [...],
  "count": 3
}
```

---

## Feature 2: Multi-Account

### Account Registry Config

Location: `~/.config/gsuite-mcp/accounts.json` (XDG-compliant, overridable via `GSUITE_MCP_ACCOUNTS_PATH`)

```json
{
  "default": "work",
  "accounts": {
    "work": {
      "token_path": "~/.local/share/gsuite-mcp/tokens/work.json"
    },
    "personal": {
      "token_path": "~/.local/share/gsuite-mcp/tokens/personal.json"
    }
  }
}
```

Shared `credentials.json` (one Google Cloud project). Each account gets its own token via separate OAuth consent.

### Package: `pkg/accounts/`

**`registry.go`** manages account lifecycle:

- `LoadRegistry(path string) (*Registry, error)` - Load config or create default single-account registry
- `registry.GetAccount(alias string) (*Account, error)` - Get by name
- `registry.GetDefault() (*Account, error)` - Get default account
- `registry.Resolve(alias string) (*Account, error)` - Empty alias returns default, otherwise looks up by name
- `registry.ListAccounts() []string` - List all aliases
- `registry.AddAccount(alias, tokenPath string) error` - Add account, writes config

Each `Account` holds:
- `Alias string`
- `TokenPath string`
- `Client *http.Client` (lazily initialized)

### Server Architecture Changes

Current (single-account):

```go
type Server struct {
    gmail    *gmail.Service
    calendar *calendar.Service
    people   *people.Service
}
```

New (multi-account):

```go
type Server struct {
    registry  *accounts.Registry
    services  map[string]*AccountServices  // keyed by alias, lazily populated
    auth      *auth.Authenticator
    mcp       *server.MCPServer
}

type AccountServices struct {
    Gmail    *gmail.Service
    Calendar *calendar.Service
    People   *people.Service
    Tasks    *tasks.Service
}
```

### Tool Parameter Changes

Every tool gains an optional `account` string parameter:

```json
{
  "account": {
    "type": "string",
    "description": "Account alias (e.g., 'work', 'personal'). Uses default if not specified."
  }
}
```

Handler resolution pattern:

```go
func (s *Server) handleGmailListMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    account := req.GetString("account", "")
    svc, err := s.resolveServices(ctx, account)
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }
    // use svc.Gmail.ListMessages(...)
}
```

### Auth Tool Changes

Existing auth tools become account-aware:

- `auth_init(account)` - Start OAuth for an account. Creates the account in the registry if it doesn't exist yet.
- `auth_complete(account, code)` - Complete OAuth for that account
- `auth_status(account)` - Auth status. No account param shows all accounts.
- `auth_info(account)` - Token info for that account
- `auth_revoke(account)` - Revoke and delete token for that account

New tool:

- `accounts_list` - List all configured accounts and their auth status

### Backward Compatibility

If no `accounts.json` exists and the legacy `token.json` has a token:

1. Server creates an in-memory single-account registry with alias `"default"`
2. Everything works exactly as before
3. First `auth_init` call with an account name triggers config file creation

### Error Handling

- Unknown alias: `"unknown account 'foo'. Available accounts: work, personal"`
- Not authenticated: `"account 'personal' is not authenticated. Run auth_init with account='personal' to set up."`
- No accounts and no legacy token: `"no accounts configured. Run auth_init to set up your first account."`

---

## Implementation Order

### Phase 1: Google Tasks (single-account)

1. Add `tasks.TasksScope` to OAuth scopes in `pkg/auth/`
2. Create `pkg/tasks/service.go`
3. Register 8 tools in `pkg/server/server.go`
4. Add 3 resources and 2 prompts
5. Tests

### Phase 2: Multi-Account Retrofit

1. Create `pkg/accounts/registry.go`
2. Refactor `pkg/server/server.go` - replace single-service fields with `AccountServices` map + lazy init
3. Update all ~32 tool handlers to resolve account param (mechanical change)
4. Update auth tools to be account-aware
5. Add `accounts_list` tool
6. Update CLI commands (`setup`, `test`, `whoami`) to accept `--account` flag
7. Tests

### What Doesn't Change

- Service packages (`pkg/gmail/`, `pkg/calendar/`, `pkg/people/`, `pkg/tasks/`) are untouched. They already accept `*http.Client` at construction.
- Retry logic
- ISH mode (single fake account)
- MCP transport (stdio)

### Testing Strategy

- Unit tests for `pkg/tasks/` service methods
- Unit tests for `pkg/accounts/` registry (load, resolve, add, fallback)
- Integration tests for account resolution in handlers
- E2E tests for full flow: auth_init with account -> auth_complete -> tool call with account param
