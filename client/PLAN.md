# Hatchat Frontend Development Plan

## Current State

### What Exists

- Basic WebSocket client with init/message/history handling
- Simple DOM helper function `$()` for element creation
- Message sending with optimistic UI updates
- Message history loading with pagination ("Load more" button)
- Static HTML layout with sidebar placeholder and chat area
- Basic Slack-inspired CSS styling (purple sidebar, white chat area)

### What's Missing

The frontend is currently a minimal proof-of-concept. To approach Slack parity, we need significant work on:

- **Room/Channel Management**: Sidebar shows hardcoded placeholder channels
- **Room Switching**: No ability to switch rooms without page reload
- **User Presence**: No online/offline indicators
- **Direct Messages**: No DM support in UI
- **Rich Messages**: Plain text only, no formatting or attachments
- **User Experience**: No typing indicators, reactions, threads, search
- **Notifications**: No unread counts, no browser notifications
- **Profile/Settings**: No user profile view or settings

---

## Architecture Decisions

### Keep Vanilla TypeScript

Per AGENTS.md, we're staying close to the web platform:
- No React/Vue/Svelte frameworks
- Use modern browser APIs (Web Components when beneficial)
- Keep bundle size small

### File Structure (Proposed)

```
client/
├── src/
│   ├── index.ts           # Entry point, app initialization
│   ├── client.ts          # WebSocket client class
│   ├── components/        # UI components (Web Components or classes)
│   │   ├── sidebar.ts
│   │   ├── channel-list.ts
│   │   ├── message-list.ts
│   │   ├── message-input.ts
│   │   ├── message.ts
│   │   └── user-presence.ts
│   ├── state.ts           # Application state management
│   ├── types.ts           # TypeScript interfaces
│   └── utils.ts           # DOM helpers, formatting utilities
├── PLAN.md
└── ...
```

### State Management

Simple pub/sub pattern or a minimal state container:
- Current user
- Current room
- Room list (channels + DMs)
- Messages per room (with pagination state)
- Online users
- Unread counts

---

## Feature Roadmap

### Phase 1: Core Navigation & Room Switching

_Goal: Users can navigate between rooms without page reload_

#### 1.1 Dynamic Sidebar

- [x] Render actual room list from `init` response
- [x] Show room names with `#` prefix for channels
- [x] Highlight currently active room
- [x] Click room to switch (without page reload)

#### 1.2 Room Switching via WebSocket

- [ ] Implement `join_room` message type handling
- [x] Clear message area when switching rooms
- [x] Load history for new room
- [x] Update URL using History API (`/chat/{room_id}`)
- [x] Update chat header with room name

#### 1.3 Room State Management

- [ ] Extract state into dedicated module
- [ ] Cache messages per room (avoid re-fetching on switch back)
- [ ] Track scroll position per room
- [ ] Restore scroll position when returning to room

#### 1.4 Better Message Display

- [ ] Show timestamps on messages
- [ ] Group consecutive messages from same user
- [ ] Add user avatars (placeholder or initials)
- [ ] Distinguish own messages visually

---

### Phase 2: Room Management UI

_Goal: Users can create and manage rooms_

#### 2.1 Create Room

- [ ] "Create Channel" button in sidebar
- [ ] Modal/dialog for room creation
- [ ] Room name input with validation
- [ ] Public/private toggle
- [ ] Submit via REST API, then join via WebSocket

#### 2.2 Room Discovery

- [ ] "Browse Channels" view
- [ ] List public rooms user isn't a member of
- [ ] Join button for each room
- [ ] Search/filter rooms by name

#### 2.3 Room Settings

- [ ] Room info panel (name, created date, member count)
- [ ] Leave room option
- [ ] Edit room name (if permitted)

#### 2.4 Room Members

- [ ] View member list for current room
- [ ] Show member online status
- [ ] Click member to view profile or start DM

---

### Phase 3: Direct Messages

_Goal: Private 1:1 conversations_

#### 3.1 DM Section in Sidebar

- [ ] Separate "Direct Messages" section
- [ ] List existing DM conversations
- [ ] Show other user's name and avatar
- [ ] Online indicator dot

#### 3.2 Start New DM

- [ ] "New Message" button or user search
- [ ] User picker/autocomplete
- [ ] Create DM room if doesn't exist
- [ ] Navigate to DM conversation

