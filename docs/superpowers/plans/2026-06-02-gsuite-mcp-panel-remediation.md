# gsuite-mcp — Expert Panel Remediation Plan

**Date:** 2026-06-02
**Source:** 10-reviewer expert panel (`/review-squad:experts`) over the Go MCP server, CI/release pipeline, docs, and web site.
**Author note:** Every finding below was re-verified against the actual code before being written here. Two consolidated severities were **corrected during verification** (see "Corrections to the consolidated report").

---

## How to use this plan

- **Recommended execution:** subagent-driven, one task at a time, with a spec-compliance + code-quality review between tasks (superpowers `subagent-driven-development`). Each task is sized so a single subagent can complete and verify it.
- **Constraint (from CLAUDE.md):** max **5 files per phase**; the repo must **build green between every task**. Tasks are ordered so this always holds.
- **TDD:** every code task is RED → GREEN → REFACTOR. Write the failing test, run it, see it fail for the stated reason, then implement.
- **Branch strategy:** the tracks are independent. Suggested branches:
  - `fix/critical-server-hardening` → Phase 1 + Phase 2 (Go code)
  - `fix/surface-and-prompts` → Phase 3
  - `ci/release-hardening` → Phase 4
  - `docs/site-and-marketing` → Phase 5
  - `test/coverage-backfill` → Phase 6
  - `chore/minor-cleanups` → Phase 7
  Merge each to `main` via PR. Do **not** use `--no-verify`.

## Scope check (writing-plans)

These findings span **five independent subsystems**: (1) Go server code, (2) CI/release, (3) prose docs, (4) the web site, (5) the test suite. Each phase below produces working, testable software on its own and can become its own PR. Phases 1–2 are the high-risk Go work and are fully specified as TDD tasks. Phases 3–7 are concrete task lists (exact files, edits, and verification) — split any of them into a dedicated plan if you'd rather drive them separately.

## Corrections to the consolidated report

| Was | Verified reality | New severity |
|-----|------------------|--------------|
| **C5** "auth_info leaks access token" | `auth_info` returns `maskToken(token)` = first-4 + `...` + last-4 (`oauth.go:277,291`). Not a raw token. Still needless exposure into the model's context. | **IMPORTANT** (was CRITICAL) — drop the field |
| **C3** "server stdout/stdin corruption (always)" | `NewServer` eager path already uses the non-interactive `GetClientIfAuthenticated` (`server.go:75`). The **only** remaining hole is the lazy-load path for non-default accounts: `resolveServices` still calls `GetClient` (`server.go:168`), which can fall through to interactive `authenticate()` (prints to stdout, blocks on stdin) and corrupt the MCP stdio framing. | **CRITICAL** (narrowed scope) |
| **C1** "exposed credentials" | Handled out-of-band on 2026-06-02: `chmod 600` on the three cred files; `.private-journal/` added to `.gitignore`. **Rotate/revoke is still pending on Doctor Biz** (see Phase 0). | CRITICAL (ops, not code) |

---

## Phase 0 — Security (out-of-band, mostly done)

Already completed this session (no code):
- `chmod 600` on `credentials.json`, `token.json`, `~/.config/gsuite-mcp/credentials.json` (verified `-rw-------`).
- `.private-journal/` added to `.gitignore` (verified ignored).

**Still owed by Doctor Biz (cannot be automated — touches the live Google account):**
1. Rotate the OAuth **client secret** in Google Cloud Console (Credentials → OAuth client → reset secret), then update local `credentials.json`.
2. Revoke the stored **refresh token** at <https://myaccount.google.com/permissions> (the in-app `auth_revoke` only deletes the local file — see Task 2.7).
3. Optional: move the root-CWD credential copies out of `~/Public`.

> These steps gate nothing in this plan but should happen before any further real-account use.

---

## Phase 1 — CRITICAL (Go code)

### Task 1.1 — Gate retries to idempotent operations (safety precondition)

**Why first:** Task 1.2 makes retry *actually fire*. Today retry never fires (the bug), so wrapping `Send*`/`Create*` calls is harmless — but the moment 1.2 lands, a retried send/create would **duplicate** the message/event/contact. Removing the retry wrapper from non-idempotent ops **before** activating retry closes that window. This task is a pure no-op today (behavior identical because retry is dead), proven by the existing suite staying green.

**Files (4):** `pkg/gmail/service.go`, `pkg/calendar/service.go`, `pkg/people/service.go`, `pkg/tasks/service.go`

**The 8 non-idempotent methods to unwrap** (replace the `retry.WithRetry(func() error { … return err }, 3, time.Second)` wrapper with a single direct `.Do()` call):

