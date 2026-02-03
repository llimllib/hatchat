# Phase 3: Rich Messaging - Design Document

## Overview

Phase 3 enriches Hatchat's messaging with three capabilities: formatted messages, message editing and deletion, and emoji reactions.

### In Scope
- Markdown rendering with syntax-highlighted code blocks (bold, italic, strikethrough, code, blockquotes, lists, links, headings)
- Auto-linking URLs (open in new tab)
- XSS sanitization of rendered HTML
- Inline message editing with up-arrow shortcut
- Soft delete with tombstone display
- Emoji reactions (any Unicode emoji, via OS picker)
- Message hover toolbar for edit/delete/react actions
- Real-time broadcast of edits, deletes, and reactions to room members

### Out of Scope (Deferred)
- File uploads and attachments
- Emoji shortcode conversion (`:smile:` → 😄)
- Emoji picker widget
- User presence and typing indicators

---

## Schema Changes

Two additive changes — no existing columns modified.

### Messages table — add `deleted_at`

```sql
ALTER TABLE messages ADD COLUMN deleted_at TEXT;
-- NULL = not deleted, RFC3339 timestamp = soft-deleted
```

When a message is deleted, we set `deleted_at` and clear `body` to an empty string. The row stays for conversation flow. Queries that return messages check `deleted_at` and return a tombstone representation instead of the original body.

### New `reactions` table

```sql
CREATE TABLE IF NOT EXISTS reactions(
  message_id TEXT REFERENCES messages(id) NOT NULL,
  user_id TEXT REFERENCES users(id) NOT NULL,
  emoji TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (message_id, user_id, emoji)
) STRICT;

CREATE INDEX IF NOT EXISTS reactions_message ON reactions(message_id);
```

The composite primary key `(message_id, user_id, emoji)` enforces that a user can only react once with the same emoji on a given message, but can add multiple different emoji. The index on `message_id` supports efficient loading of all reactions for a batch of messages.

---

## Protocol Changes

### New Message Types

Six new WebSocket message types, following the existing envelope pattern.

#### Edit

```go
// EditMessageRequest edits a message's body
// Direction: client → server
type EditMessageRequest struct {
    MessageID string `json:"message_id" jsonschema:"required"`
    Body      string `json:"body" jsonschema:"required,description=New message body"`
}

// MessageEdited is broadcast to room members when a message is edited
// Direction: server → client
type MessageEdited struct {
    MessageID  string `json:"message_id"`
    Body       string `json:"body"`
    RoomID     string `json:"room_id"`
    ModifiedAt string `json:"modified_at"`
}
```

#### Delete

```go
// DeleteMessageRequest soft-deletes a message
// Direction: client → server
type DeleteMessageRequest struct {
    MessageID string `json:"message_id" jsonschema:"required"`
}

// MessageDeleted is broadcast to room members
// Direction: server → client
type MessageDeleted struct {
    MessageID string `json:"message_id"`
    RoomID    string `json:"room_id"`
}
```

#### Reactions

```go
// AddReactionRequest adds an emoji reaction to a message
// Direction: client → server
type AddReactionRequest struct {
    MessageID string `json:"message_id" jsonschema:"required"`
    Emoji     string `json:"emoji" jsonschema:"required"`
}

// RemoveReactionRequest removes the user's reaction
// Direction: client → server
type RemoveReactionRequest struct {
    MessageID string `json:"message_id" jsonschema:"required"`
    Emoji     string `json:"emoji" jsonschema:"required"`
}

// ReactionUpdated is broadcast when a reaction is added or removed
// Direction: server → client
type ReactionUpdated struct {
    MessageID string `json:"message_id"`
    RoomID    string `json:"room_id"`
    UserID    string `json:"user_id"`
    Emoji     string `json:"emoji"`
    Action    string `json:"action"` // "add" or "remove"
}
```

### Updated Existing Types

The `Message` type gains two fields:

```go
type Message struct {
    // ... existing fields ...
    DeletedAt string     `json:"deleted_at,omitempty"` // Set if soft-deleted
    Reactions []Reaction `json:"reactions,omitempty"`   // Aggregated reactions
}

// Reaction represents an aggregated emoji reaction on a message
type Reaction struct {
    Emoji   string   `json:"emoji"`
    Count   int      `json:"count"`
    UserIDs []string `json:"user_ids"` // Who reacted (for "you reacted" highlighting)
}
```

Reactions are returned aggregated (grouped by emoji with count and user list) rather than as raw rows.

### Broadcast as Acknowledgment

Edit, delete, and reaction operations don't have separate response types. The broadcast serves as the acknowledgment — the requesting client receives the same `message_edited` / `message_deleted` / `reaction_updated` broadcast as everyone else in the room.

---

## Backend Implementation

### Handler Files

Four new files in `server/api/`:

- `edit_message.go` — Validates ownership (only your own messages), updates `body` and `modified_at`, broadcasts `message_edited` to room members.
- `delete_message.go` — Validates ownership, sets `deleted_at` and clears `body`, broadcasts `message_deleted` to room members.
- `add_reaction.go` — Validates room membership, inserts reaction row (ignore duplicate per primary key), broadcasts `reaction_updated` with action "add".
- `remove_reaction.go` — Validates room membership, deletes reaction row, broadcasts `reaction_updated` with action "remove".

### Authorization Rules

- **Edit/delete**: Only the message author. Look up the message, check `user_id` matches the requesting user. Return an error envelope if not.
- **Reactions**: Any room member. A user can react to anyone's message but can only remove their own reactions.
- **Deleted messages**: Can't be edited or reacted to. Handlers check `deleted_at` and reject with an error.

### Database Layer

New function in `server/db/`:

- `GetReactionsForMessages(ctx, messageIDs []string) (map[string][]Reaction, error)` — Batch-loads reactions for a set of messages, returns them pre-aggregated. Used by the history handler and init to populate reactions efficiently in a single query.

### History Handler Update

After fetching messages, call `GetReactionsForMessages` with the message IDs and attach the results. For deleted messages, clear the body and set `deleted_at` so the client renders a tombstone.

---

## Frontend — Markdown Rendering

### Libraries

- **`marked`** (~40KB) for Markdown parsing. Converts Markdown to HTML.
- **`highlight.js`** (~100-150KB with selected languages) for syntax highlighting. Bundled languages: JavaScript, TypeScript, Python, Go, Rust, SQL, HTML, CSS, shell/bash, JSON, YAML.
- **`DOMPurify`** (~20KB) for HTML sanitization against XSS.

### Rendering Pipeline

Message body (plain text) → `marked.parse()` → `DOMPurify.sanitize()` → set as `innerHTML`.

Rendering is client-side only. The server stores and transmits raw Markdown. This keeps the server simple and allows re-rendering if the renderer improves.

### Configuration

- Links open in new tabs (`target="_blank"` with `rel="noopener noreferrer"`)
- Code blocks get a highlight.js dark theme, distinct background, rounded corners, horizontal scroll
- No live preview while typing (YAGNI)

---

## Frontend — Message Hover Toolbar

A small floating toolbar appears at the top-right corner of a message on hover, containing icon buttons:

- 😀 (smiley face) — Opens a quick-pick reaction bar
- ✏️ (pencil) — Edit message (only shown on your own messages)
- 🗑️ (trash) — Delete message (only shown on your own messages)

The toolbar is a single shared DOM element that repositions on hover rather than one per message. On tombstoned messages, the toolbar is hidden.

### Reaction Quick-Pick Bar

Clicking the smiley button shows a small bar of 6-8 common reactions (👍 ❤️ 😂 😮 🎉 🔥). Clicking one sends `add_reaction`. A "+" button at the end of the reaction pill row also opens this bar.

---

## Frontend — Edit/Delete UI

### Inline Editing

Clicking edit (or pressing up-arrow in an empty input) replaces the message body with a textarea pre-filled with the raw Markdown. Two small buttons appear below: "Save" (Enter) and "Cancel" (Escape). Only one message can be in edit mode at a time.

### Up-Arrow Shortcut

When the main input is empty and the user presses up-arrow, find the most recent message by the current user in the current room and activate inline editing on it.