#### 3.3 DM Conversation View

- [ ] Same message UI as channels
- [ ] Header shows user name instead of room name
- [ ] User profile link in header

---

### Phase 4: User Presence & Profiles

_Goal: See who's online, view user info_

#### 4.1 Presence Indicators

- [ ] Handle presence WebSocket messages
- [ ] Green dot for online users in sidebar
- [ ] Gray/hollow dot for offline
- [ ] Update indicators in real-time

#### 4.2 User Profile Panel

- [ ] Click username to view profile
- [ ] Slide-out panel or modal
- [ ] Show: avatar, display name, username, status
- [ ] "Message" button to start DM
- [ ] "Last seen" for offline users

#### 4.3 Own Profile Settings

- [ ] Edit display name
- [ ] Upload/change avatar
- [ ] Set status message
- [ ] Logout button

---

### Phase 5: Typing Indicators

_Goal: See when others are typing_

#### 5.1 Send Typing Events

- [ ] Debounce keystrokes in message input
- [ ] Send `typing` message to server
- [ ] Stop sending after typing stops (timeout)

#### 5.2 Display Typing Indicators

- [ ] Handle `typing` WebSocket messages
- [ ] Show "X is typing..." below message list
- [ ] Handle multiple users typing
- [ ] Auto-clear after timeout

---

### Phase 6: Rich Message Formatting

_Goal: Messages beyond plain text_

#### 6.1 Markdown Rendering

- [ ] Parse message body as Markdown
- [ ] Support: bold, italic, strikethrough, code
- [ ] Inline code with backticks
- [ ] Code blocks with triple backticks
- [ ] Sanitize HTML to prevent XSS

#### 6.2 Link Handling

- [ ] Auto-detect and linkify URLs
- [ ] Open links in new tab
- [ ] Link previews (optional, requires backend)

#### 6.3 Emoji Support

- [ ] Emoji picker button in input area
- [ ] Convert shortcodes (`:smile:` → 😄)
- [ ] Native emoji input support

---

### Phase 7: Message Actions

_Goal: Edit, delete, and react to messages_

#### 7.1 Message Hover Actions

- [ ] Show action buttons on message hover
- [ ] Actions: React, Reply, Edit (own), Delete (own), More
- [ ] Keyboard shortcuts for common actions

#### 7.2 Edit Messages

- [ ] Click edit to enter edit mode
- [ ] Inline editing in message
- [ ] Save/cancel buttons
- [ ] Show "(edited)" indicator after edit
- [ ] Handle edit broadcasts from server

#### 7.3 Delete Messages

- [ ] Confirmation dialog
- [ ] Remove message from UI
- [ ] Handle delete broadcasts (show "message deleted")

#### 7.4 Reactions

- [ ] Emoji reaction picker
- [ ] Add/remove reactions
- [ ] Display reaction counts below message
- [ ] Show who reacted on hover
- [ ] Toggle own reaction by clicking existing

---

### Phase 8: Threads

_Goal: Organize conversations_

#### 8.1 Reply to Message

- [ ] "Reply in thread" action on messages
- [ ] Thread panel slides in from right
- [ ] Shows parent message + replies
- [ ] Thread input at bottom

#### 8.2 Thread Indicators

- [ ] Show reply count on parent message
- [ ] "X replies" link to open thread
- [ ] Show last reply preview

#### 8.3 Thread Navigation

- [ ] Thread panel can be collapsed
- [ ] Multiple threads can be opened (tabs?)
- [ ] Thread messages update in real-time

---

### Phase 9: Mentions & Autocomplete

_Goal: Notify specific users_

#### 9.1 @mention Autocomplete

- [ ] Trigger on `@` character in input
- [ ] Show dropdown of matching users
- [ ] Keyboard navigation (up/down/enter)
- [ ] Insert formatted mention

#### 9.2 Mention Highlighting

- [ ] Style @mentions differently in messages
- [ ] Highlight messages where current user is mentioned
- [ ] Make mentions clickable (show profile)

#### 9.3 #channel Mentions

- [ ] Autocomplete channel names with `#`
- [ ] Link to channel when clicked

---

### Phase 10: Notifications & Unread

_Goal: Don't miss important messages_

#### 10.1 Unread Counts