| File | Method | Line |
|------|--------|------|
| `pkg/gmail/service.go` | `SendMessage` | 141 |
| `pkg/gmail/service.go` | `CreateDraft` | 284 |
| `pkg/gmail/service.go` | `SendDraft` | 361 |
| `pkg/gmail/service.go` | `CreateLabel` | 481 |
| `pkg/calendar/service.go` | `CreateEvent` | 81 |
| `pkg/people/service.go` | `CreateContact` | 122 |
| `pkg/tasks/service.go` | `CreateTaskList` | 68 |
| `pkg/tasks/service.go` | `CreateTask` | 138 |

**Transform (apply to each of the 8).** Example — `gmail.SendMessage` currently:

```go
var sent *gmail.Message
err := retry.WithRetry(func() error {
	var err error
	sent, err = s.service.Users.Messages.Send("me", message).Do()
	return err
}, 3, time.Second)
if err != nil {
	return nil, fmt.Errorf("failed to send message: %w", err)
}
return sent, nil
```

becomes:

```go
// Sends are not retried: a retry after a request that succeeded but whose
// response was lost would deliver the message twice.
sent, err := s.service.Users.Messages.Send("me", message).Do()
if err != nil {
	return nil, fmt.Errorf("failed to send message: %w", err)
}
return sent, nil
```

Apply the identical shape to the other 7 (drop the closure + `retry.WithRetry(...)`, call `.Do()` once, keep the existing error wrap). Add the one-line comment explaining *why* on each.

**Step 1 (RED):** none — this is a behavior-preserving structural change (retry is currently dead, so unwrapping changes nothing observable). The guard test that proves sends don't retry lands in Task 1.2 (where retry is live and the test is meaningful).

**Step 2 (GREEN / verify):**
```bash
go build ./...
go vet ./...
go test ./pkg/gmail/... ./pkg/calendar/... ./pkg/people/... ./pkg/tasks/...
```
Expect: builds, all existing service tests pass. If any service no longer imports `retry` after unwrapping, remove the now-unused import (goimports / the compiler will tell you). If a service still uses `retry` elsewhere (gmail/calendar/people/tasks all retain idempotent calls), the import stays.

**Step 3 (commit):**
```
refactor(services): stop retrying non-idempotent send/create operations

Sends and creates must not be retried — a retry after a lost response
would duplicate the side effect. Precondition for fixing retry detection.
```

---

### Task 1.2 — Make retry actually fire (+ Retry-After, jitter, context) — **C2**

**Root cause (verified):** `retry.shouldRetry` does `err.(HTTPError)` (a direct type assertion for an interface with method `HTTPStatusCode() int`). Every service returns the raw `*googleapi.Error` from `.Do()`, which exposes a **field** `Code int` — not that method. So the assertion never matches and `WithRetry` runs the operation exactly once. Retry has never worked.

**Fix:** detect `*googleapi.Error` via `errors.As`, read `gErr.Code`; keep the `HTTPError` interface path for `RetryableError`/tests; honor `Retry-After`; add full jitter; add a context-aware variant. Keep the old `WithRetry(op, max, delay)` signature as a thin wrapper so the ~20 existing call sites and tests keep compiling (CLAUDE.md: ≤5 files/phase, green between).

**Verified facts the code relies on:** `google.golang.org/api@v0.268.0` `googleapi.Error` has `Code int` (always populated) and `Header http.Header`. `mockHTTPError` in the tests has pointer receivers, so `errors.As(err, &httpErr)` still catches it.

**Files (4):** `pkg/retry/retry.go`, `pkg/retry/retry_test.go`, `pkg/retry/retry_edge_test.go`, `pkg/gmail/service_retry_test.go` (new)

**Step 1 (RED) — add failing tests.**

Append to `pkg/retry/retry_test.go` (new imports needed: `"context"`, `"net/http"`, `"google.golang.org/api/googleapi"`):

```go
// A real *googleapi.Error (what the Google SDK returns) must be retried on 5xx.
func TestRetryOnGoogleAPIError5xx(t *testing.T) {
	attempts := 0
	op := func() error {
		attempts++
		if attempts < 3 {
			return &googleapi.Error{Code: http.StatusServiceUnavailable}
		}
		return nil
	}
	if err := WithRetry(op, 5, time.Millisecond); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// A *googleapi.Error 4xx (e.g. 403) must NOT be retried.
func TestNoRetryOnGoogleAPIError4xx(t *testing.T) {
	attempts := 0
	op := func() error {
		attempts++
		return &googleapi.Error{Code: http.StatusForbidden}
	}
	if err := WithRetry(op, 5, time.Millisecond); err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retry on 403), got %d", attempts)
	}
}

// A Retry-After header (integer seconds) is honored as the delay.
func TestRetryAfterHeaderHonored(t *testing.T) {
	retryAfter, ok := parseRetryAfter(http.Header{"Retry-After": []string{"2"}})
	_ = ok
	if retryAfter != 2*time.Second {
		t.Fatalf("expected 2s, got %v", retryAfter)
	}
	if d, _ := parseRetryAfterCheck(); d != 0 { // placeholder removed below
		t.Fatal("unreachable")
	}
}

// WithRetryCtx aborts between attempts when the context is cancelled.
func TestWithRetryCtxCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	op := func() error {
		attempts++
		if attempts == 1 {
			cancel() // cancel during the first backoff wait
		}
		return &googleapi.Error{Code: http.StatusServiceUnavailable}
	}
	err := WithRetryCtx(ctx, op, 10, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected context error")
	}
	if attempts > 2 {
		t.Fatalf("expected to stop early on cancel, got %d attempts", attempts)
	}
}
```

