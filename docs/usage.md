# GSuite MCP Server Usage Guide

This guide covers all 4 tools available in the GSuite MCP Server (Go implementation).

## Available Tools

### Gmail Tools (2 tools)

#### gmail_list_messages

List Gmail messages with optional filters.

**Parameters:**
- `query` (string, optional): Gmail search query
- `max_results` (integer, optional): Maximum number of messages to return (default: 100)

**Example:**
```json
{
  "tool": "gmail_list_messages",
  "arguments": {
    "query": "is:unread from:example@test.com",
    "max_results": 50
  }
}
```

**Gmail Query Syntax:**
- `is:unread` - Unread messages
- `is:read` - Read messages
- `is:starred` - Starred messages
- `from:email@example.com` - From specific sender
- `to:email@example.com` - To specific recipient
- `subject:meeting` - Subject contains "meeting"
- `after:2025/12/01` - After date
- `before:2025/12/31` - Before date
- `has:attachment` - Has attachments
- `label:important` - Has specific label
- `in:inbox` - In inbox
- `in:trash` - In trash

**Returns:**
```json
[
  {
    "id": "18c1a2b3d4e5f6g7",
    "threadId": "18c1a2b3d4e5f6g7",
    "labelIds": ["INBOX", "UNREAD"]
  }
]
```

List of message objects with `id`, `threadId`, and `labelIds` fields.

---

#### gmail_send_message

Send an email.

**Parameters:**
- `to` (string, required): Recipient email address
- `subject` (string, required): Email subject
- `body` (string, required): Email body (plain text)

**Example:**
```json
{
  "tool": "gmail_send_message",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "Hello from GSuite MCP",
    "body": "This is a test message from the GSuite MCP server."
  }
}
```

**Returns:**
```json
{
  "id": "18c1a2b3d4e5f6g8",
  "threadId": "18c1a2b3d4e5f6g8",
  "labelIds": ["SENT"]
}
```

Sent message object with `id`, `threadId`, and `labelIds`.

---

### Calendar Tools (1 tool)

#### calendar_list_events

List upcoming calendar events.

**Parameters:**
- `max_results` (integer, optional): Maximum events to return (default: 100)
- `time_min` (string, optional): RFC3339 timestamp for earliest event (default: now)
- `time_max` (string, optional): RFC3339 timestamp for latest event

**Example (List Next 10 Events):**
```json
{
  "tool": "calendar_list_events",
  "arguments": {
    "max_results": 10
  }
}
```

**Example (Events in Date Range):**
```json
{
  "tool": "calendar_list_events",
  "arguments": {
    "max_results": 50,
    "time_min": "2025-12-01T00:00:00Z",
    "time_max": "2025-12-31T23:59:59Z"
  }
}
```

**Returns:**
```json
[
  {
    "id": "abc123def456",
    "summary": "Team Meeting",
    "start": {
      "dateTime": "2025-12-05T10:00:00Z"
    },
    "end": {
      "dateTime": "2025-12-05T11:00:00Z"
    },
    "attendees": [
      {"email": "alice@example.com"},
      {"email": "bob@example.com"}
    ]
  }
]
```

List of event objects with details like `id`, `summary`, `start`, `end`, and `attendees`.

---

### People (Contacts) Tools (1 tool)

#### people_list_contacts

List all contacts.

**Parameters:**
- `page_size` (integer, optional): Number of contacts to return (default: 100)

**Example:**
```json
{
  "tool": "people_list_contacts",
  "arguments": {
    "page_size": 50
  }
}
```

**Returns:**
```json
[
  {
    "resourceName": "people/c123456789",
    "names": [
      {
        "displayName": "Jane Doe",
        "givenName": "Jane",
        "familyName": "Doe"
      }
    ],
    "emailAddresses": [
      {
        "value": "jane.doe@example.com"
      }
    ]
  }
]
```

List of contact objects with names and email addresses.

---

## Common Workflows

### Email Management

**Check unread emails:**
```json
{
  "tool": "gmail_list_messages",
  "arguments": {
    "query": "is:unread",
    "max_results": 10
  }
}
```

**Send a quick email:**
```json
{
  "tool": "gmail_send_message",
  "arguments": {
    "to": "colleague@example.com",
    "subject": "Quick Question",
    "body": "Hi! Do you have a moment to discuss the project?"
  }
}
```