### Delete Confirmation

Clicking the trash icon shows a small confirmation popover anchored to the button: "Delete this message? [Delete] [Cancel]". No full-screen modal.

### Tombstone Display

Deleted messages render as a grayed-out italic line: *"This message was deleted."* No avatar, no timestamp — just the text in the message's position to preserve conversation flow.

---

## Frontend — Reactions Display

### Reaction Pills

Below each message with reactions, a horizontal row of pills: `👍 3` `😂 1`. Pills wrap to multiple lines if needed.

### Highlighting

If the current user has reacted with a given emoji, that pill gets a highlighted border/background (subtle blue outline).

### Toggle Behavior

Clicking a pill toggles your reaction — sends `add_reaction` if you haven't reacted, `remove_reaction` if you have.

### Tooltip

Hovering over a pill shows usernames who reacted. For many: "Alice, Bob, and 3 others".

### Real-Time Updates

When a `reaction_updated` broadcast arrives, the client updates the reaction bar in place — incrementing/decrementing counts, adding/removing pills, updating the user list. No re-fetch needed.

---

## Error Handling & Edge Cases

- **Editing a deleted message**: Server rejects with error. Client hides edit button on tombstones.
- **Reacting to a deleted message**: Server rejects. Client hides hover toolbar on tombstones.
- **Editing a message with reactions**: Allowed. Reactions persist through edits.
- **Concurrent edits**: Last write wins. No conflict resolution — this is chat, not a document editor.
- **Delete while editing**: If a `message_deleted` broadcast arrives for a message being edited, cancel edit mode and show tombstone.
- **Stale cache**: Edit/delete/reaction broadcasts update the client's per-room message cache, not just the DOM.
- **Room membership**: All handlers verify the requesting user is a member of the message's room.
- **Message not found**: Return error envelope.
- **Empty edit body**: Rejected. Use delete instead.

---

## Testing Strategy

### Unit Tests

- `edit_message_test.go` — Edit own message succeeds, edit other's fails, edit deleted fails, empty body rejected.
- `delete_message_test.go` — Delete own succeeds, delete other's fails, delete already-deleted is idempotent.
- `add_reaction_test.go` / `remove_reaction_test.go` — Add succeeds, duplicate is idempotent, remove own succeeds, react to deleted fails, non-member rejected.
- `db/reactions_test.go` — Aggregation correctness, empty inputs, grouping by emoji.

### Integration Tests

- Edit broadcast received by other client.
- Delete broadcast with tombstone.
- Reaction add/remove broadcasts.
- History returns edited body, tombstones, aggregated reactions.
- Authorization: can't edit/delete other user's messages.

### E2E Tests (Playwright)

- Hover toolbar → edit → save → verify "(edited)" indicator.
- Up-arrow shortcut activates editing.
- Delete → confirm → tombstone appears.
- Add reaction via quick-pick → pill appears.
- Toggle reaction by clicking pill.
- Real-time: other user sees edits/deletes/reactions.

---

## File Changes Summary

### New Files
- `server/api/edit_message.go`
- `server/api/delete_message.go`
- `server/api/add_reaction.go`
- `server/api/remove_reaction.go`
- `server/db/reactions.go`
- `server/db/reactions_test.go`

### Modified Files
- `schema.sql` — Add `deleted_at` to messages, add `reactions` table
- `server/protocol/protocol.go` — New message types, updated `Message`
- `tools/schemagen/main.go` — Register new types
- `server/client.go` — Four new switch cases
- `server/api/history.go` — Attach reactions, handle tombstones
- `server/api/message.go` — Attach empty reactions for consistency
- `client/src/index.ts` — Markdown rendering, hover toolbar, inline editing, reactions UI, tombstones
- `client/src/types.ts` — Re-export new types
- `client/package.json` — Add `marked`, `highlight.js`, `dompurify` dependencies
- CSS — Styles for all new UI components

### Implementation Order
1. Schema + models
2. Protocol + types
3. Edit/delete backend
4. Reactions backend
5. Markdown rendering
6. Hover toolbar
7. Inline editing + up-arrow
8. Delete UI
9. Reactions UI
10. Tests
