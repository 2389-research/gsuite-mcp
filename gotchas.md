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