- [ ] Track last-read message per room
- [ ] Show unread count badge in sidebar
- [ ] Bold room name if unread
- [ ] Mark room as read when viewed

#### 10.2 Unread Line

- [ ] Show "New messages" divider line
- [ ] Position at first unread message
- [ ] Clear line after viewing

#### 10.3 Browser Notifications

- [ ] Request notification permission
- [ ] Show notification for new messages (when tab not focused)
- [ ] Click notification to focus tab and go to message
- [ ] Notification preferences per room

#### 10.4 Sound Notifications

- [ ] Play sound for new messages (optional)
- [ ] User setting to enable/disable
- [ ] Different sounds for mentions vs regular

---

### Phase 11: Search

_Goal: Find past messages_

#### 11.1 Search Input

- [ ] Global search in header
- [ ] Keyboard shortcut (Cmd/Ctrl+K)
- [ ] Search as you type (debounced)

#### 11.2 Search Results

- [ ] Display matching messages
- [ ] Show context (room name, timestamp)
- [ ] Click to jump to message in context
- [ ] Highlight search terms

#### 11.3 Search Filters

- [ ] Filter by room
- [ ] Filter by user
- [ ] Filter by date range
- [ ] Filter by has:file, has:link, etc.

---

### Phase 12: File Sharing

_Goal: Share images and files_

#### 12.1 File Upload UI

- [ ] Upload button in message input
- [ ] Drag and drop onto chat area
- [ ] Paste image from clipboard
- [ ] Upload progress indicator

#### 12.2 File Display

- [ ] Inline image preview
- [ ] File attachment card (icon, name, size)
- [ ] Download link
- [ ] Lightbox for images

---

### Phase 13: Polish & Accessibility

_Goal: Professional, accessible experience_

#### 13.1 Keyboard Navigation

- [ ] Tab navigation through UI
- [ ] Arrow keys in lists
- [ ] Escape to close modals/panels
- [ ] Focus management

#### 13.2 Accessibility

- [ ] ARIA labels and roles
- [ ] Screen reader announcements for new messages
- [ ] Sufficient color contrast
- [ ] Reduced motion support

#### 13.3 Responsive Design

- [ ] Mobile-friendly layout
- [ ] Collapsible sidebar on small screens
- [ ] Touch-friendly targets
- [ ] Swipe gestures (optional)

#### 13.4 Loading States

- [ ] Skeleton loaders for content
- [ ] Connection status indicator
- [ ] Reconnection handling with UI feedback
- [ ] Error states and retry options

---

## Implementation Notes

### Component Patterns

Consider Web Components for encapsulation:

```typescript
class MessageElement extends HTMLElement {
  connectedCallback() { ... }
  static get observedAttributes() { return ['username', 'body']; }
  attributeChangedCallback() { ... }
}
customElements.define('hc-message', MessageElement);
```

Or simpler class-based components with render methods.

### Performance Considerations

- Virtual scrolling for large message lists
- Debounce rapid updates
- Lazy load older messages
- Image lazy loading
- Efficient DOM updates (batch changes)

### Testing Strategy

- Unit tests for utility functions
- Component tests with jsdom or Playwright
- E2E tests for critical flows (send message, switch room)

---

## Milestones

### M1: Functional Multi-Room Chat (Phases 1-2)

- Dynamic room list
- Room switching without reload
- Create/join/leave rooms
- **Target: Actually usable for basic team chat**

### M2: Private Messaging (Phases 3-4)

- Direct messages working
- User presence indicators
- Basic profiles
- **Target: Can replace Slack for simple use cases**

### M3: Interactive Messages (Phases 5-7)

- Typing indicators
- Markdown formatting
- Edit/delete messages
- Reactions
- **Target: Comfortable daily driver**

### M4: Advanced Features (Phases 8-10)

- Threaded conversations
- @mentions with autocomplete
- Unread tracking and notifications
- **Target: Feature parity with Slack essentials**

### M5: Full Featured (Phases 11-13)

- Search
- File sharing
- Polish and accessibility
- **Target: Production-quality chat application**

---

## Next Steps

Start with **Phase 1.1: Dynamic Sidebar** since it's foundational:

1. Extract room list rendering from hardcoded HTML
2. Handle `init` response to populate sidebar
3. Add click handlers for room selection
4. Implement visual highlight for active room

This unblocks all subsequent room-related features.
