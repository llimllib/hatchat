# Hatchat Development Plan

## Current State

### What Works

- User registration and login (password + cookie sessions)
- Basic WebSocket connection with init handshake
- Single default room created on startup
- Messages can be sent and are stored in database
- Messages broadcast to all connected clients (not room-scoped)
- Basic chat UI with sidebar placeholder

### What's Missing or Broken

- Messages broadcast to ALL clients regardless of room membership (security issue)
- No message history loaded on room entry
- No way to create/join/leave rooms
- No direct messages
- UI doesn't reflect actual rooms or update properly
- No OpenAPI spec or REST API

---

## Feature Roadmap

### Phase 1: Core Messaging Foundation

_Goal: A functional, secure chat with proper room isolation_

#### 1.1 Room-Scoped Message Routing (Critical Security Fix)

- [x] Track which room each client is currently viewing in the Hub
- [x] Broadcast messages only to clients in the same room
- [x] Add room membership validation before accepting messages

#### 1.1a Testing Room-scoped message routing

- [ ] Add tests that verify that users only receive messages for the rooms they belong to
  - Test this thoroughly, as it's an important security feature

#### 1.1b dependency upgrades

- Upgrade go to the latest version everywhere
- Upgrade all node packages and node version to the latest versions

#### 1.2 Message History

- [ ] Add API endpoint/WebSocket message to fetch room message history
- [ ] Pagination support (cursor-based, load older messages)
- [ ] Load history when entering a room

#### 1.3 Room Switching

- [ ] WebSocket message type for switching rooms ("join_room")
- [ ] Update user's `last_room` when switching
- [ ] Client UI to switch between rooms in sidebar

#### 1.4 Room Management

- [ ] Create new room (name, public/private)
- [ ] List available rooms (public rooms + rooms user is member of)
- [ ] Join public room
- [ ] Leave room
- [ ] Room details (name, member count, created date)

#### 1.5 Basic REST API + OpenAPI

- [ ] Define OpenAPI 3.0 spec for all endpoints
- [ ] REST endpoints for room CRUD operations
- [ ] REST endpoint for user profile
- [ ] Document WebSocket message protocol

---

### Phase 2: Direct Messages & User Features

_Goal: Private conversations and user identity_

#### 2.1 Direct Messages