**Find emails from a specific sender:**
```json
{
  "tool": "gmail_list_messages",
  "arguments": {
    "query": "from:manager@example.com is:unread"
  }
}
```

---

### Calendar Management

**Find next meeting:**
```json
{
  "tool": "calendar_list_events",
  "arguments": {
    "max_results": 1
  }
}
```

**List this week's events:**
```json
{
  "tool": "calendar_list_events",
  "arguments": {
    "max_results": 50,
    "time_min": "2025-12-01T00:00:00Z",
    "time_max": "2025-12-07T23:59:59Z"
  }
}
```

---

### Contact Management

**List all contacts:**
```json
{
  "tool": "people_list_contacts",
  "arguments": {
    "page_size": 100
  }
}
```

**Find a contact's email:**
```json
{
  "tool": "people_list_contacts",
  "arguments": {
    "page_size": 1000
  }
}
```
Then search the results for the contact by name.

---

## Best Practices

### Email
1. **Use specific queries** - Narrow down message searches with precise Gmail query syntax
2. **Limit results** - Use `max_results` to avoid overwhelming responses
3. **Plain text only** - Current implementation supports plain text email bodies
4. **Single recipient** - Send to one recipient at a time (no CC/BCC in current version)

### Calendar
1. **Time zones** - Always use ISO 8601 format with timezone (Z for UTC)
2. **Narrow date ranges** - Use `time_min` and `time_max` for specific periods
3. **Limit results** - Set `max_results` to reasonable values (10-50 for most use cases)

### Contacts
1. **Pagination** - Use `page_size` to control result size
2. **Post-processing** - Filter and search results in your application logic
3. **Cache when possible** - Contacts don't change frequently

### General
1. **Check for errors** - All tools return error information if operations fail
2. **Rate limits** - Server handles retries automatically, but avoid excessive calls
3. **Test with ish mode** - Use ish mode for development and testing without affecting real data

---

## Error Handling

All tools return errors in this format:

```json
{
  "error": "Error message describing what went wrong"
}
```

### Common Errors

**404 - Not Found**
- Resource doesn't exist (invalid message ID, event ID, etc.)
- Check the ID and try again

**429 - Rate Limit Exceeded**
- Too many requests in a short time
- Server automatically retries with exponential backoff
- If persistent, wait and reduce request frequency

**403 - Permission Denied**
- Missing OAuth scopes
- Check OAuth consent screen configuration
- Re-authenticate with correct scopes

**400 - Bad Request**
- Invalid parameters
- Check required fields and data formats
- Verify ISO 8601 datetime format for calendar events

**500 - Server Error**
- Google API server error
- Server automatically retries
- If persistent, check Google API status

---

## Response Formats

All successful tool calls return JSON data.

### Gmail Message Object
```json
{
  "id": "string",
  "threadId": "string",
  "labelIds": ["string"]
}
```

### Calendar Event Object
```json
{
  "id": "string",
  "summary": "string",
  "start": {
    "dateTime": "RFC3339 timestamp"
  },
  "end": {
    "dateTime": "RFC3339 timestamp"
  },
  "attendees": [
    {"email": "string"}
  ]
}
```

### Contact Object
```json
{
  "resourceName": "string",
  "names": [
    {
      "displayName": "string",
      "givenName": "string",
      "familyName": "string"
    }
  ],
  "emailAddresses": [
    {"value": "string"}
  ]
}
```

---

## Testing with Ish Mode

For testing without affecting real Google data, use ish mode:

```bash
# Set up ish mode
export ISH_MODE=true
export ISH_BASE_URL=http://localhost:9000
export ISH_USER=testuser

# Run server
./gsuite-mcp
```

All tools work identically in ish mode. See [setup.md](setup.md) for ish mode configuration details.

---

## Future Tools (Planned)

The following tools are planned for future releases:

### Gmail
- `gmail_get_message` - Get full message content
- `gmail_modify_message` - Modify labels (mark read/unread, star, archive)
- `gmail_reply_to_message` - Reply to existing threads
- `gmail_create_draft` - Create draft emails
- `gmail_list_labels` - List all labels
- `gmail_create_label` - Create new labels

### Calendar
- `calendar_create_event` - Create new events
- `calendar_update_event` - Update existing events
- `calendar_delete_event` - Delete events

### People
- `people_search_contacts` - Search contacts
- `people_create_contact` - Create new contacts
- `people_update_contact` - Update existing contacts
- `people_delete_contact` - Delete contacts

See the project roadmap for implementation timeline.
