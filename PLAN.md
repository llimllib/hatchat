# Hatchat Development Plan

## Current State

### What Works

- User registration and login (password + cookie sessions)
- WebSocket connection with init handshake
- Multiple rooms with proper room-scoped message routing
- Message history with cursor-based pagination
- Room switching via sidebar without page reload
- Messages cached per room with scroll position restoration
- Message timestamps, user avatars, and message grouping
- JSON Schema generation for WebSocket protocol (`server/protocol/protocol.go` → `schema/protocol.json`)
- TypeScript type generation from schema (`client/src/protocol.generated.ts`)

### Architecture Highlights

- **Backend**: Go 1.25+, SQLite with WAL mode, gorilla/websocket
- **Frontend**: Vanilla TypeScript, esbuild bundling
- **Protocol**: WebSocket for real-time, envelope pattern (`{type, data}`)
- **Schema**: Protocol types defined in Go, generate JSON Schema + TypeScript

---

## Immediate Tasks: Schema Migration & Cleanup

_Goal: Complete the JSON Schema infrastructure work_

### 0.1 Migrate Client to Generated Types

- [x] Update `client/src/types.ts` to re-export from `protocol.generated.ts`
- [x] Keep only client-specific types in `types.ts` (e.g., `PendingMessage`)
- [x] Update all imports throughout the client codebase
- [x] Remove duplicated type definitions

### 0.2 Add Required Field Tags

- [x] Review `server/protocol/protocol.go` and add `jsonschema:"required"` tags
- [x] Regenerate schema with `just client-types`
- [x] Verify TypeScript types are now stricter (non-optional where appropriate)

### 0.3 Server API Type Alignment

- [x] Update `server/api/*.go` handlers to use `protocol.*` types directly
- [x] Remove or consolidate duplicate type definitions in `apimodels/` (removed entirely)
- [x] Ensure JSON field names match protocol spec exactly

### 0.4 Protocol Documentation

- [x] Generate HTML docs from `schema/protocol.json` using `json-schema-for-humans`
- [x] Add `just site` command to justfile
- [x] GitHub Actions workflow to deploy docs to GitHub Pages
- [x] Fix empty protocol schema page (added `anyOf` refs at root level so definitions render)

### 0.5 Runtime Type Validation

- [x] Add Zod (or similar) for runtime validation of WebSocket messages
- [x] Generate Zod schemas from JSON Schema or define alongside protocol types
- [x] Validate incoming messages in client with clear error messages on mismatch
- [ ] Consider server-side validation of client messages as well

---

## Phase 1: Room Management

_Goal: Users can create, join, and manage rooms_

### 1.1 Room Switching Protocol

- [x] Add `join_room` WebSocket message type (backend)
- [x] Update user's `last_room` when switching
- [x] Add protocol types to `server/protocol/protocol.go`

### 1.2 Join Room

- [x] Implement `join_room` WebSocket handler (auto-joins public rooms)
- [x] Client UI for joining rooms user isn't a member of

### 1.3 Create Room

- [x] "Create Channel" button in sidebar
- [x] Modal/dialog for room creation
- [x] Room name input with validation
- [x] Public/private toggle
- [x] Backend: WebSocket endpoint for room creation (`create_room`)

### 1.4 Room Discovery

- [x] "Browse Channels" view
- [x] List public rooms user isn't a member of
- [x] Join button for each room
- [ ] Search/filter rooms by name

### 1.5 Room Settings & Members

- [ ] Room info panel (name, created date, member count)
- [ ] Leave room option
- [ ] View member list for current room
- [ ] Click member to view profile or start DM

### 1.6 REST API + OpenAPI

- [ ] Define OpenAPI 3.0 spec for REST endpoints
- [ ] REST endpoints for room CRUD operations
- [ ] REST endpoint for user profile

---

## Phase 2: Direct Messages & User Features

_Goal: Private conversations and user identity_

### 2.1 Direct Messages

