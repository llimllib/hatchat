# Phase 2: Direct Messages & User Profiles - Design Document

## Overview

Phase 2 adds direct messaging (DMs) and basic user profiles to Hatchat. DMs are implemented as regular rooms with `room_type = 'dm'`, allowing group DMs to work naturally.

## Scope

### In Scope
- **2.1 Direct Messages**: DM rooms between users (1:1 and group)
- **2.3 User Profiles** (minimal): Display name, status message, profile viewer

### Deferred
- 2.2 User Presence (online/offline tracking)
- 2.4 Typing Indicators
- Avatar upload/URL

---

## 2.1 Direct Messages

### Design Principles

1. **DMs are just rooms** - They use the existing room/message infrastructure
2. **No special message handling** - Same `message` type, same storage
3. **Distinguishing trait**: `room_type = 'dm'` instead of `'channel'`
4. **Private by nature**: DMs are always `is_private = true`
5. **Group DMs are natural**: Just add more members

### Schema Changes

```sql
-- Add room_type column to rooms table
ALTER TABLE rooms ADD COLUMN room_type TEXT NOT NULL DEFAULT 'channel';
-- Valid values: 'channel', 'dm'

-- Add last_message_at for sorting DMs by recency
ALTER TABLE rooms ADD COLUMN last_message_at TEXT;
-- NULL means no messages yet; updated whenever a message is sent to the room
```

New schema for rooms:
```sql
CREATE TABLE IF NOT EXISTS rooms(
  id TEXT PRIMARY KEY NOT NULL,
  name TEXT NOT NULL,  -- Empty for DMs; display name derived from members
  room_type TEXT NOT NULL DEFAULT 'channel',  -- 'channel' or 'dm'
  is_private INTEGER NOT NULL,
  is_default INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  last_message_at TEXT  -- For sorting DMs by most recent activity
) STRICT;

-- Note: Remove UNIQUE constraint on name since DMs can have empty names
-- Instead, add a partial unique index for channels only
DROP INDEX IF EXISTS rooms_name;
CREATE UNIQUE INDEX IF NOT EXISTS rooms_name ON rooms(name) WHERE room_type = 'channel' AND name != '';
```

### Protocol Changes

#### Updated Types

```go
// Room represents a chat room
type Room struct {
    ID        string `json:"id"`
    Name      string `json:"name"`           // Empty for DMs
    RoomType  string `json:"room_type"`      // "channel" or "dm"
    IsPrivate bool   `json:"is_private"`
}
```

#### New Message Types

```go
// CreateDMRequest creates or finds an existing DM with the given users
// Direction: client → server
type CreateDMRequest struct {
    UserIDs []string `json:"user_ids" jsonschema:"required,minItems=1,description=User IDs to start DM with (not including self)"`
}

// CreateDMResponse returns the DM room (existing or newly created)
// Direction: server → client
type CreateDMResponse struct {
    Room    Room   `json:"room"`
    Created bool   `json:"created"`  // true if new DM was created, false if existing
}

// ListUsersRequest gets users for the user picker
// Direction: client → server
type ListUsersRequest struct {
    Query string `json:"query" jsonschema:"description=Search query for username"`
}

// ListUsersResponse returns matching users
// Direction: server → client  
type ListUsersResponse struct {
    Users []User `json:"users"`
}
```

#### Updated InitResponse

```go
type InitResponse struct {
    User        User    `json:"user"`
    Rooms       []*Room `json:"rooms"`       // Channels the user is in
    DMs         []*Room `json:"dms"`         // DM rooms the user is in (NEW)
    CurrentRoom string  `json:"current_room"`
}
```

### Finding Existing DMs

When creating a DM, we need to check if one already exists with exactly the same members.

**Algorithm**:
1. Get all DM rooms where the requesting user is a member
2. For each DM, get the member list
3. Check if member set matches the requested set (including self)
4. If found, return existing; otherwise create new

**Optimization** (if needed later): Add a `member_hash` column to rooms that stores a hash of sorted member IDs. This would allow a single indexed lookup.

