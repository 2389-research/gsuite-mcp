# GSuite MCP Server Usage Guide

This guide covers all 17 tools available in the GSuite MCP Server.

## Available Tools

### Gmail Tools (9 tools)

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
List of message objects with `id`, `threadId`, and `snippet`.

---

#### gmail_get_message

Get a specific Gmail message by ID with full content.

**Parameters:**
- `message_id` (string, required): The message ID

**Example:**
```json
{
  "tool": "gmail_get_message",
  "arguments": {
    "message_id": "18c1a2b3d4e5f6g7"
  }
}
```

**Returns:**
Complete message object including headers, body, and attachments.

---

#### gmail_send_message

Send an email.

**Parameters:**
- `to` (string, required): Recipient email address
- `subject` (string, required): Email subject
- `body` (string, required): Email body
- `cc` (string, optional): CC recipients
- `html` (boolean, optional): Whether body is HTML (default: false)

**Example (Plain Text):**
```json
{
  "tool": "gmail_send_message",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "Hello",
    "body": "This is a plain text message."
  }
}
```

**Example (HTML):**
```json
{
  "tool": "gmail_send_message",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "HTML Email",
    "body": "<h1>Hello</h1><p>This is <strong>HTML</strong>.</p>",
    "html": true
  }
}
```

**Example (With CC):**
```json
{
  "tool": "gmail_send_message",
  "arguments": {
    "to": "recipient@example.com",
    "cc": "cc@example.com",
    "subject": "Team Update",
    "body": "Message with CC recipient."
  }
}
```

**Returns:**
Sent message object with `id` and `threadId`.

---

#### gmail_reply_to_message

Reply to an existing email thread.

**Parameters:**
- `message_id` (string, required): ID of message to reply to
- `body` (string, required): Reply body
- `html` (boolean, optional): Whether body is HTML (default: false)

**Example:**
```json
{
  "tool": "gmail_reply_to_message",
  "arguments": {
    "message_id": "18c1a2b3d4e5f6g7",
    "body": "Thanks for your message. I'll review this today."
  }
}
```

**Returns:**
Reply message object with same `threadId` as original.

---

#### gmail_list_labels

List all Gmail labels.

**Parameters:**
None

**Example:**
```json
{
  "tool": "gmail_list_labels",
  "arguments": {}
}
```

**Returns:**
List of label objects with `id`, `name`, and `type`.

---

#### gmail_create_label

Create a new Gmail label.

**Parameters:**
- `name` (string, required): Label name

**Example:**
```json
{
  "tool": "gmail_create_label",
  "arguments": {
    "name": "ProjectX"
  }
}
```

**Returns:**
Created label object with `id` and `name`.

---

#### gmail_modify_message

Modify labels on a message (mark read/unread, star, archive, etc.).

**Parameters:**
- `message_id` (string, required): Message ID
- `add_labels` (array of strings, optional): Labels to add
- `remove_labels` (array of strings, optional): Labels to remove

**System Labels:**
- `INBOX` - Message in inbox
- `UNREAD` - Message is unread
- `STARRED` - Message is starred
- `IMPORTANT` - Message is important
- `TRASH` - Message in trash
- `SPAM` - Message is spam

**Example (Mark as Read):**
```json
{
  "tool": "gmail_modify_message",
  "arguments": {
    "message_id": "18c1a2b3d4e5f6g7",
    "remove_labels": ["UNREAD"]
  }
}
```

**Example (Archive):**
```json
{
  "tool": "gmail_modify_message",
  "arguments": {
    "message_id": "18c1a2b3d4e5f6g7",
    "remove_labels": ["INBOX"]
  }
}
```

**Example (Star and Add Label):**
```json
{
  "tool": "gmail_modify_message",
  "arguments": {
    "message_id": "18c1a2b3d4e5f6g7",
    "add_labels": ["STARRED", "ProjectX"]
  }
}
```

**Returns:**
Modified message object.

---

#### gmail_create_draft

Create a draft email.

**Parameters:**
- `to` (string, required): Recipient email
- `subject` (string, required): Email subject
- `body` (string, required): Email body
- `html` (boolean, optional): Whether body is HTML (default: false)

**Example:**
```json
{
  "tool": "gmail_create_draft",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "Draft Subject",
    "body": "This is a draft I'll send later."
  }
}
```

**Returns:**
Draft object with `id` and message details.

---

#### gmail_send_draft

Send a previously created draft.

