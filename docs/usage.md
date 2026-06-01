# GSuite MCP Server Usage Guide

The server exposes **34 tools** across Gmail, Calendar, Contacts (People), and Tasks,
plus authentication and account management. This guide is the map: what exists and how
the pieces fit. Each tool's exact parameters are described in its own MCP schema, which
your client displays — that schema is the source of truth, so this guide does not
restate every field.

For the judgment that the schemas don't capture (which calls are destructive, which are
cheaper, the traps), use the bundled **gsuite-mcp skill** (`skills/gsuite-mcp/`).

## The `account` parameter

Every tool except `accounts_list` accepts an optional `account` (e.g. `"work"`,
`"personal"`). Omit it to use the default account. When a request targets a named
account, pass the same `account` to every call in that request. See
[Multiple accounts](../README.md#multiple-accounts).

## Tools

### Gmail (9)

| Tool | Purpose |
|------|---------|
| `gmail_list_messages` | Search/list messages (Gmail query syntax; `hydrate:true` for full headers, otherwise IDs only) |
| `gmail_get_message` | Full content of one message |
| `gmail_send_message` | Send immediately (`to`/`cc`/`bcc`, `in_reply_to` for threading) |
| `gmail_create_draft` | Create a draft (same fields; does not send) |
| `gmail_send_draft` | Send an existing draft |
| `gmail_modify_labels` | Add/remove labels on a message (by label **ID**) |
| `gmail_trash_message` | Move to Trash (recoverable) — the safe default |
| `gmail_delete_message` | Permanently delete (irreversible) |
| `gmail_manage_labels` | CRUD on label definitions (`list`/`get`/`create`/`update`/`delete`) |

### Calendar (5)

| Tool | Purpose |
|------|---------|
| `calendar_list_events` | Events in a window (`time_min`/`time_max`, RFC3339) |
| `calendar_get_event` | One event's full detail |
| `calendar_create_event` | Create an event (`summary`, `start_time`, `end_time`; `attendees`; `send_notifications` defaults true) |
| `calendar_update_event` | Update an event — `attendees` **replaces** the guest list; use `add_attendees`/`remove_attendees` to change individuals |
| `calendar_delete_event` | Cancel/delete an event (permanent) |

### Contacts — People (6)

| Tool | Purpose |
|------|---------|
| `people_search_contacts` | Find a contact by name/email/phone — the right way to look someone up |
| `people_list_contacts` | List/browse all contacts |
| `people_get_contact` | One contact's detail (`resource_name`, e.g. `people/c123`) |
| `people_create_contact` | Create a contact (`given_name` required) |
| `people_update_contact` | Update a contact |
| `people_delete_contact` | Delete a contact (permanent) |

### Tasks (8)

| Tool | Purpose |
|------|---------|
| `tasks_list_tasklists` | List task lists |
| `tasks_create_tasklist` | Create a task list |
| `tasks_update_tasklist` | Rename a task list |
| `tasks_delete_tasklist` | Delete a task list and its tasks (permanent) |
| `tasks_list_tasks` | List tasks (`tasklist_id` defaults to `@default`; `show_completed`, `due_min`/`due_max`) |
| `tasks_create_task` | Create a task (`title`; `due` RFC3339; `parent` for a subtask) |
| `tasks_update_task` | Update a task — complete it with `status:"completed"`, not delete |
| `tasks_delete_task` | Delete a task (permanent) |

### Authentication & accounts (6)

| Tool | Purpose |
|------|---------|
| `auth_status` | Check auth by making a live API call |
| `auth_info` | Auth metadata (no API call) |
| `auth_init` | Start OAuth; returns an `auth_url` for the user to visit |
| `auth_complete` | Finish OAuth with the full redirect URL from the browser |
| `auth_revoke` | Revoke the stored token |
| `accounts_list` | List configured accounts and their auth status (takes no `account`) |

## Live resources

For read-only "what's on / what's new" context, the server exposes pre-built MCP
resources (read them instead of spending a tool call):

```
gsuite://calendar/today        gsuite://gmail/unread            gsuite://tasks/today
gsuite://calendar/this-week    gsuite://gmail/unread/important  gsuite://tasks/upcoming
gsuite://calendar/upcoming     gsuite://gmail/drafts            gsuite://tasks/overdue
gsuite://calendar/availability gsuite://contacts/recent
```

## Built-in prompts

The server ships ready-made prompts (slash commands in clients that support them) for
common workflows: `email_triage`, `compose_email`, `email_reply`, `schedule_meeting`,
`calendar_summary`, `follow_up_reminder`, `find_contact`, `add_contact_from_email`,
`task_review`, `plan_tasks`.

## Examples

**Summarize unread mail** (one call — `hydrate` returns headers, not just IDs):

```json
{ "tool": "gmail_list_messages",
  "arguments": { "query": "is:unread", "max_results": 10, "hydrate": true } }
```

**Create an event and invite someone:**

```json
{ "tool": "calendar_create_event",
  "arguments": { "summary": "Project sync", "start_time": "2026-06-02T14:00:00-05:00",
                 "end_time": "2026-06-02T14:30:00-05:00", "attendees": ["jane@example.com"] } }
```

## Errors & retries

Tools return an error string when an operation fails. The server automatically retries
transient failures (HTTP 429 and 5xx) with exponential backoff — don't hand-retry those.
Authentication errors won't fix themselves; resolve auth first (`auth_status`, then
`auth_init`/`auth_complete`).

## Testing

For development without touching real Google data, run in ish mode — see
[ISH Mode](ISH_MODE.md).