For now, the simple approach is fine since:
- Users typically have few DM rooms (dozens, not thousands)
- DM creation is infrequent

### DM Display Names

DMs don't have a `name` field. The display name is derived from the members:

- **1:1 DM**: Show the other user's name
- **Group DM**: Show comma-separated names, e.g., "Alice, Bob, Charlie"
  - If too long, truncate: "Alice, Bob, and 3 others"

This logic lives in the **client**, not the server. The server sends member info with the room.

**New field needed**: For efficiency, include member info in the Room for DMs:

```go
type Room struct {
    ID        string       `json:"id"`
    Name      string       `json:"name"`
    RoomType  string       `json:"room_type"`
    IsPrivate bool         `json:"is_private"`
    Members   []RoomMember `json:"members,omitempty"`  // Only populated for DMs
}
```

### UI Components

#### Sidebar Structure

```
Channels
  # general
  # random
  + Create channel
  Browse channels

Direct Messages           <-- NEW section
  Alice                   <-- 1:1 DM
  Bob, Charlie            <-- Group DM
  + New message           <-- Opens user picker
```

#### User Picker Modal

When user clicks "+ New message":

1. Modal opens with title "New message"
2. "To:" field with tag-style input (like email)
3. As user types, autocomplete dropdown shows matching users
4. Selected users appear as removable tags
5. "Start Conversation" button creates/finds DM and navigates to it

```
┌─────────────────────────────────────────┐
│ New message                           ✕ │
├─────────────────────────────────────────┤
│ To: [Alice ✕] [Bob ✕] [___________▼]   │
│     ┌─────────────────────────────┐     │
│     │ Charlie                     │     │
│     │ Carol                       │     │
│     │ Chris                       │     │
│     └─────────────────────────────┘     │
│                                         │
│           [Cancel] [Start Conversation] │
└─────────────────────────────────────────┘
```

**Behavior**:
- Type to search users by username
- Click user or press Enter to add as tag
- Click ✕ on tag to remove
- Dropdown filters as you type
- Can select multiple users for group DM

---

## 2.3 User Profiles (Minimal)

### Schema Changes

```sql
-- Add display_name and status to users table
ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT '';
```

### Protocol Changes

#### Updated User Type

```go
type User struct {
    ID          string `json:"id"`
    Username    string `json:"username"`
    DisplayName string `json:"display_name"`  // NEW: can be empty
    Status      string `json:"status"`        // NEW: custom status message
    Avatar      string `json:"avatar"`        // existing but unused for now
}
```

#### New Message Types

```go
// GetProfileRequest fetches a user's profile
// Direction: client → server
type GetProfileRequest struct {
    UserID string `json:"user_id" jsonschema:"required"`
}

// GetProfileResponse returns user profile data
// Direction: server → client
type GetProfileResponse struct {
    User User `json:"user"`
}

// UpdateProfileRequest updates the current user's profile
// Direction: client → server
type UpdateProfileRequest struct {
    DisplayName *string `json:"display_name,omitempty"`  // nil = don't update
    Status      *string `json:"status,omitempty"`        // nil = don't update
}

// UpdateProfileResponse confirms profile update
// Direction: server → client
type UpdateProfileResponse struct {
    User User `json:"user"`
}
```

### UI Components

#### Profile Viewer

Clicking on a username (in messages or member list) opens a profile panel/modal:

```
┌─────────────────────────────────────────┐
│ Profile                               ✕ │
├─────────────────────────────────────────┤
│  ┌────┐                                 │
│  │ AL │  Alice                          │
│  └────┘  @alice                         │
│                                         │
│  Status: Working on the new feature 🚀  │
│                                         │
│  [Message]                              │
└─────────────────────────────────────────┘
```

- Avatar placeholder with initials (existing logic)
- Display name (or username if not set)
- @username below
- Status message
- "Message" button starts/opens DM

#### Profile Editor

Access via dropdown in sidebar header (click on own username):

