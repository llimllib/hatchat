# Unread Tracking Design

**Date:** 2026-02-28
**Status:** Complete
**Phase:** 5.1

## Overview

Implement unread message tracking so users can see which rooms have new messages they haven't read yet.

## Goals

1. Track the last message each user has read in each room
2. Show unread counts in the sidebar
3. Bold room names with unread messages
4. Show a "New messages" divider at the first unread message
5. Provide "Mark as read" functionality

## Database Design

### New Table: `read_positions`

```sql
CREATE TABLE IF NOT EXISTS read_positions(
  user_id TEXT REFERENCES users(id) NOT NULL,
  room_id TEXT REFERENCES rooms(id) NOT NULL,
  last_read_at TEXT NOT NULL, -- RFC3339Nano timestamp of last read
  PRIMARY KEY (user_id, room_id)
) STRICT;
```

**Design decisions:**

1. **Timestamp vs Message ID**: We use `last_read_at` (timestamp) rather than `last_read_message_id` because:
   - Simpler to compare: all messages after this timestamp are unread
   - Works even if the "last read" message is deleted
   - Aligns with how we query messages (by created_at)

2. **No explicit unread count**: We calculate unread counts on-the-fly rather than maintaining a counter because:
   - Simpler data model (no denormalization)
   - Avoids race conditions when updating counts
   - SQLite is fast enough for this use case

## Protocol Changes

### New Message Types

#### `mark_read` (client → server)

Marks all messages in a room as read up to a given timestamp.

```typescript
interface MarkReadRequest {
  room_id: string;
  read_at: string;  // RFC3339Nano timestamp, typically the latest message timestamp
}
```

#### `mark_read_response` (server → client)

Confirms the read position was updated.

```typescript
interface MarkReadResponse {
  room_id: string;
  read_at: string;
  unread_count: number;  // Always 0 after marking read to current
}
```

#### `unread_counts` (server → client)

Sent as part of the init response and when unread counts change.

```typescript
interface UnreadCounts {
  counts: Record<string, number>;  // room_id -> unread count
}
```

### Changes to Existing Types

#### `InitResponse`

Add `unread_counts` to the init response:

```typescript
interface InitResponse {
  // ... existing fields ...
  unread_counts: Record<string, number>;
}
```

#### `MessageBroadcast`

No changes needed. The client receives new messages and can increment local unread counts.

## Backend Implementation

### New Handler: `mark_read`

1. Validate request
2. Verify user is member of room
3. Upsert read position in `read_positions` table
4. Return success with new unread count (0)

### Changes to `init` Handler

1. After fetching rooms and DMs, fetch unread counts for all rooms
2. Include `unread_counts` map in response

### New DB Function: `GetUnreadCounts`

```go
func (db *DB) GetUnreadCounts(ctx context.Context, userID string) (map[string]int, error) {
    // For each room the user is a member of:
    // - If no read_position exists, count all messages
    // - Otherwise, count messages with created_at > last_read_at
}
```

### New DB Function: `SetReadPosition`

```go
func (db *DB) SetReadPosition(ctx context.Context, userID, roomID, readAt string) error {
    // UPSERT into read_positions
}
```

## Frontend Implementation

### State Management

Add to app state:
- `unreadCounts: Map<string, number>` - current unread counts per room
- Update counts when receiving messages
- Clear count when marking room as read

### Sidebar UI

- Show unread count badge next to room name (e.g., `#general (3)`)
- Bold room name if unread count > 0
- Consider maximum display count (e.g., "99+" for more than 99)

### Message Area

- When switching to a room, check if there are unread messages
- If unread, insert a "New messages" divider above the first unread message
- Divider styled prominently (red/orange line with "New messages" text)

### Mark as Read Behavior

**Automatic marking:**
- When user views a room and it's scrolled to the bottom, auto-mark as read
- Use IntersectionObserver to detect when the latest message is visible
- Debounce the mark_read call (e.g., wait 500ms of visibility)

**Manual marking:**
- Right-click room name → "Mark as read"
- Consider "Mark all as read" in sidebar header

### Local Unread Tracking

When receiving a new `message_broadcast`:
- If room_id != current room OR window is not focused:
  - Increment local unread count for that room
- If room_id == current room AND window is focused AND scrolled to bottom:
  - Don't increment; instead, send mark_read immediately

## Performance Considerations

1. **Unread count query**: Use a single query with GROUP BY to get all counts at once
2. **Index**: The existing `messages_room_created` index supports efficient unread counting
3. **Initial load**: Unread counts are included in init, so no extra round-trip

## Edge Cases

1. **First time in room**: No read_position exists → all messages are unread
2. **Room with no messages**: Count is 0
3. **Multiple tabs**: Last mark_read wins (idempotent timestamp comparison)
4. **Deleted messages**: Don't affect unread count (soft-deleted messages are still counted by timestamp, but this is acceptable)

## Implementation Plan

### Step 1: Database Schema
- [x] Add `read_positions` table to schema.sql
- [x] Add model (readposition.dbtpl.go)

### Step 2: Protocol Types
- [x] Add `MarkReadRequest`, `MarkReadResponse`, `UnreadCounts` to protocol.go
- [x] Add to schemagen
- [x] Regenerate client types
- [x] Update `InitResponse` to include `unread_counts`

### Step 3: Backend
- [x] Add `GetUnreadCounts` and `SetReadPosition` to db package
- [x] Add `mark_read` handler
- [x] Update `init` handler to include unread counts

### Step 4: Frontend
- [x] Add unread state management (AppState)
- [x] Update sidebar to show unread badges
- [x] Auto-mark-as-read when switching rooms
- [x] Auto-mark-as-read after history loads
- [x] Increment unread count for messages in other rooms
- [x] Add "New messages" divider line at first unread message
- [x] Add manual "Mark as read" context menu on right-click room
- [x] Add "Mark all as read" option in user dropdown menu
- [x] Add `mark_all_read` WebSocket message type (backend + frontend)

### Step 5: Testing
- [x] Unit tests for new DB functions
- [x] Unit tests for unread state management
- [x] Unit tests for read positions and markAllRoomsAsRead
- [x] DB tests for GetReadPositions and MarkAllRead
- [x] Integration tests for mark_read handler
- [x] Integration tests for mark_all_read handler