**Parameters:**
- `draft_id` (string, required): Draft ID to send

**Example:**
```json
{
  "tool": "gmail_send_draft",
  "arguments": {
    "draft_id": "r123456789"
  }
}
```

**Returns:**
Sent message object.

---

### Calendar Tools (4 tools)

#### calendar_list_events

List calendar events.

**Parameters:**
- `calendar_id` (string, optional): Calendar ID (default: "primary")
- `max_results` (integer, optional): Maximum events to return (default: 100)
- `query` (string, optional): Free text search query

**Example (List Upcoming Events):**
```json
{
  "tool": "calendar_list_events",
  "arguments": {
    "calendar_id": "primary",
    "max_results": 10
  }
}
```

**Example (Search Events):**
```json
{
  "tool": "calendar_list_events",
  "arguments": {
    "query": "team meeting"
  }
}
```

**Returns:**
List of event objects with details like `summary`, `start`, `end`, `attendees`.

---

#### calendar_create_event

Create a new calendar event.

**Parameters:**
- `summary` (string, required): Event title
- `start_time` (string, required): Start time in ISO 8601 format
- `end_time` (string, required): End time in ISO 8601 format
- `description` (string, optional): Event description
- `location` (string, optional): Event location
- `attendees` (array of strings, optional): List of attendee emails

**Example (Simple Event):**
```json
{
  "tool": "calendar_create_event",
  "arguments": {
    "summary": "Team Meeting",
    "start_time": "2025-12-05T10:00:00Z",
    "end_time": "2025-12-05T11:00:00Z"
  }
}
```

**Example (Event with Details):**
```json
{
  "tool": "calendar_create_event",
  "arguments": {
    "summary": "Project Review",
    "start_time": "2025-12-05T14:00:00Z",
    "end_time": "2025-12-05T15:00:00Z",
    "description": "Q4 project status review",
    "location": "Conference Room A",
    "attendees": ["alice@example.com", "bob@example.com"]
  }
}
```

**Returns:**
Created event object with `id` and all details.

---

#### calendar_update_event

Update an existing calendar event.

**Parameters:**
- `event_id` (string, required): Event ID to update
- `calendar_id` (string, optional): Calendar ID (default: "primary")
- `summary` (string, optional): New event title
- `start_time` (string, optional): New start time
- `end_time` (string, optional): New end time

**Example:**
```json
{
  "tool": "calendar_update_event",
  "arguments": {
    "event_id": "abc123def456",
    "summary": "Rescheduled Team Meeting",
    "start_time": "2025-12-05T15:00:00Z",
    "end_time": "2025-12-05T16:00:00Z"
  }
}
```

**Returns:**
Updated event object.

---

#### calendar_delete_event

Delete a calendar event.

**Parameters:**
- `event_id` (string, required): Event ID to delete
- `calendar_id` (string, optional): Calendar ID (default: "primary")

**Example:**
```json
{
  "tool": "calendar_delete_event",
  "arguments": {
    "event_id": "abc123def456"
  }
}
```

**Returns:**
Success confirmation.

---

### People (Contacts) Tools (4 tools)

#### people_list_contacts

List all contacts.

**Parameters:**
- `page_size` (integer, optional): Number of contacts per page (default: 100)

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
List of contact objects with names, emails, and phone numbers.

---

#### people_search_contacts

Search for contacts by name or email.

**Parameters:**
- `query` (string, required): Search query

**Example:**
```json
{
  "tool": "people_search_contacts",
  "arguments": {
    "query": "John"
  }
}
```

**Returns:**
List of matching contact objects.

---

#### people_create_contact

Create a new contact.

**Parameters:**
- `given_name` (string, required): First name
- `family_name` (string, optional): Last name
- `email` (string, optional): Email address
- `phone` (string, optional): Phone number

**Example (Minimal):**
```json
{
  "tool": "people_create_contact",
  "arguments": {
    "given_name": "Jane"
  }
}
```

**Example (Complete):**
```json
{
  "tool": "people_create_contact",
  "arguments": {
    "given_name": "Jane",
    "family_name": "Doe",
    "email": "jane.doe@example.com",
    "phone": "+1-555-0123"
  }
}
```

**Returns:**
Created contact object with `resourceName` and all details.

---

#### people_delete_contact

Delete a contact.

**Parameters:**
- `resource_name` (string, required): Contact resource name (from contact object)

**Example:**
```json
{
  "tool": "people_delete_contact",
  "arguments": {
    "resource_name": "people/c123456789"
  }
}
```