- [ ] Create DM "room" between two users (or find existing)
- [ ] DM rooms are private, exactly 2 members
- [ ] DM section in sidebar (separate from rooms)
- [ ] DM room naming (show other user's name)
- [ ] "New Message" button with user picker/autocomplete

### 2.2 User Presence

- [ ] Track online/offline status based on WebSocket connection
- [ ] Broadcast presence changes to relevant users
- [ ] Show presence indicators in UI (green/gray dot)
- [ ] "Last seen" timestamp for offline users

### 2.3 User Profiles

- [ ] Display name (separate from username)
- [ ] Avatar upload/URL
- [ ] Status message (custom text)
- [ ] Profile viewing panel (click username)
- [ ] "Message" button to start DM from profile

### 2.4 Typing Indicators

- [ ] Client sends "typing" events (debounced)
- [ ] Server broadcasts to room members
- [ ] UI shows "X is typing..." indicator
- [ ] Auto-clear after timeout
- [ ] Handle multiple users typing

---

## Phase 3: Rich Messaging

_Goal: Messages beyond plain text_

### 3.1 Message Formatting

- [ ] Markdown support (bold, italic, code, links)
- [ ] Code blocks with syntax highlighting
- [ ] Auto-link URLs (open in new tab)
- [ ] Emoji shortcodes (`:smile:` → 😄)
- [ ] Emoji picker in input area
- [ ] Sanitize HTML to prevent XSS

### 3.2 Message Editing & Deletion

- [ ] Edit own messages (inline editing UI)
- [ ] Show "(edited)" indicator after edit
- [ ] Delete own messages with confirmation
- [ ] Broadcast edits/deletions to room
- [ ] Handle edit/delete broadcasts in client

### 3.3 Reactions

- [ ] Add emoji reaction to message
- [ ] Remove own reaction
- [ ] Aggregate reaction counts below message
- [ ] Show who reacted on hover
- [ ] Toggle own reaction by clicking existing

### 3.4 File Uploads & Attachments

- [ ] File upload endpoint (size limits, type restrictions)
- [ ] Upload button in message input
- [ ] Drag and drop onto chat area
- [ ] Paste image from clipboard
- [ ] Upload progress indicator
- [ ] Inline image preview / file attachment card
- [ ] Store files locally or S3-compatible storage

---

## Phase 4: Threads & Organization

_Goal: Better conversation organization_

### 4.1 Threaded Replies

- [ ] Reply to specific message (creates thread)
- [ ] Thread panel slides in from right
- [ ] Shows parent message + replies
- [ ] Thread reply count on parent message
- [ ] "Also send to channel" option

### 4.2 Mentions

- [ ] @username autocomplete (trigger on `@`)
- [ ] @room (notify all room members)
- [ ] #channel autocomplete
- [ ] Style mentions differently in messages
- [ ] Highlight messages where user is mentioned
- [ ] Make mentions clickable

### 4.3 Pinned Messages

- [ ] Pin important messages in room
- [ ] View pinned messages list
- [ ] Unpin messages

### 4.4 Search

- [ ] Global search input in header (Cmd/Ctrl+K)
- [ ] Full-text search across messages (SQLite FTS5)
- [ ] Filter by room, user, date range
- [ ] Search result context (surrounding messages)
- [ ] Click to jump to message in context
- [ ] Highlight search terms

### 4.5 Message Permalinks

- [ ] Generate unique permalink URL for each message
- [ ] Clicking permalink navigates to room and scrolls to message in context
- [ ] Highlight the linked message briefly
- [ ] Copy permalink button on message hover/menu

---

## Phase 5: Notifications & Unread Tracking

_Goal: Don't miss important messages_

### 5.1 Unread Tracking

- [ ] Track last-read message per user per room
- [ ] Unread count badges in sidebar
- [ ] Bold room name if unread
- [ ] "New messages" divider line at first unread
- [ ] "Mark as read" on room view
- [ ] "Mark all as read" option

### 5.2 In-App Notifications

- [ ] Notification preferences per room (all, mentions, none)
- [ ] Toast notifications for new messages

### 5.3 Browser Notifications

- [ ] Request notification permission
- [ ] Notifications when tab not focused
- [ ] Click notification to jump to message
- [ ] Sound notifications (optional, with user setting)

### 5.4 Email Notifications (Optional)

- [ ] Email for mentions when offline
- [ ] Daily digest option
- [ ] Notification preferences

---

## Phase 6: Administration & Security

_Goal: Workspace management and hardening_

### 6.1 Password Reset

- [ ] "Forgot password" flow
- [ ] Email-based reset tokens
- [ ] Token expiration

### 6.2 Session Management

- [ ] Logout endpoint (clear session, redirect to login with message)
- [ ] Logout button in UI
- [ ] List active sessions
- [ ] Revoke sessions
- [ ] Session expiration and refresh

### 6.3 Room Administration

- [ ] Room owners/admins
- [ ] Kick/ban users from room
- [ ] Room settings (who can post, invite-only, etc.)
- [ ] Archive/delete room

### 6.4 Workspace Administration

- [ ] Admin user role
- [ ] User management (deactivate, delete)
- [ ] Workspace settings
- [ ] Usage statistics

### 6.5 Rate Limiting & Abuse Prevention

- [ ] Message rate limiting
- [ ] Connection rate limiting
- [ ] Report message/user functionality

### 6.6 Security Checklist

- [ ] Room membership checked on every message ✓ (done)
- [ ] Rate limiting on all endpoints
- [ ] Input sanitization (XSS prevention)
- [ ] SQL injection prevention (parameterized queries via dbtpl) ✓
- [ ] CSRF protection for REST endpoints
- [ ] Secure session cookie settings (HttpOnly, Secure, SameSite)

---

## Phase 7: Polish & Accessibility

_Goal: Professional, accessible experience_

### 7.1 Keyboard Navigation

- [ ] Tab navigation through UI
- [ ] Arrow keys in lists
- [ ] Escape to close modals/panels
- [ ] Focus management

### 7.2 Accessibility

- [ ] ARIA labels and roles
- [ ] Screen reader announcements for new messages
- [ ] Sufficient color contrast
- [ ] Reduced motion support

### 7.3 Responsive Design

- [ ] Mobile-friendly layout
- [ ] Collapsible sidebar on small screens
- [ ] Touch-friendly targets
- [ ] Swipe gestures (optional)

### 7.4 Loading States & Error Handling

- [ ] Skeleton loaders for content
- [ ] Connection status indicator (visual feedback when WebSocket disconnects)
- [ ] WebSocket auto-reconnection with exponential backoff
- [ ] Reconnection handling with UI feedback
- [ ] Error states and retry options

### 7.5 Sign-In Page Polish

- [ ] Restyle sign-in page to match app design
- [ ] Add "hatchat" branding and app description
- [ ] Display error messages on failed login/registration (flash messages or query params)
- [ ] Show success message after logout
- [ ] Auto-login after successful registration (skip the extra login step)

### 7.6 Visual Design

- [ ] Design new color scheme (replace Slack aubergine)
- [ ] Apply consistent color palette across app
- [ ] Update CSS variables for theming

---

## Developer Experience

_Goal: Make development faster and debugging easier_

### DX.1 Panic Stack Traces

- [ ] Add stack trace logging to panic recovery middleware
- [ ] Use `runtime/debug.Stack()` or similar to capture full traceback
- [ ] Ensure stack traces appear in development logs

### DX.2 CI Optimization

- [ ] Use path filters in GitHub Actions to run Go CI only when Go files change
- [ ] Run JS CI only when client/ files change
- [ ] Keep full CI on main branch merges

### DX.3 End-to-End Tests

- [ ] Set up Playwright for e2e testing
- [ ] Test user registration and login flow
- [ ] Test room creation and switching
- [ ] Test real-time message sending/receiving
- [ ] Add e2e tests to CI pipeline

### DX.4 Build Tooling

- [ ] Add `just clean` command to remove build artifacts (hatchat binary, node_modules, etc.)

---

## Phase 8: Advanced Features (Future)

_Nice to have, lower priority_

### 8.1 OAuth Integration

- [ ] "Sign in with Google"
- [ ] "Sign in with GitHub"
- [ ] Account linking

### 8.2 Integrations & Bots

- [ ] Incoming webhooks (post messages via HTTP)
- [ ] Outgoing webhooks (notify external services)
- [ ] Bot user accounts
- [ ] Slash commands

### 8.3 Voice & Video (Major undertaking)

- [ ] WebRTC integration
- [ ] Huddles (quick audio calls)
- [ ] Screen sharing

### 8.4 Mobile Apps

- [ ] React Native or native apps
- [ ] Push notifications via APNs/FCM

---

## Milestones

### M1: Complete Multi-Room Chat (Current → Phase 1)

- ✅ Room-scoped message routing
- ✅ Message history with pagination
- ✅ Room switching in UI
- Schema migration complete
- Create/join/leave rooms
- REST API with OpenAPI spec
- **Target: Actually usable for basic team chat**

### M2: Private Messaging (Phase 2)

- DMs working
- User presence
- Basic profiles
- Typing indicators
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
- Mentions
- Notifications
- **Target: Feature parity with Slack essentials**

### M5: Production Ready (Phases 6-7)

- Admin features
- Security hardening
- Accessibility
- **Target: Deploy for real teams**

---

## Implementation Notes

### Adding New WebSocket Message Types

1. Add type definition in `server/protocol/protocol.go`
2. Add type to `tools/schemagen/main.go` types slice
3. Run `just client-types` to regenerate schema + TypeScript
4. Add handler in `server/api/`
5. Add case in `client.go` `readPump()` switch
6. Add client-side handler using generated types

### Frontend Architecture

- Keep vanilla TypeScript (no frameworks)
- Consider Web Components for encapsulated UI elements
- Virtual scrolling for large message lists (future)
- Simple pub/sub or state container for app state

### Database Considerations

- Add indexes as query patterns emerge
- Consider FTS5 virtual table for search early
- May need message archival strategy for large deployments

### Testing Strategy

- Unit tests for utility functions and handlers
- Integration tests for full request lifecycle
- Load testing for Hub broadcast performance
- `just test` must pass before marking work complete