> Note: `parseRetryAfter` returns just `time.Duration` (see implementation). Write the test as:
> ```go
> func TestRetryAfterHeaderHonored(t *testing.T) {
> 	if got := parseRetryAfter(http.Header{"Retry-After": []string{"2"}}); got != 2*time.Second {
> 		t.Fatalf("expected 2s, got %v", got)
> 	}
> 	if got := parseRetryAfter(http.Header{"Retry-After": []string{"0"}}); got != 0 {
> 		t.Fatalf("expected 0 for non-positive, got %v", got)
> 	}
> 	if got := parseRetryAfter(nil); got != 0 {
> 		t.Fatalf("expected 0 for nil header, got %v", got)
> 	}
> }
> ```
> (Use this version; ignore the placeholder `parseRetryAfterCheck` line above.)

Create `pkg/gmail/service_retry_test.go` (the non-idempotent **guard** — fails if anyone re-wraps a send in retry once retry is live):

```go
// ABOUTME: Verifies non-idempotent Gmail operations are not retried.
// ABOUTME: A send that hit a 503 must reach the server exactly once, never duplicated.

package gmail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/harper/gsuite-mcp/pkg/auth"
)

func TestSendMessageNotRetriedOn503(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"message":"unavailable"}}`))
	}))
	defer srv.Close()

	t.Setenv("ISH_MODE", "true")
	t.Setenv("ISH_BASE_URL", srv.URL)

	ctx := context.Background()
	svc, err := NewService(ctx, auth.NewFakeClient("test@example.com"))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, _ = svc.SendMessage(ctx, "to@example.com", "subj", "body", "", "", "")

	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("send must hit the server exactly once (no retry), got %d", got)
	}
}
```

Run:
```bash
go test ./pkg/retry/... ./pkg/gmail/... -run 'GoogleAPI|RetryAfter|WithRetryCtx|NotRetried'
```
Expect: **compile failure** (`WithRetryCtx` and `parseRetryAfter` don't exist yet; `shouldRetry` returns one value). That's RED.

**Step 2 (GREEN) — rewrite `pkg/retry/retry.go`** in full:

```go
// ABOUTME: This file implements retry logic with exponential backoff for HTTP operations.
// ABOUTME: It retries idempotent calls on rate limits (429) and server errors (5xx), never on 4xx.

package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/api/googleapi"
)

// HTTPError is implemented by errors that carry an HTTP status code.
type HTTPError interface {
	error
	HTTPStatusCode() int
}

// WithRetry executes an IDEMPOTENT operation with exponential backoff.
//
// Only wrap idempotent calls (GET/LIST/UPDATE/DELETE). Operations that create or
// send resources must NOT be retried: a retry after a request that actually
// succeeded but whose response was lost would duplicate the side effect.
//
// It retries on 429 and 5xx, honors a server Retry-After header, adds full jitter,
// and gives up after maxRetries additional attempts.
//
// WithRetry delegates to WithRetryCtx with a background context. New call sites
// should prefer WithRetryCtx so an upstream cancellation stops the retry loop.
func WithRetry(operation func() error, maxRetries int, baseDelay time.Duration) error {
	return WithRetryCtx(context.Background(), operation, maxRetries, baseDelay)
}

// WithRetryCtx is WithRetry plus context cancellation: if ctx is cancelled while
// waiting between attempts, it stops and returns ctx.Err().
func WithRetryCtx(ctx context.Context, operation func() error, maxRetries int, baseDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == maxRetries {
			break
		}

		retryable, retryAfter := shouldRetry(err)
		if !retryable {
			return err
		}

		delay := backoffDelay(baseDelay, attempt, retryAfter)
		if delay <= 0 {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return lastErr
}

// backoffDelay returns the wait before the next attempt. A server-provided
// Retry-After wins; otherwise the delay is exponential (baseDelay * 2^attempt)
// with full jitter in [0, cap].
func backoffDelay(baseDelay time.Duration, attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	if baseDelay <= 0 {
		return 0
	}
	capped := baseDelay * time.Duration(1<<uint(attempt))
	if capped <= 0 { // overflow guard for absurd inputs
		return 0
	}
	return time.Duration(rand.Int63n(int64(capped) + 1))
}

// shouldRetry reports whether err is retryable and any Retry-After delay it carries.
func shouldRetry(err error) (bool, time.Duration) {
	statusCode, header, ok := httpStatus(err)
	if !ok {
		return false, 0
	}
	if statusCode == http.StatusTooManyRequests || (statusCode >= 500 && statusCode < 600) {
		return true, parseRetryAfter(header)
	}
	return false, 0
}

// httpStatus extracts an HTTP status (and response header, if any) from err,
// recognizing both *googleapi.Error (what the Google SDK returns) and HTTPError.
func httpStatus(err error) (int, http.Header, bool) {
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code, gErr.Header, true
	}
	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.HTTPStatusCode(), nil, true
	}
	return 0, nil, false
}