- [ ] Create DM "room" between two users (or find existing)
- [ ] DM rooms are private, exactly 2 members
- [ ] List DM conversations in sidebar separately from rooms
- [ ] DM room naming (show other user's name, not room name)

#### 2.2 User Presence

- [ ] Track online/offline status based on WebSocket connection
- [ ] Broadcast presence changes to relevant users
- [ ] Show presence indicators in UI
- [ ] "Last seen" timestamp for offline users

#### 2.3 User Profiles

- [ ] Display name (separate from username)
- [ ] Avatar upload/URL
- [ ] Status message (custom text)
- [ ] Profile viewing UI

#### 2.4 Typing Indicators

- [ ] Client sends "typing" events (debounced)
- [ ] Server broadcasts to room members
- [ ] UI shows "X is typing..." indicator
- [ ] Auto-clear after timeout

---

### Phase 3: Rich Messaging

_Goal: Messages beyond plain text_

#### 3.1 Message Formatting

- [ ] Markdown support (bold, italic, code, links)
- [ ] Code blocks with syntax highlighting
- [ ] Auto-link URLs
- [ ] Emoji shortcodes (:smile: → 😄)

#### 3.2 Message Editing & Deletion

- [ ] Edit own messages (within time limit?)
- [ ] Delete own messages
- [ ] Show edit indicator and edit history
- [ ] Broadcast edits/deletions to room

#### 3.3 Reactions

- [ ] Add emoji reaction to message
- [ ] Remove own reaction
- [ ] Aggregate reaction counts
- [ ] Show who reacted (on hover)

#### 3.4 File Uploads & Attachments

- [ ] File upload endpoint (size limits, type restrictions)
- [ ] Store files locally or S3-compatible storage
- [ ] Image preview in chat
- [ ] File download links
- [ ] Image paste from clipboard

---

### Phase 4: Threads & Organization

_Goal: Better conversation organization_

#### 4.1 Threaded Replies

- [ ] Reply to specific message (creates thread)
- [ ] Thread view (slide-out panel or modal)
- [ ] Thread reply count shown on parent message
- [ ] "Also send to channel" option

#### 4.2 Mentions

- [ ] @username mentions with autocomplete
- [ ] @room (notify all room members)
- [ ] Highlight messages where user is mentioned
- [ ] Mention notifications

#### 4.3 Pinned Messages

- [ ] Pin important messages in room
- [ ] View pinned messages list
- [ ] Unpin messages

#### 4.4 Search

- [ ] Full-text search across messages
- [ ] Filter by room, user, date range
- [ ] Search result context (surrounding messages)
- [ ] SQLite FTS5 for search index

---

### Phase 5: Notifications & Polish

_Goal: Don't miss important messages_

#### 5.1 Unread Tracking

- [ ] Track last-read message per user per room
- [ ] Unread count badges in sidebar
- [ ] "Mark as read" on room view
- [ ] "Mark all as read" option

#### 5.2 In-App Notifications

- [ ] Notification preferences per room (all, mentions, none)
- [ ] Toast notifications for new messages
- [ ] Sound notifications (optional)

#### 5.3 Browser Notifications

- [ ] Push notification permission request
- [ ] Notifications when tab not focused
- [ ] Click notification to jump to message

#### 5.4 Email Notifications (Optional)

- [ ] Email for mentions when offline
- [ ] Daily digest option
- [ ] Notification preferences

---

### Phase 6: Administration & Security

_Goal: Workspace management and hardening_

#### 6.1 Password Reset

- [ ] "Forgot password" flow
- [ ] Email-based reset tokens
- [ ] Token expiration

#### 6.2 Session Management

- [ ] List active sessions
- [ ] Revoke sessions
- [ ] Session expiration and refresh

#### 6.3 Room Administration

- [ ] Room owners/admins
- [ ] Kick/ban users from room
- [ ] Room settings (who can post, invite-only, etc.)
- [ ] Archive/delete room

#### 6.4 Workspace Administration

- [ ] Admin user role
- [ ] User management (deactivate, delete)
- [ ] Workspace settings
- [ ] Usage statistics

#### 6.5 Rate Limiting & Abuse Prevention

- [ ] Message rate limiting
- [ ] Connection rate limiting
- [ ] Report message/user functionality

---

### Phase 7: Advanced Features (Future)

_Nice to have, lower priority_

#### 7.1 OAuth Integration

- [ ] "Sign in with Google"
- [ ] "Sign in with GitHub"
- [ ] Account linking

#### 7.2 Integrations & Bots

- [ ] Incoming webhooks (post messages via HTTP)
- [ ] Outgoing webhooks (notify external services)
- [ ] Bot user accounts
- [ ] Slash commands

#### 7.3 Voice & Video (Major undertaking)

- [ ] WebRTC integration
- [ ] Huddles (quick audio calls)
- [ ] Screen sharing

#### 7.4 Mobile Apps

- [ ] React Native or native apps
- [ ] Push notifications via APNs/FCM

---

## Implementation Notes

### API Strategy

- WebSocket for real-time: messages, presence, typing indicators
- REST for CRUD operations: rooms, users, file uploads
- All REST endpoints documented in OpenAPI spec
- Consider GraphQL for complex queries (search, history) - evaluate later

### Database Considerations

- Add indexes as query patterns emerge
- Consider FTS5 virtual table for search early
- May need message archival strategy for large deployments

### Testing Strategy

- Unit tests for API handlers
- Integration tests for WebSocket flows
- Load testing for Hub broadcast performance

### Security Checklist

- [ ] Room membership checked on every message
- [ ] Rate limiting on all endpoints
- [ ] Input sanitization (XSS prevention)
- [ ] SQL injection prevention (parameterized queries via xo)
- [ ] CSRF protection for REST endpoints
- [ ] Secure session cookie settings (HttpOnly, Secure, SameSite)

---

## Milestones

### M1: Secure Room Chat (Phase 1)

- Multiple rooms working correctly
- Message history loads
- Messages only go to room members
- REST API with OpenAPI spec
- **Target: Usable for small team internal chat**

### M2: Private Messaging (Phase 2)

- DMs working
- User presence
- Basic profiles
- **Target: Replace simple Slack workspace**

### M3: Rich Chat (Phase 3)

- Markdown formatting
- Edit/delete messages
- Reactions
- File sharing
- **Target: Comfortable daily driver**

### M4: Full Featured (Phases 4-5)

- Threads
- Search
- Notifications
- **Target: Feature parity with Slack essentials**

### M5: Production Ready (Phase 6)

- Admin features
- Security hardening
- **Target: Deploy for real teams**