**Returns:**
Success confirmation.

---

## Common Workflows

### Email Management

**Check unread emails:**
```json
{
  "tool": "gmail_list_messages",
  "arguments": {
    "query": "is:unread"
  }
}
```

**Read a specific email:**
```json
{
  "tool": "gmail_get_message",
  "arguments": {
    "message_id": "18c1a2b3d4e5f6g7"
  }
}
```

**Mark as read and archive:**
```json
{
  "tool": "gmail_modify_message",
  "arguments": {
    "message_id": "18c1a2b3d4e5f6g7",
    "remove_labels": ["INBOX", "UNREAD"]
  }
}
```

**Create draft and send later:**
```json
// Step 1: Create draft
{
  "tool": "gmail_create_draft",
  "arguments": {
    "to": "recipient@example.com",
    "subject": "Follow-up",
    "body": "Draft message"
  }
}

// Step 2: Send when ready
{
  "tool": "gmail_send_draft",
  "arguments": {
    "draft_id": "r123456789"
  }
}
```

**Organize with labels:**
```json
// Create label
{
  "tool": "gmail_create_label",
  "arguments": {
    "name": "Important-Client"
  }
}

// Apply label
{
  "tool": "gmail_modify_message",
  "arguments": {
    "message_id": "18c1a2b3d4e5f6g7",
    "add_labels": ["Important-Client"]
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

**Schedule a meeting:**
```json
{
  "tool": "calendar_create_event",
  "arguments": {
    "summary": "Weekly Standup",
    "start_time": "2025-12-05T09:00:00Z",
    "end_time": "2025-12-05T09:30:00Z",
    "attendees": ["team@example.com"]
  }
}
```

**Reschedule a meeting:**
```json
{
  "tool": "calendar_update_event",
  "arguments": {
    "event_id": "abc123",
    "start_time": "2025-12-05T10:00:00Z",
    "end_time": "2025-12-05T10:30:00Z"
  }
}
```

**Cancel a meeting:**
```json
{
  "tool": "calendar_delete_event",
  "arguments": {
    "event_id": "abc123"
  }
}
```

---

### Contact Management

**Find a contact:**
```json
{
  "tool": "people_search_contacts",
  "arguments": {
    "query": "John Doe"
  }
}
```

**Add new contact:**
```json
{
  "tool": "people_create_contact",
  "arguments": {
    "given_name": "Alice",
    "family_name": "Smith",
    "email": "alice@example.com",
    "phone": "+1-555-0199"
  }
}
```

**Email a contact:**
```json
// Step 1: Search for contact
{
  "tool": "people_search_contacts",
  "arguments": {
    "query": "John Doe"
  }
}

// Step 2: Extract email and send
{
  "tool": "gmail_send_message",
  "arguments": {
    "to": "john.doe@example.com",
    "subject": "Hello",
    "body": "Message to contact"
  }
}
```

---

## Best Practices

### Email
1. **Use specific queries** - Narrow down message searches with precise Gmail query syntax
2. **Batch operations** - Process multiple messages efficiently
3. **Labels over deletion** - Use labels and archive instead of permanent deletion
4. **Draft workflow** - Create drafts for important emails, review before sending

### Calendar
1. **Include attendees** - Always specify attendees for meetings
2. **Add locations** - Specify location for physical meetings or video conference links
3. **Descriptive summaries** - Use clear, searchable event titles
4. **Time zones** - Use ISO 8601 format with timezone (Z for UTC)

### Contacts
1. **Complete information** - Add as much detail as available (name, email, phone)
2. **Search before create** - Check if contact exists before creating duplicates
3. **Use resource_name** - Save the `resourceName` field for updates/deletes

### General
1. **Check for errors** - All tools return error information if operations fail
2. **Rate limits** - Server handles retries automatically, but avoid excessive calls
3. **Test with ish mode** - Use ish mode for development and testing without affecting real data

---

## Error Handling

All tools return errors in this format:

```json
{
  "error": "Error message",
  "details": "Detailed error information"
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

---

## Testing with Ish Mode

For testing without affecting real Google data, use ish mode:

```bash
# Set up ish mode
export ISH_MODE=true
export ISH_BASE_URL=http://localhost:9000
export ISH_USER=testuser

# Run server
uv run python -m gsuite_mcp
```

All tools work identically in ish mode. See [ISH_MODE.md](ISH_MODE.md) for details.
