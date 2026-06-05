# Ish Mode — Testing Against a Mock Google API

Ish mode points the server's Google API clients at a mock HTTP server instead of the
real Gmail, Calendar, People, and Tasks APIs, using fake Bearer-token auth instead of
OAuth. It exists for local development and CI, so you can exercise the server without
real credentials or touching live data.

## Enable it

Set environment variables before starting the server (or running tests):

| Variable | Description | Default |
|----------|-------------|---------|
| `ISH_MODE` | Enable ish mode. Must be exactly `true` (case-sensitive; `TRUE` does not count) | unset (off) |
| `ISH_BASE_URL` | Base URL of the mock server | `http://localhost:9000` |
| `ISH_USER` | Username embedded in the fake Bearer token | `testuser` |

```bash
export ISH_MODE=true
export ISH_BASE_URL=http://localhost:9000
export ISH_USER=testuser@example.com
./gsuite-mcp mcp
```

Or in an MCP client config:

```json
{
  "mcpServers": {
    "gsuite-test": {
      "command": "/path/to/gsuite-mcp",
      "args": ["mcp"],
      "env": {
        "ISH_MODE": "true",
        "ISH_BASE_URL": "http://localhost:9000",
        "ISH_USER": "testuser@example.com"
      }
    }
  }
}
```

## How it works

When `ISH_MODE=true`:

- The server (`NewServer` in `pkg/server`) builds a fake HTTP client with
  `auth.NewFakeClient` and an in-memory `default` account — no OAuth, no
  `credentials.json` required.
- Each service's `NewService` (Gmail, Calendar, People, Tasks) points the Google SDK at
  `ISH_BASE_URL` with `option.WithEndpoint`, disables the SDK's built-in OAuth with
  `option.WithoutAuthentication()`, and sends requests through the fake client handed
  down from the server.

The fake client stamps every request with a static header:

```
Authorization: Bearer user:<ISH_USER>
```

All tools, resources, and prompts go through those services, so the whole surface talks
to the mock. The auth tools (`auth_status`, `auth_info`, `auth_init`, `auth_complete`,
`auth_revoke`) short-circuit in ish mode and report simulated success instead of touching
real OAuth.

## You provide the mock server

**No mock server ships with this repo.** Supply one that:

1. Listens at `ISH_BASE_URL`,
2. Accepts the `Bearer user:USERNAME` header, and
3. Implements the Google REST endpoints your test exercises (Gmail, Calendar, People,
   Tasks).

Point `ISH_BASE_URL` at it. Without a running mock, calls fail with connection errors —
which is expected, and several tests assert exactly that.

## Running tests

```bash
go test ./...
```

Unit tests set `ISH_MODE` themselves and verify the wiring (that clients are built and
point at the mock); they do not need `credentials.json`. Integration tests that require a
live mock server call `t.Skip` when no mock server is reachable, so they are skipped
unless one is running.

## Where it lives

- `pkg/auth/fake.go` — `NewFakeClient`, the fake Bearer transport
- `pkg/gmail/service.go`, `pkg/calendar/service.go`, `pkg/people/service.go`,
  `pkg/tasks/service.go` — `ISH_MODE` checks in `NewService`
- `pkg/server/server.go` — the fake client + in-memory account, and the simulated
  auth-tool responses

## Troubleshooting

**Connection refused / timeouts** — no mock server is running at `ISH_BASE_URL`. Start
one, or correct the URL.

**Ish mode seems off** — `ISH_MODE` must be the exact string `true`. Any other value
(including `TRUE` or `1`) leaves the server in normal OAuth mode.

```bash
echo "$ISH_MODE" "$ISH_BASE_URL" "$ISH_USER"
```