```
┌─────────────────────────────────────────┐
│ Edit Profile                          ✕ │
├─────────────────────────────────────────┤
│ Display name                            │
│ [Alice Smith_____________________]      │
│                                         │
│ Status                                  │
│ [Working on the new feature 🚀___]      │
│                                         │
│              [Cancel] [Save]            │
└─────────────────────────────────────────┘
```

---

## Implementation Plan

### Step 1: Schema Migration

1. Add migration script to add columns:
   - `rooms.room_type` (with default 'channel')
   - `rooms.last_message_at` (for DM ordering by recency)
   - `users.display_name` 
   - `users.status`
2. Update `schema.sql` with new schema
3. Update unique index on room name (partial index for channels only)
4. Regenerate models with `just models`

### Step 2: Protocol Updates

1. Update `server/protocol/protocol.go`:
   - Add `RoomType` to `Room`
   - Add `DisplayName`, `Status` to `User`
   - Add `Members` to `Room` (optional, for DMs)
   - Add `DMs` to `InitResponse`
   - Add new message types (CreateDM, ListUsers, GetProfile, UpdateProfile)
2. Update `tools/schemagen/main.go` with new types
3. Run `just client-types`

### Step 3: Backend - DM Support

1. Add `create_dm` handler in `server/api/dm.go`:
   - Find existing DM with same members, or create new
   - Return the room with members populated
2. Add `list_users` handler in `server/api/users.go`:
   - Search users by username
   - Exclude the requesting user from results
3. Update `init` handler to separate rooms and DMs
4. Update `InitResponse` building to populate DM member info
5. Update message handler to set `last_message_at` on the room when a message is sent
6. Update `leave_room` handler to reject leaving 1:1 DMs (only group DMs can be left)
7. Sort DMs by `last_message_at` DESC (most recent first) in init response

### Step 4: Backend - Profile Support

1. Add `get_profile` handler
2. Add `update_profile` handler
3. Update user queries to include new fields

### Step 5: Frontend - DM UI

1. Update sidebar to have two sections (Channels, Direct Messages)
2. Add "+ New message" button
3. Implement user picker modal with tag-style input
4. Handle `create_dm` and `list_users` message types
5. Display DM names derived from members

### Step 6: Frontend - Profile UI

1. Make usernames clickable (in messages, member list)
2. Implement profile viewer modal
3. Add "Message" button in profile to start DM
4. Add profile editor modal (accessible from sidebar dropdown)
5. Add user dropdown menu in sidebar header

### Step 7: Testing

1. Unit tests for DM creation/finding logic
2. Integration tests for new WebSocket handlers
3. E2E tests:
   - Create 1:1 DM
   - Create group DM
   - Find existing DM (don't duplicate)
   - Update profile
   - View other user's profile
   - Start DM from profile

---

## Design Decisions

1. **DM ordering in sidebar**: Most recent message first (Slack-style)
   - Requires: Track `last_message_at` on rooms

2. **Empty DM rooms**: Persist even if no messages sent
   - Simpler implementation, cleanup can be added later if needed

3. **Leave DM behavior**: 
   - Group DMs (3+ members): Users can leave
   - 1:1 DMs: Cannot leave (would need moderation/block feature instead)

4. **Profile access**: Click on username in sidebar header to access "Edit Profile"

---

## File Changes Summary

### New Files
- `server/api/dm.go` - CreateDM handler
- `server/api/users.go` - ListUsers handler  
- `server/api/profile.go` - GetProfile, UpdateProfile handlers

### Modified Files
- `schema.sql` - Add room_type, display_name, status columns
- `server/protocol/protocol.go` - New types and updated existing types
- `tools/schemagen/main.go` - Register new types
- `server/api/init.go` - Separate rooms/DMs in response
- `server/client.go` - Add switch cases for new message types
- `client/src/index.ts` - DM sidebar, user picker, profile UI
- `client/src/types.ts` - Re-export new types
- CSS files - Styles for new UI components
