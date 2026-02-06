# Phase 4.4: Search - Design Document

## Overview

Phase 4.4 adds search capabilities to Hatchat with two distinct interfaces: a quick-search panel (Cmd+K) for navigation, and a dedicated search page for full-text message search.

### In Scope
- Quick-search modal (Cmd+K) for navigating to rooms, DMs, and users
- Recent items shown in quick-search before typing
- Dedicated search page at `/search` with full-text message search
- SQLite FTS5 for efficient full-text indexing
- Filters by room and user
- Search result snippets with highlighted terms
- Message permalinks (`/chat#msg_abc123`)
- Jump-to-message with brief highlight animation

### Out of Scope (Deferred)
- Date range filtering
- Search within attachments/files
- Saved searches
- Search result ranking/relevance tuning

---

## Quick-Search (Cmd+K) Panel

### Trigger & Appearance
- **Keyboard shortcut**: Cmd+K (Mac) / Ctrl+K (Windows/Linux)
- **Visual**: Centered modal, ~500px wide, with a large search input at top
- **Backdrop**: Semi-transparent overlay dimming the chat behind it
- **Dismiss**: Escape key, clicking backdrop, or selecting an item

### Initial State (before typing)
When opened, shows a "Recent" section with:
- Last 5-8 visited rooms/DMs (tracked in localStorage)
- Each item shows icon (# for channel, avatar for DM) + name

### Search Behavior
As user types:
- Filter rooms by name (case-insensitive substring match)
- Filter users by username/display_name
- Results grouped: "Channels", "Direct Messages", "Users"
- Limit ~10 total results for speed

### Navigation
- Arrow keys move selection highlight
- Enter selects (switches to room, or opens DM with user)
- Results are keyboard-navigable with visible focus state

### "Search messages" Escape Hatch
- At bottom of results: "Search messages for '[query]' →"
- Clicking or pressing Enter when highlighted navigates to `/search?q=[query]`
- Also shown when no navigation results match

---

## Advanced Search Page

### URL Structure
- `/search` - Empty search page with input focused
- `/search?q=hello` - Search for "hello"
- `/search?q=hello&room=roo_abc123` - Filter to specific room
- `/search?q=hello&from=usr_xyz789` - Filter to specific user
- Combined filters work: `/search?q=hello&room=...&from=...`

### Page Layout
```
┌─────────────────────────────────────────────────┐
│ [← Back]              Search                    │
├─────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────┐ │
│ │ 🔍 Search messages...                       │ │
│ └─────────────────────────────────────────────┘ │
│ Filters: [All rooms ▾]  [Anyone ▾]              │
├─────────────────────────────────────────────────┤
│                                                 │
│ 12 results for "hello"                          │
│                                                 │
│ ┌─────────────────────────────────────────────┐ │
│ │ #general · alice · Feb 3, 2026              │ │
│ │ "...said **hello** to everyone in the..."   │ │
│ └─────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────┐ │
│ │ #random · bob · Feb 2, 2026                 │ │
│ │ "**hello** world, this is my first..."      │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ [Load more results]                             │
└─────────────────────────────────────────────────┘
```

### Search Input
- Large, prominent text input at top
- Searches on Enter (not live keystroke - avoids excessive queries)
- Query persists in URL for bookmarking/sharing

### Filter Dropdowns
- **Room filter**: "All rooms" + list of user's rooms (channels + DMs)
- **User filter**: "Anyone" + searchable user dropdown (reuse existing user picker)
- Selecting a filter updates URL and re-runs search

### Result Cards
Each result shows:
- Room name + author + timestamp (header line)
- Message snippet with search terms **highlighted** (bold)
- Snippet is ~100-150 chars with ellipsis, centered on match
- Clicking navigates to `/chat#msg_abc123`

### Pagination
- Initial load: 20 results
- "Load more" button appends next 20 (cursor-based, like message history)

---

## Message Permalinks & Jump-to-Message

### URL Format
- `/chat#msg_abc123` - Chat view with message highlighted
- Works from search results, and later can be copied/shared directly

### Navigation Flow
When visiting a permalink:
1. Parse message ID from URL hash
2. Fetch message metadata (room ID) via `get_message_context` API call
3. Switch to that room (if not already there)
4. Load messages around that timestamp using existing history API
5. Scroll to the target message
6. Briefly highlight it (yellow fade-out over ~2 seconds)

### Edge Cases
- **Message deleted**: Show room at that timestamp, display "Message was deleted" toast
- **No access to room**: Show error "You don't have access to this message"
- **Message not found**: Show error "Message not found"
- **Already in room**: Skip room switch, just scroll and highlight

---

## Schema Changes

### FTS5 Virtual Table

```sql
-- FTS5 virtual table for message search
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    body,
    content='messages',
    content_rowid='rowid'
);
```

Uses "external content" mode - the FTS table doesn't store text itself, just the index. The `content='messages'` links it to the messages table.

### Triggers for Index Sync

```sql
-- Insert trigger
CREATE TRIGGER messages_fts_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, body) VALUES (NEW.rowid, NEW.body);
END;

-- Update trigger  
CREATE TRIGGER messages_fts_update AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, body) VALUES('delete', OLD.rowid, OLD.body);
    INSERT INTO messages_fts(rowid, body) VALUES (NEW.rowid, NEW.body);
END;

-- Delete trigger
CREATE TRIGGER messages_fts_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, body) VALUES('delete', OLD.rowid, OLD.body);
END;
```

### Backfill Existing Messages

One-time command after migration:
```sql
INSERT INTO messages_fts(rowid, body) SELECT rowid, body FROM messages;
```

---

## Protocol Changes

### Search Request/Response

```go
// SearchRequest searches messages across rooms the user has access to
// Direction: client → server
type SearchRequest struct {
    Query  string `json:"query" jsonschema:"required,description=Search query text"`
    RoomID string `json:"room_id,omitempty" jsonschema:"description=Filter to specific room"`
    UserID string `json:"user_id,omitempty" jsonschema:"description=Filter to messages from specific user"`
    Cursor string `json:"cursor,omitempty" jsonschema:"description=Pagination cursor for next page"`
    Limit  int    `json:"limit,omitempty" jsonschema:"description=Max results to return (default 20)"`
}

// SearchResponse returns matching messages with snippets
// Direction: server → client
type SearchResponse struct {
    Results    []SearchResult `json:"results"`
    NextCursor string         `json:"next_cursor,omitempty"`
    Total      int            `json:"total,omitempty" jsonschema:"description=Approximate total matches"`
}

// SearchResult is a single search hit with context snippet
type SearchResult struct {
    MessageID string `json:"message_id"`
    RoomID    string `json:"room_id"`
    RoomName  string `json:"room_name"`
    UserID    string `json:"user_id"`
    Username  string `json:"username"`
    Snippet   string `json:"snippet" jsonschema:"description=Message excerpt with **highlighted** matches"`
    CreatedAt string `json:"created_at"`
}
```

### Message Context (for permalinks)

```go
// GetMessageContextRequest fetches a message with surrounding context
// Direction: client → server
type GetMessageContextRequest struct {
    MessageID string `json:"message_id" jsonschema:"required"`
}

// GetMessageContextResponse returns the message and its room for navigation
// Direction: server → client
type GetMessageContextResponse struct {
    Message Message `json:"message"`
    RoomID  string  `json:"room_id"`
}
```

---

## Search Query

```sql
SELECT m.id, m.room_id, m.user_id, m.body, m.created_at,
       snippet(messages_fts, 0, '**', '**', '...', 20) as snippet
FROM messages_fts
JOIN messages m ON messages_fts.rowid = m.rowid
WHERE messages_fts MATCH ?
  AND m.deleted_at IS NULL
  AND m.room_id IN (SELECT room_id FROM rooms_members WHERE user_id = ?)
ORDER BY m.created_at DESC
LIMIT 20 OFFSET ?
```

The `snippet()` function extracts context with highlighted terms:
- `0` = column index (body)
- `'**'` / `'**'` = start/end markers for highlights
- `'...'` = ellipsis for truncation
- `20` = max tokens to return

Optional filters add:
- Room filter: `AND m.room_id = ?`
- User filter: `AND m.user_id = ?`

---

## Backend Implementation

### New Files

**`server/api/search.go`**
- `Search()` handler - validates query (non-empty), builds FTS5 query with optional room/user filters, checks room membership authorization, returns paginated results with snippets

**`server/api/message_context.go`**
- `GetMessageContext()` handler - fetches message by ID, verifies user has access to the room, returns message and room ID

**`server/db/search.go`**
- `SearchMessages(ctx, userID, query, roomID, userID, cursor, limit)` - executes FTS5 query with authorization check, returns results with snippets

### Modified Files

- `schema.sql` - Add FTS5 virtual table and triggers
- `server/protocol/protocol.go` - Add new message types
- `tools/schemagen/main.go` - Register new types for schema generation
- `server/client.go` - Add `search` and `get_message_context` cases to readPump switch

### Authorization

Both handlers verify room membership:
- `Search`: Query only returns messages from rooms where user is a member (enforced in SQL via subquery)
- `GetMessageContext`: Check user is member of message's room before returning

---

## Frontend Implementation

### New Files

**`client/src/search.ts`** (or inline in index.ts)
- `SearchPage` class/functions for the `/search` page
- Render search input, filter dropdowns, results list
- Handle URL params, form submission, pagination

**`client/src/quicksearch.ts`** (or inline in index.ts)
- `QuickSearch` class for the Cmd+K modal
- Track recent rooms/DMs in localStorage
- Filter and render results as user types
- Keyboard navigation (arrow keys, enter, escape)

### Modified Files

- `client/src/index.ts` - Add Cmd+K keyboard listener, handle URL hash for permalinks, handle `/search` route
- `client/src/types.ts` - Re-export new protocol types
- `static/chat.css` - Styles for quick-search modal, search page, message highlight animation

### Routing Approach

- `/search` serves same HTML template
- Client checks `window.location.pathname` on init
- If `/search`, render search UI instead of chat UI
- Back button navigates to `/chat`

### Recent Items Storage

For Cmd+K recents:
- Store last 8 room IDs in `localStorage` under key `hatchat:recent_rooms`
- Update on room switch
- On quick-search open, look up room details from app state

---

## Testing Strategy

### Unit Tests

**`server/api/search_test.go`**
- Empty query returns error
- Search returns matching messages
- Results respect room membership (can't see messages from rooms user isn't in)
- Room filter works
- User filter works
- Pagination works (cursor-based)
- Deleted messages excluded from results

**`server/api/message_context_test.go`**
- Returns message and room ID
- Returns error for non-existent message
- Returns error if user not a member of room

**`server/db/search_test.go`**
- FTS5 query building
- Snippet generation
- Authorization filtering

### Integration Tests

- Search → click result → navigates to room with message highlighted
- Filters update URL and results
- Quick-search filters rooms/users as you type
- Quick-search "Search messages" link navigates to search page with query

### E2E Tests (Playwright)

**`e2e/tests/search.spec.ts`**
- Cmd+K opens quick-search modal
- Typing filters rooms/users
- Selecting room navigates to it
- "Search messages" link works
- Search page returns results
- Clicking result jumps to message in room
- Message is highlighted briefly
- Filters work (room dropdown, user dropdown)
- Permalink URL is shareable (visit directly, lands on correct message)

---

## Implementation Order

### Phase A: FTS5 Foundation
1. Add FTS5 table and triggers to `schema.sql`
2. Run migration (manually or via startup)
3. Backfill existing messages into FTS index (one-time script)

### Phase B: Search Backend
4. Add protocol types (`SearchRequest`, `SearchResponse`, etc.)
5. Add `server/db/search.go` with FTS query
6. Add `server/api/search.go` handler
7. Wire up in `client.go`
8. Regenerate types (`just client-types`)
9. Unit tests for search

### Phase C: Search Page Frontend
10. Add `/search` route handling
11. Build search page UI (input, filters, results)
12. Wire up WebSocket search calls
13. Style results with highlighted snippets

### Phase D: Permalinks
14. Add `GetMessageContext` protocol types
15. Add `server/api/message_context.go` handler
16. Handle URL hash on page load
17. Scroll-to and highlight animation
18. Unit tests for message context

### Phase E: Quick-Search (Cmd+K)
19. Build modal UI
20. Track recent rooms in localStorage
21. Filter logic for rooms/users
22. Keyboard navigation
23. "Search messages" escape hatch

### Phase F: E2E Tests
24. Add `e2e/tests/search.spec.ts`

---

## File Changes Summary

### New Files
- `server/api/search.go`
- `server/api/search_test.go`
- `server/api/message_context.go`
- `server/api/message_context_test.go`
- `server/db/search.go`
- `server/db/search_test.go`
- `e2e/tests/search.spec.ts`

### Modified Files
- `schema.sql` - FTS5 table and triggers
- `server/protocol/protocol.go` - New message types
- `tools/schemagen/main.go` - Register new types
- `server/client.go` - New switch cases
- `client/src/index.ts` - Quick-search, permalinks, search page routing
- `client/src/types.ts` - Re-export new types
- `static/chat.css` - New UI styles
