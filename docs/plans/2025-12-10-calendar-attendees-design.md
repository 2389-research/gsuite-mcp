# Calendar Attendees Support

Add attendees (guests) to calendar events via `calendar_create_event` and `calendar_update_event`.

## Parameters

### calendar_create_event

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `attendees` | `string[]` | `[]` | Email addresses of required attendees |
| `optional_attendees` | `string[]` | `[]` | Email addresses of optional attendees |
| `send_notifications` | `boolean` | `true` | Send invite emails to attendees |

### calendar_update_event

| Parameter | Type | Description |
|-----------|------|-------------|
| `attendees` | `string[]` | Full replacement - replaces ALL required attendees |
| `optional_attendees` | `string[]` | Full replacement - replaces ALL optional attendees |
| `add_attendees` | `string[]` | Incremental - add as required attendees |
| `add_optional_attendees` | `string[]` | Incremental - add as optional attendees |
| `remove_attendees` | `string[]` | Incremental - remove by email |
| `send_notifications` | `boolean` | Send update emails (default: `true`) |

## Behavior

### Create Event

All attendees added to the event with appropriate `optional` flag. Google sends invite emails if `send_notifications` is true.

### Update Event

Two mutually exclusive modes:

1. **Full replacement mode** - If `attendees` OR `optional_attendees` provided, wipes existing attendees and sets new ones.

2. **Incremental mode** - If only `add_*` / `remove_*` params provided, merges changes with existing attendees.

Mixing full replacement and incremental params in the same call returns an error.

Omitting all attendee params leaves attendees unchanged.

## Examples

### Create with attendees

```json
{
  "summary": "Team Standup",
  "start_time": "2025-12-15T10:00:00-06:00",
  "end_time": "2025-12-15T10:30:00-06:00",
  "attendees": ["alice@example.com", "bob@example.com"],
  "optional_attendees": ["charlie@example.com"],
  "send_notifications": true
}
```

### Update - Full replacement

```json
{
  "event_id": "abc123",
  "attendees": ["new-team@example.com"]
}
```

### Update - Incremental

```json
{
  "event_id": "abc123",
  "add_attendees": ["newcomer@example.com"],
  "remove_attendees": ["departed@example.com"]
}
```

## Implementation

### Files to modify

1. `pkg/calendar/service.go` - Add attendee handling to CreateEvent, update signatures
2. `pkg/server/server.go` - Update tool schemas and handlers

### Google Calendar API

```go
event := &calendar.Event{
    Summary: summary,
    Attendees: []*calendar.EventAttendee{
        {Email: "alice@example.com"},
        {Email: "bob@example.com", Optional: true},
    },
}

call := s.svc.Events.Insert("primary", event).
    SendNotifications(true)
```

## Test Coverage

- Unit tests for service layer attendee handling
- Integration tests for create/update with attendees
- Edge cases: empty arrays, incremental add/remove, mode mixing error