// parseRetryAfter reads a Retry-After header given in integer seconds. The
// HTTP-date form is intentionally unsupported; Google APIs send seconds.
func parseRetryAfter(header http.Header) time.Duration {
	if header == nil {
		return 0
	}
	secs, err := strconv.Atoi(header.Get("Retry-After"))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// RetryableError wraps an HTTP status code as an error.
type RetryableError struct {
	StatusCode int
	Message    string
}

func (e *RetryableError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("HTTP %d error", e.StatusCode)
}

func (e *RetryableError) HTTPStatusCode() int {
	return e.StatusCode
}

// NewRetryableError creates a new retryable error with the given status code.
func NewRetryableError(statusCode int, message string) *RetryableError {
	return &RetryableError{StatusCode: statusCode, Message: message}
}
```

**Step 2b (GREEN) — fix the two existing tests the change breaks:**

1. `pkg/retry/retry_edge_test.go:340` — `shouldRetry` now returns two values:
   ```go
   result, _ := shouldRetry(err)
   ```
2. `pkg/retry/retry_test.go:209-227` (`TestExponentialBackoff`) — jitter makes delays land in `[0, cap]`, not a tight band around `cap`. Replace the three band checks with upper-bound checks:
   ```go
   // Full jitter means each delay is in [0, cap]; assert the (jittered) upper bound.
   tolerance := 10 * time.Millisecond
   caps := []time.Duration{baseDelay, 2 * baseDelay, 4 * baseDelay} // 20ms, 40ms, 80ms
   for i, d := range delays {
   	if d < 0 || d > caps[i]+tolerance {
   		t.Errorf("delay %d = %v, want within [0, %v]", i, d, caps[i]+tolerance)
   	}
   }
   ```

**Step 3 (verify):**
```bash
go test ./pkg/retry/... -race
go test ./pkg/gmail/... -run NotRetried
go build ./... && go vet ./...
```
Expect: all green. The new tests prove retry now fires on real Google 5xx, skips 4xx, honors Retry-After, cancels on ctx, and that `SendMessage` hits the server exactly once.

**Step 4 (commit):**
```
fix(retry): detect googleapi.Error so retries actually fire

shouldRetry used a type assertion for an interface method the Google SDK
errors never implement, so WithRetry ran once and never retried. Detect
*googleapi.Error via errors.As, honor Retry-After, add full jitter, and add
WithRetryCtx for context-aware cancellation.
```

---

### Task 1.3 — Close the interactive-auth hole on the lazy-load path — **C3**

**Root cause (verified):** `resolveServices` (`server.go:134-177`) lazy-loads non-default accounts via `authenticator.GetClient(ctx)` (`:168`). `GetClient` falls through to `authenticate()` on a missing/expired token, which `fmt.Printf`s a URL to **stdout** and blocks on **stdin** (`oauth.go:161-166`). Under the MCP stdio transport that corrupts the JSON-RPC framing and hangs the server. The eager path already avoids this (`NewServer` uses `GetClientIfAuthenticated`, `:75`); this is the one remaining call to the interactive getter on a server request path.

**Fix:** use the non-interactive `GetClientIfAuthenticated` (already exists, `oauth.go:203`) and return a clear "not authenticated, use auth_init" error instead of prompting.

**Files (2):** `pkg/server/server.go`, `pkg/server/server_lazyload_test.go` (new)

**Step 1 (RED):** new test — lazy-loading an account whose token file is absent must return an error (not block on stdin), and must not panic:

```go
// ABOUTME: Verifies lazy account resolution never triggers interactive OAuth.
// ABOUTME: A missing token yields an auth_init hint, not a stdin prompt.

package server

import (
	"context"
	"strings"
	"testing"

	"github.com/harper/gsuite-mcp/pkg/accounts"
)

func TestResolveServicesUnauthedAccountReturnsError(t *testing.T) {
	s := &Server{
		registry: &accounts.Registry{
			DefaultAlias: "default",
			Accounts: map[string]*accounts.Account{
				"default": {Alias: "default"},
				"work":    {Alias: "work", TokenPath: "/nonexistent/path/token.json"},
			},
		},
		services: make(map[string]*AccountServices),
	}

	done := make(chan struct{})
	var err error
	go func() {
		_, err = s.resolveServices(context.Background(), "work")
		close(done)
	}()

	select {
	case <-done:
		if err == nil {
			t.Fatal("expected error for unauthenticated account, got nil")
		}
		if !strings.Contains(err.Error(), "auth_init") {
			t.Fatalf("error should point user to auth_init, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resolveServices blocked — it must not prompt on stdin")
	}
}
```
(Add `"time"` to imports.) Run `go test ./pkg/server/ -run UnauthedAccount` → **fails** (today it calls `GetClient` → would attempt interactive auth / hang, tripping the 2s guard).

**Step 2 (GREEN):** in `resolveServices`, change the lazy-load block (`server.go:158-174`) so the inner getter is non-interactive:

```go
client := acct.GetClient()
if client == nil {
	// Lazy-load: build an authenticated client from the account's stored token.
	// Never trigger interactive OAuth here — this runs on the MCP request path,
	// where prompting to stdout/stdin would corrupt the stdio transport.
	tokenPath := acct.TokenPath
	if tokenPath == "" {
		tokenPath = auth.GetTokenPath()
	}
	authenticator, authErr := auth.NewAuthenticator(auth.GetCredentialsPath(), tokenPath)
	if authErr != nil {
		return nil, fmt.Errorf("account '%s' is not authenticated: %w (use auth_init tool to authenticate)", acct.Alias, authErr)
	}
	authedClient, clientErr := authenticator.GetClientIfAuthenticated(ctx)
	if clientErr != nil {
		return nil, fmt.Errorf("account '%s' authentication failed: %w (token may be expired, use auth_init to re-authenticate)", acct.Alias, clientErr)
	}
	if authedClient == nil {
		return nil, fmt.Errorf("account '%s' is not authenticated (use auth_init tool to authenticate)", acct.Alias)
	}
	acct.SetClient(authedClient)
	client = authedClient
}
```
(Only two changes vs. current: `GetClient` → `GetClientIfAuthenticated`, plus the `authedClient == nil` guard. Everything else identical.)

**Step 3 (verify):**
```bash
go test ./pkg/server/... -race
go build ./... && go vet ./...
```

**Step 4 (commit):**
```
fix(server): never trigger interactive OAuth on the request path

resolveServices lazy-loaded accounts via GetClient, which prompts on
stdout/stdin and corrupts the MCP stdio transport. Use the non-interactive
GetClientIfAuthenticated and return an auth_init hint instead.
```

---

### Task 1.4 — Single source of truth for version — **C4**

**Root cause (verified):** `cmd/gsuite-mcp/main.go:20` is `const version = "1.4.2"`. `-X main.version` cannot patch a `const` (the linker silently no-ops), so `.goreleaser.yml:21`'s `-X main.version={{.Version}}` is ineffective and the `Makefile` injects nothing. Separately, the MCP server reports a hardcoded `"1.0.0"` (`server.go:120`), and the web site shows `1.4.1` in JSON-LD vs `1.4.2` on the badge (fixed in Phase 5).

**Fix:** make `version` a `var` (ldflag-patchable), default `"dev"`; propagate it to the MCP server via a package var set from `main`; inject version in the `Makefile`.

**Files (3):** `cmd/gsuite-mcp/main.go`, `pkg/server/server.go`, `Makefile`

**Step 1 (RED):** add `pkg/server/version_test.go`:
```go
// ABOUTME: Verifies the MCP server reports the propagated build version.
// ABOUTME: Guards against the hardcoded "1.0.0" regressing.

package server

import "testing"

func TestVersionDefault(t *testing.T) {
	if Version == "" {
		t.Fatal("server.Version must have a non-empty default")
	}
}
```
And in `cmd/gsuite-mcp`, add `main_version_test.go`:
```go
package main

import "testing"

func TestVersionIsVar(t *testing.T) {
	// Compiles only if version is an addressable var (ldflag target), not a const.
	_ = &version
}
```
Run `go test ./cmd/... ./pkg/server/...` → `&version` fails to compile against a `const` (RED), and `server.Version` doesn't exist yet (RED).

**Step 2 (GREEN):**
- `cmd/gsuite-mcp/main.go:20`: `const version = "1.4.2"` → `var version = "1.4.2"`.
- `pkg/server/server.go`: add near the top of the file (package level):
  ```go
  // Version is the build version reported to MCP clients. Set from main at startup.
  var Version = "dev"
  ```
  and change `server.NewMCPServer("gsuite-mcp", "1.0.0")` (`:118-121`) to `server.NewMCPServer("gsuite-mcp", Version)`.
- In `main`, where the `mcp` command boots the server (the case that calls `server.NewServer`), set `server.Version = version` **before** constructing the server.
- `Makefile:9`: keep `LDFLAGS` but add version injection:
  ```make
  VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
  LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
  ```

**Step 3 (verify):**
```bash
go test ./cmd/... ./pkg/server/...
make build && ./gsuite-mcp version    # prints the git-described version, not a constant
go vet ./...
```

**Step 4 (commit):**
```
fix(version): make version a var and inject it at build time

const version cannot be patched by -X ldflags, so goreleaser's injection
was a no-op and the MCP server reported a hardcoded 1.0.0. Use a var,
propagate to the server, and inject VERSION in the Makefile.
```

---

## Phase 2 — IMPORTANT (Go code: robustness & security)

### Task 2.1 — Enable panic recovery on the MCP server

**Verified:** `server.NewMCPServer("gsuite-mcp", …)` (`server.go:118`) is constructed without `server.WithRecovery()`. A panic in any tool handler crashes the whole stdio process and drops the client connection.

**Files (1-2):** `pkg/server/server.go` (+ optional `server_recovery_test.go`)

- GREEN: add the option:
  ```go
  mcpServer := server.NewMCPServer(
  	"gsuite-mcp",
  	Version,
  	server.WithRecovery(),
  )
  ```
- Verify the option exists in the pinned `mark3labs/mcp-go v0.43.2` (`go doc github.com/mark3labs/mcp-go/server.WithRecovery`). If the symbol differs, use the equivalent recovery middleware that version exposes.
- Verify: `go build ./... && go test ./pkg/server/...`.
- Commit: `fix(server): recover from tool-handler panics instead of crashing`

### Task 2.2 — Drop the masked token fragment from `auth_info` (was C5)

**Verified:** `AuthInfoResponse.AccessToken` (`server.go:2016`) is set from `info.AccessToken` (`:2053`), which is `maskToken(...)` (`oauth.go:277`). It's masked, but a token fragment has no diagnostic value and needlessly enters model context.

**Files (2):** `pkg/server/server.go`, `pkg/auth/oauth.go`

- Remove `AccessToken` from `AuthInfoResponse` (`:2016`) and stop setting it (`:2053`).
- Remove `AccessToken` from `TokenInfo` (`oauth.go:261`) and its assignment (`:277`); delete `maskToken` (`:291`) if it becomes unused (the compiler/linter will confirm).
- Update any test asserting `access_token` in `auth_info` output.
- Verify: `go build ./... && go vet ./... && go test ./pkg/server/... ./pkg/auth/...`
- Commit: `fix(auth): stop returning a token fragment from auth_info`

### Task 2.3 — Drop filesystem token paths from `accounts_list`

**Verified:** `AccountInfo.TokenPath` (`server.go:2249`) is populated (`:2266`), exposing absolute local paths (often containing the OS username) to the model. The boolean `Authenticated` already conveys what the user needs.

**Files (1-2):** `pkg/server/server.go` (+ test)

- Remove the `TokenPath` field from `AccountInfo` (`:2249`) and its assignment (`:2266`). Keep `Alias`, `IsDefault`, `Authenticated`.
- Update the `accounts_list` test to assert `token_path` is absent.
- Commit: `fix(accounts): stop exposing token file paths via accounts_list`

### Task 2.4 — Registry encapsulation + prove race-freedom in CI

**Verified:** `Registry.DefaultAlias` and `Registry.Accounts` are exported (`registry.go:61-66`) and read **without** the lock at `server.go:2086` and `:2264`. Today nothing mutates `DefaultAlias` after load and the map mutators (`AddAccount`) are lock-guarded, so this is an **encapsulation/latent-race** issue, not an active data race — but `auth_init` calling `AddAccount` concurrently with iteration is exactly the kind of thing that bites later. Lock it down and let `-race` prove it.

**Files (3):** `pkg/accounts/registry.go`, `pkg/server/server.go`, `Makefile`

- Add accessors to `registry.go`:
  ```go
  // Default returns the default account alias under the read lock.
  func (r *Registry) Default() string {
  	r.mu.RLock()
  	defer r.mu.RUnlock()
  	return r.DefaultAlias
  }
  ```
- Replace the two unlocked reads:
  - `server.go:2086`: `resolvedAlias = s.registry.DefaultAlias` → `resolvedAlias = s.registry.Default()`
  - `server.go:2264`: `IsDefault: alias == s.registry.DefaultAlias` → `IsDefault: alias == s.registry.Default()`
- Add `-race` to the Makefile test targets:
  ```make
  test:
  	$(GO) test -race -v ./...
  ```
- Verify: `go test -race ./...` green.
- (Unexporting the fields themselves is deferred — the ISH constructor in `server.go:60-65` and tests build `Registry` with named exported fields; unexporting would ripple. The accessor + `-race` gives the safety win now; full unexport can be a follow-up if desired.)
- Commit: `fix(accounts): read DefaultAlias under lock; run tests with -race`

### Task 2.5 — Per-request timeout / context deadline on outbound API calls

**Verified:** services build the Google SDK client from the OAuth `*http.Client` with no timeout; `WithRetryCtx` now respects ctx but nothing bounds a single hung request.

**Files (1-2):** `pkg/server/server.go` (where handlers obtain `ctx`) or a small helper.

- Wrap each tool handler's `ctx` with a sane deadline (e.g. `context.WithTimeout(ctx, 30*time.Second)`) before calling services, so a stuck API call can't hang the handler indefinitely. Apply at the handler boundary (one helper used by handlers) to keep it DRY.
- Add a test that a context already past its deadline causes a service call to return promptly.
- Commit: `fix(server): bound outbound API calls with a request timeout`

> If threading a deadline through every handler exceeds 5 files, split into per-domain sub-tasks (gmail handlers, calendar handlers, …), each its own commit.

### Task 2.6 — Sanitize Google API errors at the tool boundary

**Verified:** services do `fmt.Errorf("failed to …: %w", err)` and the raw `*googleapi.Error` (including server `Body`) propagates to the tool result. That can surface internal detail/PII to the model.

**Files (1-2):** a small error-mapping helper in `pkg/server` + handler use.

- Add a helper that maps a `*googleapi.Error` to a concise, safe message (status class + actionable hint), logging the full error server-side (stderr) only.
- Apply in the tool-result error path.
- Test: a `*googleapi.Error{Code:403, Body:"secret"}` produces a result string that does **not** contain `"secret"`.
- Commit: `fix(server): sanitize upstream API errors before returning to client`

### Task 2.7 — Make `auth_revoke` actually revoke (or rename it honestly)

**Verified:** `Authenticator.RevokeToken` (`oauth.go:187`) only `os.Remove`s the local token file — the refresh token stays valid at Google. The tool name/description implies real revocation.

**Files (2-3):** `pkg/auth/oauth.go`, `pkg/server/server.go`, test.

- Add a real revoke: POST the token to `https://oauth2.googleapis.com/revoke?token=<refresh_or_access>` (best-effort, with timeout), then delete the local file. On network failure, delete locally and return a message telling the user to also revoke at <https://myaccount.google.com/permissions>.
- Update the `auth_revoke` tool description to state both effects.
- Test the URL/params construction and the local-delete fallback (use an `httptest.Server` for the revoke endpoint).
- Commit: `fix(auth): revoke token at Google's endpoint, not just locally`

---

## Phase 3 — IMPORTANT (tool surface & prompts)

Concrete edits; each is its own small commit. Add/adjust a test only where behavior (not just description text) changes.

1. **Destructive-tool clarity.** In tool descriptions for `gmail_delete_message`, `calendar_delete_event`, `people_delete_contact`, `tasks_delete_task`, `tasks_delete_tasklist`, make irreversibility explicit and point to the safe alternative where one exists (e.g. `gmail_trash_message`). Source of truth is the Go registration in `pkg/server`. Verify the skill's `skills/gsuite-mcp/` guidance matches.
2. **`schedule_meeting` hardcodes `America/Chicago`.** Find the prompt definition in `pkg/server` and make the timezone a parameter (default to the caller's offset or UTC), not a literal. Test the prompt renders the provided tz.
3. **Resource PII projection.** The `gsuite://gmail/unread`, `contacts/recent`, etc. resources dump full payloads into model context. Trim each resource projection to the minimal useful fields (subject/from/snippet; name/email). One commit per resource group; test the projected JSON shape.
4. **Response-shape consistency.** Audit tool results for nil-slice vs `[]` (JSON `null` vs `[]`). Normalize list-returning tools to always emit `[]`. Test one representative per service.
5. **`gmail_manage_labels` action enum.** Ensure the `action` parameter advertises its allowed values (`list`/`get`/`create`/`update`/`delete`) in the schema so clients can validate.

> If you'd rather, split Phase 3 into its own plan — it's independent of Phases 1–2.

---

## Phase 4 — IMPORTANT (CI / release hardening)

Verification here is "the workflow runs green" / tooling output, not Go unit tests.

1. **Pin GitHub Actions to commit SHAs.** In every `.github/workflows/*.yml`, replace `uses: org/action@vN` with `uses: org/action@<full-sha>  # vN`. Verify with a CI run.
2. **`-race` + coverage in CI.** Ensure the test job runs `go test -race ./...` (matches Task 2.4) and uploads coverage.
3. **Release integrity.** In `.goreleaser.yml`, add a `checksums:` block and (optionally) cosign signing + an SBOM (`sboms:`). Verify with `goreleaser release --snapshot --clean`.
4. **Dependabot.** Add `.github/dependabot.yml` for `gomod` and `github-actions` ecosystems (weekly).
5. **Least-privilege workflow permissions.** Add a top-level `permissions: contents: read` to each workflow; elevate per-job only where needed (e.g. `contents: write` on release).
6. **`go.mod`/`go.sum` hygiene.** Run `go mod tidy`; commit the result (`chore: sync go.mod/go.sum`). (Resolves the long-standing dirty `go.sum`.)

---

## Phase 5 — IMPORTANT (docs & web site)

1. **Version consistency on the site.** Fix the JSON-LD `softwareVersion` (`1.4.1`) to match the release/badge. Grep `docs/` for `1.4.1`/`1.4.2` and reconcile to one value sourced from the latest tag.
2. **Stale marketing docs.** Reconcile `product-description.md`, `docs/brief.md`, and any `blog-post.md` with the current 34-tool / multi-account / Tasks reality (the docs reviewer flagged drift). Update counts and feature lists.
3. **Web a11y/perf (web reviewers):**
   - Color contrast: the flagged `2.8:1` pair fails WCAG AA — adjust to ≥ 4.5:1. (`docs/index.html` / CSS.)
   - Add a favicon (`docs/favicon.ico` + `<link rel="icon">`).
   - Resize the 5.4 MB OpenGraph image to a < 300 KB optimized asset; confirm `og:image`/`twitter:image` point at it.
   - Add a real `404.html` content check and ensure GitHub Pages serves it.
4. **Real email scrub.** Replace the literal address in `docs/index.html:881` (and `README.md:88`) with a non-personal contact or a role address. (Also a MINOR item — do it here for the site.)

---

## Phase 6 — IMPORTANT (test backfill)

The panel flagged hollow coverage (tasks ~12.4%, auth handlers ~0%) and vacuous tests (the `test/scenario_test.go` cases `t.Logf` on error and assert almost nothing).

1. **`pkg/tasks` service tests.** Cover `CreateTask`/`CreateTaskList`/`Update`/`Delete`/`List` against an `httptest` server (success + 4xx + 5xx). Target meaningful coverage, not a number.
2. **Auth tool handler tests.** Cover `auth_status`/`auth_info`/`auth_init`/`auth_complete`/`auth_revoke` handlers in ISH mode and with a fake token file.
3. **De-vacuous the scenario tests.** Where `test/scenario_test.go` currently logs-and-continues on error, assert the expected outcome against the ish server (so a regression fails the build). Do **not** delete the tests — strengthen them.
4. **Coverage gate (optional).** Add a CI step failing if total coverage drops below an agreed floor.

---

## Phase 7 — MINOR (backlog quick wins)

Each a one-line commit; batch a few per PR.

1. Scrub the real email from `README.md:88` (if not already done in Phase 5).
2. De-duplicate the `ISH_MODE == "true"` magic string into one `auth.IshModeEnabled()` helper used by all four services + server.
3. CLI polish: non-zero exit codes on error; usage/errors to **stderr**, data to **stdout**; `--help` exit 0.
4. Audit remaining `fmt.Print*` calls outside the OAuth flow to ensure none write to stdout under the `mcp` command.
5. Ensure every Go file has the two `// ABOUTME:` header lines (CLAUDE.md) — grep for files missing them.

---

## Task → finding traceability

| Task | Finding | Severity |
|------|---------|----------|
| 1.1 + 1.2 | Retry dead + latent double-send (C2; Reliability vs Go-Quality contradiction, adjudicated) | CRITICAL |
| 1.3 | Interactive OAuth on request path (C3, narrowed) | CRITICAL |
| 1.4 | `const version` un-patchable + MCP `1.0.0` (C4) | CRITICAL |
| 2.1 | No `WithRecovery()` → panic crashes server | IMPORTANT |
| 2.2 | `auth_info` masked-token fragment (was C5) | IMPORTANT |
| 2.3 | `accounts_list` leaks token paths | IMPORTANT |
| 2.4 | Registry exported fields / unlocked reads | IMPORTANT |
| 2.5 | No request timeout on API calls | IMPORTANT |
| 2.6 | Raw Google errors → client/model | IMPORTANT |
| 2.7 | `auth_revoke` local-only / misleading | IMPORTANT |
| 3.* | Destructive-tool clarity, tz hardcode, resource PII, response shape, label enum | IMPORTANT |
| 4.* | Action SHA-pin, `-race`, checksums/signing/SBOM, Dependabot, permissions, go.sum | IMPORTANT |
| 5.* | Site version, stale marketing docs, contrast/favicon/OG/404, email scrub | IMPORTANT |
| 6.* | tasks/auth coverage, vacuous scenario tests | IMPORTANT |
| 7.* | email scrub, ISH_MODE dedup, exit codes/stderr, ABOUTME headers | MINOR |

## Self-review (writing-plans)

- **No placeholders:** all Phase 1–2 code is complete and compile-checked against the real signatures (`googleapi.Error.Code/.Header`, `GetClientIfAuthenticated`, `server.WithRecovery`, the existing retry tests). Phases 3–7 give exact files + edits + verification.
- **Ordering safety:** Task 1.1 (gate) precedes Task 1.2 (activate retry) so the double-send window never opens; every task keeps the repo green and touches ≤ 5 files.
- **TDD:** Phase 1–2 tasks are RED→GREEN with named tests and expected failure reasons.
- **Independence:** Phases 3–7 are separable into their own PRs/plans.
- **Open items for Doctor Biz:** rotate client secret + revoke refresh token (Phase 0); confirm whether to fully unexport registry fields (Task 2.4 note); confirm coverage floor (Phase 6.4).
