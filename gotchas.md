# Gotchas

Hard-won lessons. Review at session start before new work.

## Subagents share the working directory — they can move shared git HEAD

**2026-06-03.** During the Phase-1 finishing step, `git rev-parse --abbrev-ref HEAD`
unexpectedly reported `main` instead of the feature branch. A review subagent,
dispatched to compare old vs. new code, had run `git checkout 7f79344` then
`git checkout main` in the shared working directory. Subagents launched without
worktree isolation operate on the **same** checkout, so a `git checkout`/`switch`/
`stash`/`reset`/`restore`/`rebase` inside one of them mutates the controller's HEAD
and index. No commits were lost (the branch ref was intact), but the controller was
left standing on `main`.

**Rule:** When dispatching a subagent that will run git in a shared (non-worktree)
checkout, either
1. explicitly forbid state-changing git commands (checkout, switch, stash, reset,
   restore, rebase) — read-only `git diff/show/log/status` only, comparisons via
   revision ranges like `git diff A..B` (never check a ref out), **or**
2. give the subagent `isolation: "worktree"` so its git state is its own.

**Verify after review fan-out:** before any merge/PR/push, re-check
`git rev-parse --abbrev-ref HEAD` and `git status` — a reviewer may have moved you.

## Never put a secret in a URL query string — it leaks via `*url.Error` and access logs

**2026-06-05.** Task 2.7 first implemented OAuth token revocation by appending the
token to the revoke endpoint's URL query string (`?token=<secret>`) and POSTing with
a `nil` body. Two reviewers independently flagged it. Three distinct leak vectors:

1. **`*url.Error` carries the full URL.** When `http.Client.Do` hits a transport
   failure, it returns a `*url.Error` whose `.Error()` string embeds the **entire
   URL, query string included** — so the token lands in any log line that prints the
   error. We log `err` on the failure path, so the secret would have leaked there.
2. **Server-side access logs.** Google (and every proxy in between) logs request
   URLs; a query-string token is recorded in their access logs by design.
3. **Dishonest framing.** Sending `Content-Type: application/x-www-form-urlencoded`
   with a `nil` body is a contradiction — the form data belongs in the body.

**Fix (`fa06bb5`):** send the secret in the **form-encoded request body** —
`strings.NewReader(url.Values{"token": {value}}.Encode())` with the
`application/x-www-form-urlencoded` header (RFC 7009 form). Error strings then
contain only the bare endpoint URL, never the token.

**Rule:** secrets (tokens, passwords, API keys) go in the request **body** or a
header that you never log — **never** in the URL/query string. Before logging any
error from an HTTP call, remember `*url.Error.Error()` includes the request URL;
keep secrets out of that URL so the error is safe to log.
