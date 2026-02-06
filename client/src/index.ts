import { Autocomplete } from "./autocomplete";
import { $ } from "./dom";
import { containsMention, renderMarkdown } from "./markdown";
import { AppState } from "./state";
import {
  type CreateDMResponse,
  type CreateRoomResponse,
  type GetMessageContextResponse,
  type GetProfileResponse,
  type HistoryResponse,
  type InitResponse,
  type JoinRoomResponse,
  type LeaveRoomResponse,
  type ListRoomsResponse,
  type ListUsersResponse,
  type Message,
  type MessageDeleted,
  type MessageEdited,
  makePendingKey,
  type PendingMessage,
  parseServerEnvelope,
  type Reaction,
  type ReactionUpdated,
  type Room,
  type RoomInfoResponse,
  type SearchResponse,
  type SearchResult,
  type UpdateProfileResponse,
  type User,
} from "./types";
import {
  formatDate,
  formatTimestamp,
  formatTimestampFull,
  getInitials,
  stringToColor,
} from "./utils";

// Local storage key for recent rooms
const RECENT_ROOMS_KEY = "hatchat:recent_rooms";
const MAX_RECENT_ROOMS = 8;

/**
 * Quick-search result item for display
 */
interface QuickSearchItem {
  type: "room" | "dm" | "user" | "search-escape";
  id: string;
  name: string;
  secondary?: string; // e.g., @username for users with display name
  isMember?: boolean; // For rooms: whether user is already a member
}

class Client {
  conn: WebSocket;
  state: AppState;

  // Track pending messages waiting for server confirmation
  pendingMessages: Map<string, PendingMessage> = new Map();

  // Track loading state
  isLoadingHistory: boolean = false;

  // Autocomplete for @mentions and #channels
  autocomplete: Autocomplete | null = null;

  // Search state
  isSearchPage: boolean = false;
  searchResults: SearchResult[] = [];
  searchNextCursor: string = "";
  isSearching: boolean = false;

  // Permalink jump target
  pendingPermalinkMessageId: string | null = null;

  // Quick-search state
  quickSearchOpen: boolean = false;
  quickSearchSelectedIndex: number = 0;
  quickSearchItems: QuickSearchItem[] = [];
  quickSearchQuery: string = "";
  quickSearchUsers: User[] = []; // Cached users for quick-search
  quickSearchAllRooms: Room[] = []; // All accessible rooms (not just joined)
  quickSearchRoomMembership: boolean[] = []; // Membership status for quickSearchAllRooms

  constructor(conn: WebSocket) {
    this.conn = conn;
    this.state = new AppState();

    conn.addEventListener("open", this.wsOpen.bind(this));
    conn.addEventListener("message", this.wsReceive.bind(this));
    conn.addEventListener("close", this.wsClose.bind(this));
  }

  wsClose(_: CloseEvent) {
    // TODO: try to reconnect
    console.warn("connection closed", _);
  }

  wsReceive(evt: MessageEvent) {
    if (!evt.data) {
      console.debug("unable to process empty message", evt);
      return;
    }
    try {
      const raw = JSON.parse(evt.data);
      const envelope = parseServerEnvelope(raw);
      switch (envelope.type) {
        case "init": {
          this.handleInit(envelope.data);
          break;
        }
        case "history": {
          this.handleHistory(envelope.data);
          break;
        }
        case "message": {
          // Handle incoming message - could be from us (confirmation) or others
          this.handleIncomingMessage(envelope.data);
          break;
        }
        case "join_room": {
          this.handleJoinRoom(envelope.data);
          break;
        }
        case "create_room": {
          this.handleCreateRoom(envelope.data);
          break;
        }
        case "list_rooms": {
          this.handleListRooms(envelope.data);
          break;
        }
        case "leave_room": {
          this.handleLeaveRoom(envelope.data);
          break;
        }
        case "room_info": {
          this.handleRoomInfo(envelope.data);
          break;
        }
        case "create_dm": {
          this.handleCreateDM(envelope.data);
          break;
        }
        case "list_users": {
          this.handleListUsers(envelope.data);
          break;
        }
        case "get_profile": {
          this.handleGetProfile(envelope.data);
          break;
        }
        case "update_profile": {
          this.handleUpdateProfile(envelope.data);
          break;
        }
        case "message_edited": {
          this.handleMessageEdited(envelope.data);
          break;
        }
        case "message_deleted": {
          this.handleMessageDeleted(envelope.data);
          break;
        }
        case "reaction_updated": {
          this.handleReactionUpdated(envelope.data);
          break;
        }
        case "search": {
          this.handleSearch(envelope.data);
          break;
        }
        case "get_message_context": {
          this.handleGetMessageContext(envelope.data);
          break;
        }
        case "error": {
          console.error("server error:", envelope.data);
          break;
        }
      }
      console.debug("received: ", envelope);
    } catch (e) {
      console.error("unable to parse or validate message", evt.data, e);
    }
  }

  handleInit(data: InitResponse) {
    this.state.setInitialData(data);

    // Check if we're on the search page
    if (window.location.pathname === "/search") {
      this.renderSearchPage();
      this.populateSearchRoomFilter();
      return;
    }

    // Get room ID from URL or use the current_room from init
    const parts = window.location.pathname.split("/");
    const urlRoomID = parts[parts.length - 1];
    this.state.setCurrentRoom(urlRoomID || data.current_room);

    // Render the sidebar with rooms
    this.renderSidebar();

    // Initialize autocomplete for message input
    this.initAutocomplete();

    // Check for permalink hash (e.g., #msg_abc123)
    const hash = window.location.hash.slice(1);
    if (hash?.startsWith("msg_")) {
      this.pendingPermalinkMessageId = hash;
    }

    // Request history for the current room
    if (this.state.currentRoom) {
      this.requestHistory(this.state.currentRoom);
      this.updateChatHeader();
    }
  }

  /**
   * Initialize the autocomplete for @mentions and #channel references
   */
  initAutocomplete() {
    const messageBox = document.querySelector(
      "#message",
    ) as HTMLTextAreaElement;
    if (!messageBox) return;

    this.autocomplete = new Autocomplete({
      input: messageBox,
      onQueryUsers: (query: string) => {
        this.requestListUsers(query);
      },
      onSelectChannel: (channelName: string) => {
        console.debug("selected channel mention:", channelName);
      },
    });

    // Set available channels
    this.updateAutocompleteChannels();

    // Set up click delegation for mention spans in the message area
    const messageWindow = document.querySelector(".chat-messages");
    if (messageWindow) {
      messageWindow.addEventListener("click", (e) => {
        const target = e.target as HTMLElement;
        if (!target.classList.contains("mention")) return;

        const mentionType = target.getAttribute("data-mention-type");
        const mentionName = target.getAttribute("data-mention-name");
        if (!mentionType || !mentionName) return;

        if (mentionType === "user") {
          // Find user by username and open their profile
          this.handleMentionUserClick(mentionName);
        } else if (mentionType === "channel") {
          // Find room by name and switch to it
          this.handleMentionChannelClick(mentionName);
        }
      });
    }
  }

  /**
   * Handle click on a @username mention
   */
  handleMentionUserClick(username: string) {
    // We need the user ID to request their profile, but we only have the username.
    // Search for the user to get their ID
    // First check if we know this user from current room members
    const roomState = this.state.getCurrentRoomState();
    if (roomState) {
      for (const msg of roomState.messages) {
        if (msg.username === username) {
          this.requestProfile(msg.user_id);
          return;
        }
      }
    }

    // If we couldn't find them locally, request user list and open profile
    // For now, request via list_users and handle the result
    // We'll use a one-shot approach: request users with the username query
    this.pendingMentionProfileLookup = username;
    this.requestListUsers(username);
  }

  // Track pending @mention profile lookups
  private pendingMentionProfileLookup: string | null = null;

  /**
   * Handle click on a #channel mention
   */
  handleMentionChannelClick(channelName: string) {
    // Find the room by name
    const room = this.state.rooms.find(
      (r) => r.name.toLowerCase() === channelName.toLowerCase(),
    );
    if (room) {
      this.switchRoom(room.id);
    } else {
      // Room not found in our list - might be one we're not a member of
      // Try to join it
      console.debug("channel not found locally, searching...", channelName);
      // Request room list to find it
      this.requestListRooms(channelName);
    }
  }

  /**
   * Update the autocomplete with current channel list
   */
  updateAutocompleteChannels() {
    if (!this.autocomplete) return;
    this.autocomplete.setChannels(this.state.rooms);
  }

  wsOpen(evt: Event) {
    console.log("opened", evt);
    this.conn.send(
      JSON.stringify({
        type: "init",
        data: {},
      }),
    );
  }

  requestHistory(roomID: string, cursor?: string) {
    if (this.isLoadingHistory) {
      return;
    }
    this.isLoadingHistory = true;

    const request = {
      type: "history",
      data: {
        room_id: roomID,
        cursor: cursor || "",
        limit: 50,
      },
    };
    console.debug("requesting history", request);
    this.conn.send(JSON.stringify(request));
  }

  handleHistory(response: HistoryResponse) {
    this.isLoadingHistory = false;

    const roomId = this.state.currentRoom;
    if (!roomId) {
      console.error("no current room set");
      return;
    }

    // Messages come in newest-first order, we need chronological
    const messages = [...response.messages].reverse();

    // Check if this is loading more (has cursor) or initial load
    const roomState = this.state.getRoomState(roomId);
    const isLoadingMore = roomState.historyCursor !== undefined;

    // Update state with messages and pagination
    this.state.addMessages(roomId, messages, isLoadingMore);
    this.state.updatePagination(
      roomId,
      response.next_cursor || undefined,
      response.has_more,
    );

    // Re-render the messages from state
    this.renderMessages();

    // If we have a pending permalink, try to jump to it now
    if (this.pendingPermalinkMessageId) {
      // Small delay to allow DOM to update
      setTimeout(() => {
        if (this.pendingPermalinkMessageId) {
          this.jumpToMessage(this.pendingPermalinkMessageId);
        }
      }, 100);
    }
  }

  /**
   * Render all messages for the current room from state
   */
  renderMessages() {
    const roomId = this.state.currentRoom;
    if (!roomId) return;

    const messageWindow = document.querySelector(".chat-messages");
    if (!messageWindow) {
      console.error("no message window found");
      return;
    }

    const roomState = this.state.getRoomState(roomId);

    // Clear and re-render
    messageWindow.innerHTML = "";

    // Add load more button if needed
    if (roomState.hasMoreHistory) {
      const loadMoreBtn = $("button", {
        text: "Load older messages",
        class: "load-more-button",
      });
      loadMoreBtn.addEventListener("click", () => {
        if (roomState.historyCursor) {
          this.requestHistory(roomId, roomState.historyCursor);
        }
      });
      messageWindow.appendChild(loadMoreBtn);
    }

    // Group and render messages
    const messages = roomState.messages;
    let lastMessage: Message | undefined;

    for (const msg of messages) {
      const isGrouped = this.shouldGroupWithPrevious(msg, lastMessage);
      const isOwn = msg.user_id === this.state.user.id;
      const element = this.createMessageElement(msg, isGrouped, isOwn);
      messageWindow.appendChild(element);
      lastMessage = msg;
    }

    // Scroll to bottom for initial load, restore position otherwise
    if (roomState.scrollPosition > 0) {
      messageWindow.scrollTop = roomState.scrollPosition;
    } else {
      messageWindow.scrollTop = messageWindow.scrollHeight;
    }
  }

  /**
   * Check if a message should be visually grouped with the previous one
   */
  shouldGroupWithPrevious(msg: Message, prevMsg: Message | undefined): boolean {
    if (!prevMsg) return false;

    // Different user - no grouping
    if (msg.user_id !== prevMsg.user_id) return false;

    // More than 5 minutes apart - no grouping
    const msgTime = new Date(msg.created_at).getTime();
    const prevTime = new Date(prevMsg.created_at).getTime();
    const fiveMinutes = 5 * 60 * 1000;
    if (msgTime - prevTime > fiveMinutes) return false;

    return true;
  }

  /**
   * Create a message element with full formatting
   */
  createMessageElement(
    msg: Message,
    isGrouped: boolean,
    isOwn: boolean,
  ): HTMLElement {
    // Handle deleted messages (tombstone)
    if (msg.deleted_at) {
      const wrapper = $("div", {
        class: "chat-message deleted",
        "data-message-id": msg.id,
      });
      const tombstone = $("div", { class: "message-content tombstone" });
      tombstone.appendChild($("em", { text: "This message was deleted." }));
      wrapper.appendChild(tombstone);
      return wrapper;
    }

    // Check if this message mentions the current user
    const isMentioned = containsMention(msg.body, this.state.user.username);
    const mentionClass = isMentioned ? " mentioned" : "";

    const wrapper = $("div", {
      class: `chat-message ${isGrouped ? "grouped" : ""} ${isOwn ? "own-message" : ""}${mentionClass}`,
      "data-message-id": msg.id,
    });

    // Build hover toolbar (shown/hidden via CSS :hover)
    this.buildHoverToolbar(wrapper);

    if (!isGrouped) {
      // Full message with avatar and header
      const avatar = this.createAvatar(msg.username);
      const header = $("div", { class: "message-header" });

      const usernameEl = $("span", {
        class: "message-username",
        text: msg.username,
      });
      // Make username clickable to view profile
      usernameEl.addEventListener("click", () => {
        this.requestProfile(msg.user_id);
      });

      const timestamp = $("a", {
        class: "message-timestamp",
        href: `/chat/${msg.room_id}#${msg.id}`,
        text: formatTimestamp(msg.created_at),
        title: `${formatTimestampFull(msg.created_at)} · Click to copy link`,
      });
      timestamp.addEventListener("click", (e) => {
        e.preventDefault();
        this.copyMessageLink(msg.id);
      });

      header.appendChild(usernameEl);
      header.appendChild(timestamp);

      const content = $("div", { class: "message-content" });
      content.appendChild(avatar);

      const textArea = $("div", { class: "message-text-area" });
      textArea.appendChild(header);

      const body = $("div", { class: "message-body" });
      body.innerHTML = renderMarkdown(msg.body);
      textArea.appendChild(body);

      // Show (edited) indicator if message was modified after creation
      if (msg.modified_at !== msg.created_at) {
        textArea.appendChild(
          $("span", { class: "edited-indicator", text: "(edited)" }),
        );
      }

      content.appendChild(textArea);
      wrapper.appendChild(content);
    } else {
      // Grouped message - just the body with indent to align with text
      const content = $("div", { class: "message-content grouped-content" });

      const body = $("div", { class: "message-body" });
      body.innerHTML = renderMarkdown(msg.body);

      // Add timestamp on hover (clickable permalink)
      const timestamp = $("a", {
        class: "message-timestamp hover-timestamp",
        href: `/chat/${msg.room_id}#${msg.id}`,
        text: formatTimestamp(msg.created_at),
        title: `${formatTimestampFull(msg.created_at)} · Click to copy link`,
      });
      timestamp.addEventListener("click", (e) => {
        e.preventDefault();
        this.copyMessageLink(msg.id);
      });

      content.appendChild(timestamp);
      content.appendChild(body);

      // Show (edited) indicator if message was modified
      if (msg.modified_at !== msg.created_at) {
        content.appendChild(
          $("span", { class: "edited-indicator", text: "(edited)" }),
        );
      }

      wrapper.appendChild(content);
    }

    // Add reactions if present
    if (msg.reactions && msg.reactions.length > 0) {
      const bar = this.createReactionBar(msg.id, msg.reactions);
      wrapper.appendChild(bar);
    }

    return wrapper;
  }

  /**
   * Create an avatar element with initials
   */
  createAvatar(username: string): HTMLElement {
    const initials = getInitials(username);
    const color = stringToColor(username);

    const avatar = $("div", {
      class: "message-avatar",
      text: initials,
    });
    avatar.style.backgroundColor = color;

    return avatar;
  }

  handleIncomingMessage(msg: Message) {
    // Handle messages for rooms we're not currently viewing
    if (msg.room_id !== this.state.currentRoom) {
      // Still cache the message so it's there when we switch
      this.state.addMessage(msg.room_id, msg);

      // Bump DM to top of list if it's a DM
      const room = this.state.getRoom(msg.room_id);
      if (room?.room_type === "dm") {
        this.state.bumpDM(msg.room_id);
        this.renderSidebar();
      }

      // TODO: Update unread count for other room (Phase 5)
      console.debug("message for different room", msg.room_id);
      return;
    }

    // Check if this is a confirmation of our pending message
    const pendingKey = makePendingKey(msg.body, msg.room_id, msg.user_id);
    const pending = this.pendingMessages.get(pendingKey);

    if (pending) {
      // This is our message confirmed - update the element with real data
      pending.element.setAttribute("data-message-id", msg.id);
      pending.element.classList.remove("pending");
      this.pendingMessages.delete(pendingKey);
      console.debug("confirmed pending message", msg.id);

      // Add to state cache
      this.state.addMessage(msg.room_id, msg);
    } else {
      // Message from someone else - add to state and render
      this.state.addMessage(msg.room_id, msg);
      this.appendMessageToUI(msg);
    }
  }

  /**
   * Append a single new message to the UI (for real-time updates)
   */
  appendMessageToUI(msg: Message) {
    const messageWindow = document.querySelector(".chat-messages");
    if (!messageWindow) {
      console.error("no message window found");
      return;
    }

    // Get the previous message to determine grouping
    const roomState = this.state.getCurrentRoomState();
    const messages = roomState?.messages || [];
    const prevMsg =
      messages.length >= 2 ? messages[messages.length - 2] : undefined;

    const isGrouped = this.shouldGroupWithPrevious(msg, prevMsg);
    const isOwn = msg.user_id === this.state.user.id;
    const element = this.createMessageElement(msg, isGrouped, isOwn);

    messageWindow.appendChild(element);
    messageWindow.scrollTop = messageWindow.scrollHeight;
  }

  renderSidebar() {
    // Render channels section
    const channelList = document.querySelector(".sidebar-channels ul");
    if (!channelList) {
      console.error("no channel list found");
      return;
    }

    // Clear existing placeholder channels
    channelList.innerHTML = "";

    // Render each channel room
    for (const room of this.state.rooms) {
      const li = $("li", { "data-room-id": room.id });
      const link = $("a", {
        href: `/chat/${room.id}`,
        text: `# ${room.name}`,
      });

      // Mark the active room
      if (room.id === this.state.currentRoom) {
        li.classList.add("active");
      }

      // Add click handler for room switching
      link.addEventListener("click", (e) => {
        e.preventDefault();
        this.switchRoom(room.id);
      });

      li.appendChild(link);
      channelList.appendChild(li);
    }

    // Add action buttons at the bottom of channels section
    const actionsContainer = document.querySelector(".sidebar-channels");
    if (actionsContainer) {
      // Remove existing action buttons if any
      const existingActions =
        actionsContainer.querySelector(".channel-actions");
      if (existingActions) {
        existingActions.remove();
      }

      const actions = $("div", { class: "channel-actions" });

      const createBtn = $("button", {
        class: "channel-action-btn",
        text: "+ Create channel",
      });
      createBtn.addEventListener("click", () => this.showCreateChannelModal());

      const browseBtn = $("button", {
        class: "channel-action-btn",
        text: "Browse channels",
      });
      browseBtn.addEventListener("click", () => this.requestListRooms());

      actions.appendChild(createBtn);
      actions.appendChild(browseBtn);
      actionsContainer.appendChild(actions);
    }

    // Render DMs section
    const dmList = document.querySelector(".sidebar-direct-messages ul");
    if (dmList) {
      dmList.innerHTML = "";

      // Render each DM
      for (const dm of this.state.dms) {
        const li = $("li", { "data-room-id": dm.id });
        const displayName = this.getDMDisplayName(dm);
        const link = $("a", {
          href: `/chat/${dm.id}`,
          text: displayName,
        });

        // Mark the active DM
        if (dm.id === this.state.currentRoom) {
          li.classList.add("active");
        }

        // Add click handler for DM switching
        link.addEventListener("click", (e) => {
          e.preventDefault();
          this.switchRoom(dm.id);
        });

        li.appendChild(link);
        dmList.appendChild(li);
      }

      // Add "New message" button at the bottom of DM section
      const dmContainer = document.querySelector(".sidebar-direct-messages");
      if (dmContainer) {
        // Remove existing actions if any
        const existingActions = dmContainer.querySelector(".dm-actions");
        if (existingActions) {
          existingActions.remove();
        }

        const actions = $("div", { class: "dm-actions" });
        const newMsgBtn = $("button", {
          class: "channel-action-btn",
          text: "+ New message",
        });
        newMsgBtn.addEventListener("click", () => this.showNewMessageModal());
        actions.appendChild(newMsgBtn);
        dmContainer.appendChild(actions);
      }
    }

    // Update sidebar header with user dropdown
    this.renderSidebarHeader();
  }

  /**
   * Render the sidebar header with user dropdown menu
   */
  renderSidebarHeader() {
    const header = document.querySelector(".sidebar-header");
    if (!header) return;

    // Clear existing content
    header.innerHTML = "";

    // Workspace name
    const workspaceName = $("h3", { text: "Workspace" });
    header.appendChild(workspaceName);

    // User info section
    const userSection = $("div", { class: "sidebar-user" });
    const userName = this.state.user.display_name || this.state.user.username;
    const userBtn = $("button", {
      class: "sidebar-user-btn",
      text: userName,
    });

    // Add dropdown indicator
    const dropdownIcon = $("span", { class: "dropdown-icon", text: " ▾" });
    userBtn.appendChild(dropdownIcon);

    userBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      this.toggleUserDropdown();
    });

    userSection.appendChild(userBtn);
    header.appendChild(userSection);
  }

  /**
   * Toggle the user dropdown menu
   */
  toggleUserDropdown() {
    // Close if already open
    const existingDropdown = document.querySelector(".user-dropdown");
    if (existingDropdown) {
      existingDropdown.remove();
      return;
    }

    const userSection = document.querySelector(".sidebar-user");
    if (!userSection) return;

    const dropdown = $("div", { class: "user-dropdown" });

    // Edit profile option
    const editProfileBtn = $("button", {
      class: "dropdown-item",
      text: "Edit profile",
    });
    editProfileBtn.addEventListener("click", () => {
      this.closeUserDropdown();
      this.showEditProfileModal();
    });

    // View profile option
    const viewProfileBtn = $("button", {
      class: "dropdown-item",
      text: "View profile",
    });
    viewProfileBtn.addEventListener("click", () => {
      this.closeUserDropdown();
      this.requestProfile(this.state.user.id);
    });

    dropdown.appendChild(editProfileBtn);
    dropdown.appendChild(viewProfileBtn);
    userSection.appendChild(dropdown);

    // Close dropdown when clicking elsewhere
    const closeHandler = (e: MouseEvent) => {
      if (!dropdown.contains(e.target as Node)) {
        this.closeUserDropdown();
        document.removeEventListener("click", closeHandler);
      }
    };
    // Add handler on next tick to avoid immediate close
    setTimeout(() => document.addEventListener("click", closeHandler), 0);
  }

  /**
   * Close the user dropdown menu
   */
  closeUserDropdown() {
    const dropdown = document.querySelector(".user-dropdown");
    if (dropdown) {
      dropdown.remove();
    }
  }

  /**
   * Get display name for a DM room based on members
   */
  getDMDisplayName(dm: Room): string {
    if (!dm.members || dm.members.length === 0) {
      return "Direct Message";
    }

    // Filter out the current user
    const otherMembers = dm.members.filter((m) => m.id !== this.state.user.id);

    if (otherMembers.length === 0) {
      // DM with self
      return this.state.user.display_name || this.state.user.username;
    }

    if (otherMembers.length === 1) {
      // 1:1 DM - show the other user's name
      return otherMembers[0].display_name || otherMembers[0].username;
    }

    // Group DM - show comma-separated names
    const names = otherMembers.map((m) => m.display_name || m.username);
    if (names.length <= 3) {
      return names.join(", ");
    }

    // Too many - truncate
    return `${names.slice(0, 2).join(", ")}, and ${names.length - 2} others`;
  }

  switchRoom(roomId: string) {
    if (roomId === this.state.currentRoom) {
      return;
    }

    // Save scroll position for current room before switching
    const messageWindow = document.querySelector(".chat-messages");
    if (messageWindow && this.state.currentRoom) {
      this.state.saveScrollPosition(
        this.state.currentRoom,
        messageWindow.scrollTop,
      );
    }

    // Tell the server we're switching rooms (updates last_room, validates membership)
    const request = {
      type: "join_room",
      data: {
        room_id: roomId,
      },
    };
    console.debug("sending join_room", request);
    this.conn.send(JSON.stringify(request));

    // Note: we update the UI optimistically here; the server will confirm
    // or send an error if the room switch is invalid

    // Update current room
    this.state.setCurrentRoom(roomId);

    // Track in recent rooms for quick-search
    this.addRecentRoom(roomId);

    // Update URL without reload
    window.history.pushState({ roomId }, "", `/chat/${roomId}`);

    // Update sidebar highlighting
    this.updateSidebarHighlight();

    // Update chat header
    this.updateChatHeader();

    // Clear pending messages (they were for the old room)
    this.pendingMessages.clear();

    // Check if we have cached messages for this room
    if (this.state.hasMessagesForRoom(roomId)) {
      // Render from cache
      this.renderMessages();
    } else {
      // Clear messages and request history
      this.clearMessageUI();
      this.requestHistory(roomId);
    }
  }

  /**
   * Handle server confirmation of room switch
   */
  handleJoinRoom(response: JoinRoomResponse) {
    console.debug(
      "join_room confirmed",
      response.room,
      "joined:",
      response.joined,
    );

    // If user was newly joined (not already a member), add to state and re-render sidebar
    if (response.joined) {
      this.state.addRoom(response.room);
      this.renderSidebar();
      this.updateAutocompleteChannels();
      // Update the header now that the room is in state
      this.updateChatHeader();
    }
  }

  /**
   * Handle server response to room creation
   */
  handleCreateRoom(response: CreateRoomResponse) {
    console.debug("create_room confirmed", response.room);

    // Add the new room to state
    this.state.addRoom(response.room);

    // Re-render sidebar to show new room
    this.renderSidebar();
    this.updateAutocompleteChannels();

    // Switch to the new room
    this.switchRoom(response.room.id);

    // Close the modal if open
    this.closeModal();
  }

  /**
   * Handle server response to list rooms request
   */
  handleListRooms(response: ListRoomsResponse) {
    console.debug("list_rooms response", response);

    // Update quick-search room data if open
    if (this.quickSearchOpen) {
      this.quickSearchAllRooms = response.rooms;
      this.quickSearchRoomMembership = response.is_member;
      this.updateQuickSearchResults();
    } else {
      // Only show browse modal if quick-search is not open
      this.showBrowseChannelsModal(response.rooms, response.is_member);
    }
  }

  /**
   * Handle server response to leave room request
   */
  handleLeaveRoom(response: LeaveRoomResponse) {
    console.debug("leave_room response", response);

    // Remove the room from state
    this.state.removeRoom(response.room_id);

    // Close the modal
    this.closeModal();

    // If we left the current room, switch to the first available room
    if (response.room_id === this.state.currentRoom) {
      const firstRoom = this.state.rooms[0];
      if (firstRoom) {
        this.switchRoom(firstRoom.id);
      }
    }

    // Re-render sidebar
    this.renderSidebar();
    this.updateAutocompleteChannels();
  }

  /**
   * Handle server response to room info request
   */
  handleRoomInfo(response: RoomInfoResponse) {
    console.debug("room_info response", response);
    this.showRoomInfoModal(response);
  }

  /**
   * Handle server response to create DM request
   */
  handleCreateDM(response: CreateDMResponse) {
    console.debug("create_dm response", response);

    // Add the DM to state (will handle duplicates)
    this.state.addRoom(response.room);

    // Re-render sidebar to show new DM
    this.renderSidebar();

    // Switch to the DM room
    this.switchRoom(response.room.id);

    // Close the modal
    this.closeModal();
  }

  /**
   * Handle server response to list users request
   */
  handleListUsers(response: ListUsersResponse) {
    console.debug("list_users response", response);
    // Update the user list in the "New message" modal if it's open
    this.updateUserPickerResults(response.users);
    // Also feed results to autocomplete if it's active
    if (this.autocomplete?.isActive) {
      this.autocomplete.updateUserSuggestions(response.users);
    }
    // Handle pending @mention profile lookup
    if (this.pendingMentionProfileLookup) {
      const username = this.pendingMentionProfileLookup;
      this.pendingMentionProfileLookup = null;
      const user = response.users.find(
        (u) => u.username.toLowerCase() === username.toLowerCase(),
      );
      if (user) {
        this.requestProfile(user.id);
      }
    }
    // Update quick-search user results if open
    if (this.quickSearchOpen) {
      this.quickSearchUsers = response.users;
      this.updateQuickSearchResults();
    }
  }

  /**
   * Handle server response to get profile request
   */
  handleGetProfile(response: GetProfileResponse) {
    console.debug("get_profile response", response);
    this.showProfileModal(response.user);
  }

  /**
   * Handle server response to update profile request
   */
  handleUpdateProfile(response: UpdateProfileResponse) {
    console.debug("update_profile response", response);

    // Update the user in our state
    if (this.state.initialData) {
      this.state.initialData.user = response.user;
    }

    // Re-render sidebar to update displayed name
    this.renderSidebar();

    // Close the modal
    this.closeModal();
  }

  // =========================================================================
  // Search and Permalinks
  // =========================================================================

  /**
   * Handle search response from server
   */
  handleSearch(response: SearchResponse) {
    console.debug("search response", response);
    this.isSearching = false;

    // Append to results if this is a "load more" (has cursor)
    if (this.searchNextCursor && response.results.length > 0) {
      this.searchResults = [...this.searchResults, ...response.results];
    } else {
      this.searchResults = response.results;
    }

    this.searchNextCursor = response.next_cursor || "";
    this.renderSearchResults();
  }

  /**
   * Handle get_message_context response for permalinks
   */
  handleGetMessageContext(response: GetMessageContextResponse) {
    console.debug("get_message_context response", response);

    const message = response.message;
    const roomId = response.room_id;

    // If the message is deleted, show a toast and navigate to room anyway
    if (message.deleted_at) {
      this.showToast("Message was deleted");
    }

    // Store the message ID we want to jump to
    this.pendingPermalinkMessageId = message.id;

    // If we're on the search page, navigate to chat first
    if (this.isSearchPage) {
      window.location.href = `/chat/${roomId}#${message.id}`;
      return;
    }

    // If already in this room, just scroll to the message
    if (roomId === this.state.currentRoom) {
      this.jumpToMessage(message.id);
    } else {
      // Switch rooms and wait for messages to load
      this.switchRoom(roomId);
      // The message will be jumped to after history loads
    }
  }

  /**
   * Request search results from server
   */
  requestSearch(
    query: string,
    roomId?: string,
    userId?: string,
    cursor?: string,
  ) {
    if (this.isSearching) return;

    this.isSearching = true;
    this.searchNextCursor = cursor || "";

    const request = {
      type: "search",
      data: {
        query: query,
        room_id: roomId || "",
        user_id: userId || "",
        cursor: cursor || "",
        limit: 20,
      },
    };
    console.debug("requesting search", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Request message context for permalink navigation
   */
  requestMessageContext(messageId: string) {
    const request = {
      type: "get_message_context",
      data: {
        message_id: messageId,
      },
    };
    console.debug("requesting get_message_context", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Jump to and highlight a specific message
   */
  jumpToMessage(messageId: string) {
    this.pendingPermalinkMessageId = null;

    const messageEl = document.querySelector(
      `.chat-message[data-message-id="${messageId}"]`,
    );

    if (messageEl) {
      // Scroll the message into view
      messageEl.scrollIntoView({ behavior: "smooth", block: "center" });

      // Add highlight animation
      messageEl.classList.add("highlight-flash");
      setTimeout(() => {
        messageEl.classList.remove("highlight-flash");
      }, 2000);

      // Update URL hash without triggering navigation
      window.history.replaceState(
        null,
        "",
        `/chat/${this.state.currentRoom}#${messageId}`,
      );
    } else {
      // Message not in current view - may need to load more history
      console.debug("message not found in view, may need to load more history");
      this.showToast("Message not found in current view");
    }
  }

  /**
   * Copy a permalink to a message to the clipboard
   */
  copyMessageLink(messageId: string) {
    const url = `${window.location.origin}/chat/${this.state.currentRoom}#${messageId}`;
    navigator.clipboard
      .writeText(url)
      .then(() => {
        this.showToast("Link copied to clipboard");
      })
      .catch((err) => {
        console.error("Failed to copy link:", err);
        this.showToast("Failed to copy link");
      });
  }

  /**
   * Show a toast notification
   */
  showToast(message: string, duration = 3000) {
    // Remove any existing toast
    document.querySelector(".toast")?.remove();

    const toast = $("div", { class: "toast", text: message });
    document.body.appendChild(toast);

    // Trigger animation
    requestAnimationFrame(() => {
      toast.classList.add("show");
    });

    setTimeout(() => {
      toast.classList.remove("show");
      setTimeout(() => toast.remove(), 300);
    }, duration);
  }

  /**
   * Render the search page UI
   */
  renderSearchPage() {
    this.isSearchPage = true;

    // Parse URL params for initial query
    const params = new URLSearchParams(window.location.search);
    const initialQuery = params.get("q") || "";
    const initialFrom = params.get("from") || "";

    // Replace the main chat area with search UI
    const chatArea = document.querySelector(".chat-area");
    if (!chatArea) return;

    chatArea.innerHTML = "";

    // Header with back button
    const header = $("div", { class: "search-header" });
    const backBtn = $("button", {
      class: "search-back-btn",
      text: "← Back to Chat",
    });
    backBtn.addEventListener("click", () => {
      window.location.href = "/chat";
    });
    header.appendChild(backBtn);
    header.appendChild($("h2", { text: "Search Messages" }));
    chatArea.appendChild(header);

    // Search form
    const form = $("form", { class: "search-form" });
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.performSearch();
    });

    const searchInput = $("input", {
      type: "text",
      id: "search-input",
      class: "search-input",
      placeholder: "Search messages...",
      value: initialQuery,
    }) as HTMLInputElement;
    form.appendChild(searchInput);

    // Filters row
    const filtersRow = $("div", { class: "search-filters" });

    // Room filter dropdown
    const roomSelect = $("select", {
      id: "search-room-filter",
      class: "search-select",
    });
    roomSelect.appendChild($("option", { value: "", text: "All rooms" }));
    // Rooms will be populated after init

    // User filter (will use the search input approach for simplicity)
    const userInput = $("input", {
      type: "text",
      id: "search-user-filter",
      class: "search-filter-input",
      placeholder: "From user...",
      value: initialFrom,
    });

    filtersRow.appendChild(
      $("label", { text: "Room:", for: "search-room-filter" }),
    );
    filtersRow.appendChild(roomSelect);
    filtersRow.appendChild(
      $("label", { text: "From:", for: "search-user-filter" }),
    );
    filtersRow.appendChild(userInput);
    filtersRow.appendChild(
      $("button", { type: "submit", class: "btn btn-primary", text: "Search" }),
    );

    form.appendChild(filtersRow);
    chatArea.appendChild(form);

    // Results area
    const resultsArea = $("div", {
      class: "search-results",
      id: "search-results",
    });
    resultsArea.appendChild(
      $("p", {
        class: "search-hint",
        text: "Enter a search term to find messages.",
      }),
    );
    chatArea.appendChild(resultsArea);

    // Focus input
    searchInput.focus();

    // If there's an initial query, run the search
    if (initialQuery) {
      this.performSearch();
    }
  }

  /**
   * Perform a search using current form values
   */
  performSearch() {
    const queryInput = document.getElementById(
      "search-input",
    ) as HTMLInputElement;
    const roomSelect = document.getElementById(
      "search-room-filter",
    ) as HTMLSelectElement;
    const userInput = document.getElementById(
      "search-user-filter",
    ) as HTMLInputElement;

    const query = queryInput?.value.trim() || "";
    if (!query) {
      this.showToast("Please enter a search term");
      return;
    }

    const roomId = roomSelect?.value || "";
    const userId = userInput?.value.trim() || "";

    // Update URL
    const params = new URLSearchParams();
    params.set("q", query);
    if (roomId) params.set("room", roomId);
    if (userId) params.set("from", userId);
    window.history.replaceState(null, "", `/search?${params.toString()}`);

    // Clear previous results and search
    this.searchResults = [];
    this.searchNextCursor = "";
    this.requestSearch(query, roomId, userId);

    // Show loading state
    const resultsArea = document.getElementById("search-results");
    if (resultsArea) {
      resultsArea.innerHTML = "";
      resultsArea.appendChild(
        $("p", { class: "search-loading", text: "Searching..." }),
      );
    }
  }

  /**
   * Render search results
   */
  renderSearchResults() {
    const resultsArea = document.getElementById("search-results");
    if (!resultsArea) return;

    resultsArea.innerHTML = "";

    if (this.searchResults.length === 0) {
      resultsArea.appendChild(
        $("p", {
          class: "search-empty",
          text: "No messages found. Try a different search term.",
        }),
      );
      return;
    }

    // Results count
    resultsArea.appendChild(
      $("p", {
        class: "search-count",
        text: `${this.searchResults.length} result${this.searchResults.length === 1 ? "" : "s"}`,
      }),
    );

    // Render each result
    for (const result of this.searchResults) {
      const card = this.createSearchResultCard(result);
      resultsArea.appendChild(card);
    }

    // Load more button
    if (this.searchNextCursor) {
      const loadMoreBtn = $("button", {
        class: "btn btn-secondary load-more-search",
        text: "Load more results",
      });
      loadMoreBtn.addEventListener("click", () => {
        const queryInput = document.getElementById(
          "search-input",
        ) as HTMLInputElement;
        const roomSelect = document.getElementById(
          "search-room-filter",
        ) as HTMLSelectElement;
        const userInput = document.getElementById(
          "search-user-filter",
        ) as HTMLInputElement;

        const query = queryInput?.value.trim() || "";
        const roomId = roomSelect?.value || "";
        const userId = userInput?.value.trim() || "";

        this.requestSearch(query, roomId, userId, this.searchNextCursor);
      });
      resultsArea.appendChild(loadMoreBtn);
    }
  }

  /**
   * Create a search result card element
   */
  createSearchResultCard(result: SearchResult): HTMLElement {
    const card = $("div", { class: "search-result-card" });

    // Header line: room · user · date
    const header = $("div", { class: "search-result-header" });
    header.appendChild(
      $("span", { class: "search-result-room", text: `#${result.room_name}` }),
    );
    header.appendChild($("span", { class: "search-result-sep", text: " · " }));
    header.appendChild(
      $("span", { class: "search-result-user", text: result.username }),
    );
    header.appendChild($("span", { class: "search-result-sep", text: " · " }));
    header.appendChild(
      $("span", {
        class: "search-result-date",
        text: formatTimestamp(result.created_at),
      }),
    );
    card.appendChild(header);

    // Snippet with highlighted matches (** → <strong>)
    const snippet = $("div", { class: "search-result-snippet" });
    // Convert **term** to <strong>term</strong>
    const highlightedSnippet = result.snippet.replace(
      /\*\*(.+?)\*\*/g,
      "<strong>$1</strong>",
    );
    snippet.innerHTML = highlightedSnippet;
    card.appendChild(snippet);

    // Click to navigate to message
    card.addEventListener("click", () => {
      this.requestMessageContext(result.message_id);
    });

    return card;
  }

  /**
   * Populate room filter dropdown with user's rooms
   */
  populateSearchRoomFilter() {
    const roomSelect = document.getElementById(
      "search-room-filter",
    ) as HTMLSelectElement;
    if (!roomSelect) return;

    // Clear existing options except "All rooms"
    while (roomSelect.options.length > 1) {
      roomSelect.remove(1);
    }

    // Add channels
    for (const room of this.state.rooms) {
      roomSelect.appendChild(
        $("option", {
          value: room.id,
          text: `# ${room.name}`,
        }) as HTMLOptionElement,
      );
    }

    // Add DMs
    for (const dm of this.state.dms) {
      const name = this.getDMDisplayName(dm);
      roomSelect.appendChild(
        $("option", { value: dm.id, text: name }) as HTMLOptionElement,
      );
    }

    // Set initial value from URL if present
    const params = new URLSearchParams(window.location.search);
    const initialRoom = params.get("room") || "";
    if (initialRoom) {
      roomSelect.value = initialRoom;
    }
  }

  // =========================================================================
  // Quick-Search (Cmd+K) Panel
  // =========================================================================

  /**
   * Get recent rooms from localStorage
   */
  getRecentRooms(): string[] {
    try {
      const stored = localStorage.getItem(RECENT_ROOMS_KEY);
      return stored ? JSON.parse(stored) : [];
    } catch {
      return [];
    }
  }

  /**
   * Add a room to the recent rooms list
   */
  addRecentRoom(roomId: string) {
    const recent = this.getRecentRooms().filter((id) => id !== roomId);
    recent.unshift(roomId);
    const trimmed = recent.slice(0, MAX_RECENT_ROOMS);
    try {
      localStorage.setItem(RECENT_ROOMS_KEY, JSON.stringify(trimmed));
    } catch {
      // Ignore localStorage errors
    }
  }

  /**
   * Open the quick-search modal
   */
  openQuickSearch() {
    if (this.quickSearchOpen) return;
    this.quickSearchOpen = true;
    this.quickSearchQuery = "";
    this.quickSearchSelectedIndex = 0;
    this.quickSearchUsers = [];
    this.quickSearchAllRooms = [];
    this.quickSearchRoomMembership = [];

    // Request all accessible rooms from server
    this.requestListRooms();

    // Create modal
    const overlay = $("div", { class: "quick-search-overlay" });
    const modal = $("div", { class: "quick-search-modal" });

    // Search input
    const inputContainer = $("div", { class: "quick-search-input-container" });
    const searchIcon = $("span", { class: "quick-search-icon", text: "🔍" });
    const input = $("input", {
      type: "text",
      class: "quick-search-input",
      placeholder: "Search rooms, users, or messages...",
      id: "quick-search-input",
    }) as HTMLInputElement;

    inputContainer.appendChild(searchIcon);
    inputContainer.appendChild(input);
    modal.appendChild(inputContainer);

    // Results container
    const results = $("div", {
      class: "quick-search-results",
      id: "quick-search-results",
    });
    modal.appendChild(results);

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    // Show initial results (recents)
    this.updateQuickSearchResults();

    // Focus input
    input.focus();

    // Event handlers
    input.addEventListener("input", () => {
      this.quickSearchQuery = input.value;
      this.quickSearchSelectedIndex = 0;

      // If there's a query with 2+ chars, request users from server
      if (this.quickSearchQuery.length >= 2) {
        this.requestListUsers(this.quickSearchQuery);
      } else {
        this.quickSearchUsers = [];
      }

      this.updateQuickSearchResults();
    });

    input.addEventListener("keydown", (e) => {
      this.handleQuickSearchKeydown(e);
    });

    // Close on overlay click
    overlay.addEventListener("click", (e) => {
      if (e.target === overlay) {
        this.closeQuickSearch();
      }
    });
  }

  /**
   * Close the quick-search modal
   */
  closeQuickSearch() {
    if (!this.quickSearchOpen) return;
    this.quickSearchOpen = false;
    this.quickSearchQuery = "";
    this.quickSearchItems = [];
    this.quickSearchUsers = [];
    this.quickSearchAllRooms = [];
    this.quickSearchRoomMembership = [];

    const overlay = document.querySelector(".quick-search-overlay");
    if (overlay) {
      overlay.remove();
    }
  }

  /**
   * Handle keyboard navigation in quick-search
   */
  handleQuickSearchKeydown(e: KeyboardEvent) {
    switch (e.key) {
      case "Escape":
        e.preventDefault();
        this.closeQuickSearch();
        break;

      case "ArrowDown":
        e.preventDefault();
        if (this.quickSearchItems.length > 0) {
          this.quickSearchSelectedIndex =
            (this.quickSearchSelectedIndex + 1) % this.quickSearchItems.length;
          this.renderQuickSearchSelection();
        }
        break;

      case "ArrowUp":
        e.preventDefault();
        if (this.quickSearchItems.length > 0) {
          this.quickSearchSelectedIndex =
            (this.quickSearchSelectedIndex - 1 + this.quickSearchItems.length) %
            this.quickSearchItems.length;
          this.renderQuickSearchSelection();
        }
        break;

      case "Enter":
        e.preventDefault();
        this.selectQuickSearchItem();
        break;
    }
  }

  /**
   * Update quick-search results based on current query
   */
  updateQuickSearchResults() {
    const query = this.quickSearchQuery.toLowerCase().trim();
    const items: QuickSearchItem[] = [];

    if (!query) {
      // Show recent rooms
      const recentIds = this.getRecentRooms();
      for (const roomId of recentIds) {
        const room = this.state.getRoom(roomId);
        if (room) {
          if (room.room_type === "dm") {
            items.push({
              type: "dm",
              id: room.id,
              name: this.getDMDisplayName(room),
              isMember: true,
            });
          } else {
            items.push({
              type: "room",
              id: room.id,
              name: room.name,
              isMember: true,
            });
          }
        }
      }
    } else {
      // Filter channels from ALL accessible rooms (not just joined)
      // Use quickSearchAllRooms if available, fall back to user's rooms
      const roomsToSearch =
        this.quickSearchAllRooms.length > 0
          ? this.quickSearchAllRooms
          : this.state.rooms;

      for (let i = 0; i < roomsToSearch.length; i++) {
        const room = roomsToSearch[i];
        if (room.name.toLowerCase().includes(query)) {
          // Determine membership status
          const isMember =
            this.quickSearchAllRooms.length > 0
              ? this.quickSearchRoomMembership[i]
              : true; // If using state.rooms, user is always a member

          items.push({
            type: "room",
            id: room.id,
            name: room.name,
            isMember: isMember,
            secondary: isMember ? undefined : "Join",
          });
        }
      }

      // Filter DMs (user is always a member of their DMs)
      for (const dm of this.state.dms) {
        const displayName = this.getDMDisplayName(dm).toLowerCase();
        if (displayName.includes(query)) {
          items.push({
            type: "dm",
            id: dm.id,
            name: this.getDMDisplayName(dm),
            isMember: true,
          });
        }
      }

      // Add users from server response (filtered to exclude already-shown DMs and self)
      for (const user of this.quickSearchUsers) {
        if (user.id === this.state.user.id) continue;

        const matchesUsername = user.username.toLowerCase().includes(query);
        const matchesDisplayName = user.display_name
          ?.toLowerCase()
          .includes(query);

        if (matchesUsername || matchesDisplayName) {
          // Check if we already have a DM with this user shown
          const existingDM = items.find(
            (item) =>
              item.type === "dm" &&
              this.state.dms
                .find((d) => d.id === item.id)
                ?.members?.some((m) => m.id === user.id),
          );
          if (!existingDM) {
            items.push({
              type: "user",
              id: user.id,
              name: user.display_name || user.username,
              secondary: user.display_name ? `@${user.username}` : undefined,
            });
          }
        }
      }

      // Limit total results
      items.splice(10);

      // Add "Search messages" escape hatch at the end
      items.push({
        type: "search-escape",
        id: "search",
        name: `Search messages for "${this.quickSearchQuery}"`,
      });
    }

    this.quickSearchItems = items;

    // Ensure selected index is valid
    if (this.quickSearchSelectedIndex >= items.length) {
      this.quickSearchSelectedIndex = Math.max(0, items.length - 1);
    }

    this.renderQuickSearchResults();
  }

  /**
   * Render the quick-search results list
   */
  renderQuickSearchResults() {
    const container = document.getElementById("quick-search-results");
    if (!container) return;

    container.innerHTML = "";

    if (this.quickSearchItems.length === 0) {
      const empty = $("div", {
        class: "quick-search-empty",
        text: "No results found",
      });
      container.appendChild(empty);
      return;
    }

    // Section header for recents
    if (!this.quickSearchQuery) {
      const header = $("div", {
        class: "quick-search-section-header",
        text: "Recent",
      });
      container.appendChild(header);
    }

    // Render items
    for (let i = 0; i < this.quickSearchItems.length; i++) {
      const item = this.quickSearchItems[i];
      const isSelected = i === this.quickSearchSelectedIndex;

      const el = $("div", {
        class: `quick-search-item ${isSelected ? "selected" : ""}`,
        "data-index": String(i),
      });

      // Icon
      let icon = "";
      if (item.type === "room") {
        icon = "#";
      } else if (item.type === "dm") {
        icon = "💬";
      } else if (item.type === "user") {
        icon = "👤";
      } else if (item.type === "search-escape") {
        icon = "🔍";
      }

      const iconSpan = $("span", {
        class: "quick-search-item-icon",
        text: icon,
      });
      el.appendChild(iconSpan);

      // Name
      const nameSpan = $("span", {
        class: "quick-search-item-name",
        text: item.name,
      });
      el.appendChild(nameSpan);

      // Secondary text (e.g., @username or "Join" for non-member rooms)
      if (item.secondary) {
        const isJoinHint = item.type === "room" && item.isMember === false;
        const secondarySpan = $("span", {
          class: `quick-search-item-secondary${isJoinHint ? " join-hint" : ""}`,
          text: item.secondary,
        });
        el.appendChild(secondarySpan);
      }

      // Click handler
      el.addEventListener("click", () => {
        this.quickSearchSelectedIndex = i;
        this.selectQuickSearchItem();
      });

      // Hover to select
      el.addEventListener("mouseenter", () => {
        this.quickSearchSelectedIndex = i;
        this.renderQuickSearchSelection();
      });

      container.appendChild(el);
    }
  }

  /**
   * Update the visual selection without re-rendering all items
   */
  renderQuickSearchSelection() {
    const items = document.querySelectorAll(".quick-search-item");
    for (let i = 0; i < items.length; i++) {
      items[i].classList.toggle(
        "selected",
        i === this.quickSearchSelectedIndex,
      );
    }

    // Scroll selected item into view
    const selected = items[this.quickSearchSelectedIndex];
    if (selected) {
      selected.scrollIntoView({ block: "nearest" });
    }
  }

  /**
   * Select the currently highlighted quick-search item
   */
  selectQuickSearchItem() {
    const item = this.quickSearchItems[this.quickSearchSelectedIndex];
    if (!item) return;

    this.closeQuickSearch();

    switch (item.type) {
      case "room":
        this.addRecentRoom(item.id);
        if (item.isMember) {
          // Already a member, just switch to the room
          this.switchRoom(item.id);
        } else {
          // Not a member, join the room first (joinRoom handles the switch)
          this.joinRoom(item.id);
        }
        break;

      case "dm":
        this.addRecentRoom(item.id);
        this.switchRoom(item.id);
        break;

      case "user":
        // Start a DM with this user
        this.requestCreateDM([item.id]);
        break;

      case "search-escape":
        // Navigate to search page with query
        window.location.href = `/search?q=${encodeURIComponent(this.quickSearchQuery)}`;
        break;
    }
  }

  /**
   * Handle global Cmd+K / Ctrl+K keyboard shortcut
   */
  handleGlobalKeydown(e: KeyboardEvent) {
    // Cmd+K (Mac) or Ctrl+K (Windows/Linux) to open quick-search
    const isMac = navigator.platform.toUpperCase().includes("MAC");
    const modifier = isMac ? e.metaKey : e.ctrlKey;

    if (modifier && e.key === "k") {
      e.preventDefault();
      if (this.quickSearchOpen) {
        this.closeQuickSearch();
      } else {
        this.openQuickSearch();
      }
    }

    // Escape to close quick-search (handled in modal keydown, but also here for safety)
    if (e.key === "Escape" && this.quickSearchOpen) {
      this.closeQuickSearch();
    }
  }

  // =========================================================================
  // Rich messaging handlers (edit, delete, reactions)
  // =========================================================================

  /**
   * Handle a message_edited broadcast
   */
  handleMessageEdited(data: MessageEdited) {
    console.debug("message_edited", data);

    // Update message in state cache
    const roomState = this.state.getRoomState(data.room_id);
    const msg = roomState.messages.find((m) => m.id === data.message_id);
    if (msg) {
      msg.body = data.body;
      msg.modified_at = data.modified_at;
    }

    // Update DOM if this room is currently visible
    if (data.room_id === this.state.currentRoom) {
      const el = document.querySelector(
        `.chat-message[data-message-id="${data.message_id}"]`,
      );
      if (el) {
        // Cancel inline edit if this message is being edited
        if (this.editingMessageId === data.message_id) {
          this.cancelEdit();
        }

        const bodyEl = el.querySelector(".message-body");
        if (bodyEl) {
          bodyEl.innerHTML = renderMarkdown(data.body);
        }

        // Add or update the (edited) indicator
        let editedEl = el.querySelector(".edited-indicator");
        if (!editedEl) {
          editedEl = $("span", {
            class: "edited-indicator",
            text: "(edited)",
          });
          // Insert after the message body
          const bodyContainer = bodyEl?.parentElement;
          if (bodyContainer) {
            bodyContainer.appendChild(editedEl);
          }
        }
      }
    }
  }

  /**
   * Handle a message_deleted broadcast
   */
  handleMessageDeleted(data: MessageDeleted) {
    console.debug("message_deleted", data);

    // Update message in state cache
    const roomState = this.state.getRoomState(data.room_id);
    const msg = roomState.messages.find((m) => m.id === data.message_id);
    if (msg) {
      msg.body = "";
      msg.deleted_at = new Date().toISOString();
    }

    // Update DOM if this room is currently visible
    if (data.room_id === this.state.currentRoom) {
      // Cancel inline edit if this message is being edited
      if (this.editingMessageId === data.message_id) {
        this.cancelEdit();
      }

      const el = document.querySelector(
        `.chat-message[data-message-id="${data.message_id}"]`,
      );
      if (el) {
        // Replace with tombstone
        el.className = "chat-message deleted";
        el.innerHTML = "";
        const tombstone = $("div", { class: "message-content tombstone" });
        tombstone.appendChild($("em", { text: "This message was deleted." }));
        el.appendChild(tombstone);
      }
    }
  }

  /**
   * Handle a reaction_updated broadcast
   */
  handleReactionUpdated(data: ReactionUpdated) {
    console.debug("reaction_updated", data);

    // Update message in state cache
    const roomState = this.state.getRoomState(data.room_id);
    const msg = roomState.messages.find((m) => m.id === data.message_id);
    if (msg) {
      if (!msg.reactions) {
        msg.reactions = [];
      }

      const existing = msg.reactions.find((r) => r.emoji === data.emoji);
      if (data.action === "add") {
        if (existing) {
          if (!existing.user_ids.includes(data.user_id)) {
            existing.user_ids.push(data.user_id);
            existing.count++;
          }
        } else {
          msg.reactions.push({
            emoji: data.emoji,
            count: 1,
            user_ids: [data.user_id],
          });
        }
      } else if (data.action === "remove") {
        if (existing) {
          existing.user_ids = existing.user_ids.filter(
            (id) => id !== data.user_id,
          );
          existing.count = existing.user_ids.length;
          if (existing.count === 0) {
            msg.reactions = msg.reactions.filter((r) => r.emoji !== data.emoji);
          }
        }
      }
    }

    // Update DOM if this room is currently visible
    if (data.room_id === this.state.currentRoom) {
      this.renderReactionsForMessage(data.message_id);
    }
  }

  /**
   * Send an edit message request
   */
  requestEditMessage(messageId: string, body: string) {
    const request = {
      type: "edit_message",
      data: {
        message_id: messageId,
        body: body,
      },
    };
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Send a delete message request
   */
  requestDeleteMessage(messageId: string) {
    const request = {
      type: "delete_message",
      data: {
        message_id: messageId,
      },
    };
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Send an add reaction request
   */
  requestAddReaction(messageId: string, emoji: string) {
    const request = {
      type: "add_reaction",
      data: {
        message_id: messageId,
        emoji: emoji,
      },
    };
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Send a remove reaction request
   */
  requestRemoveReaction(messageId: string, emoji: string) {
    const request = {
      type: "remove_reaction",
      data: {
        message_id: messageId,
        emoji: emoji,
      },
    };
    this.conn.send(JSON.stringify(request));
  }

  // =========================================================================
  // Inline editing
  // =========================================================================

  private editingMessageId: string | null = null;

  /**
   * Start inline editing of a message
   */
  startEdit(messageId: string) {
    // Cancel any existing edit
    if (this.editingMessageId) {
      this.cancelEdit();
    }

    // Find the message in state
    const roomState = this.state.getCurrentRoomState();
    if (!roomState) return;
    const msg = roomState.messages.find((m) => m.id === messageId);
    if (!msg || msg.user_id !== this.state.user.id) return;
    if (msg.deleted_at) return;

    this.editingMessageId = messageId;

    const el = document.querySelector(
      `.chat-message[data-message-id="${messageId}"]`,
    );
    if (!el) return;

    const bodyEl = el.querySelector(".message-body");
    if (!bodyEl) return;

    // Replace body with textarea
    const editContainer = $("div", { class: "edit-container" });
    const textarea = $("textarea", {
      class: "edit-textarea",
    }) as HTMLTextAreaElement;
    textarea.value = msg.body;

    const buttonRow = $("div", { class: "edit-buttons" });
    const saveBtn = $("button", {
      class: "btn btn-small btn-primary",
      text: "Save",
    });
    const cancelBtn = $("button", {
      class: "btn btn-small btn-secondary",
      text: "Cancel",
    });
    const hint = $("span", {
      class: "edit-hint",
      text: "Enter to save, Escape to cancel",
    });

    saveBtn.addEventListener("click", () => {
      const newBody = textarea.value.trim();
      if (newBody && newBody !== msg.body) {
        this.requestEditMessage(messageId, newBody);
      }
      this.cancelEdit();
    });

    cancelBtn.addEventListener("click", () => {
      this.cancelEdit();
    });

    textarea.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        const newBody = textarea.value.trim();
        if (newBody && newBody !== msg.body) {
          this.requestEditMessage(messageId, newBody);
        }
        this.cancelEdit();
      } else if (e.key === "Escape") {
        this.cancelEdit();
      }
    });

    buttonRow.appendChild(hint);
    buttonRow.appendChild(cancelBtn);
    buttonRow.appendChild(saveBtn);
    editContainer.appendChild(textarea);
    editContainer.appendChild(buttonRow);

    // Hide original body, show edit
    (bodyEl as HTMLElement).style.display = "none";
    bodyEl.parentElement?.insertBefore(editContainer, bodyEl.nextSibling);

    // Auto-resize and focus
    textarea.style.height = `${Math.max(textarea.scrollHeight, 40)}px`;
    textarea.focus();
    textarea.setSelectionRange(textarea.value.length, textarea.value.length);
  }

  /**
   * Cancel inline editing
   */
  cancelEdit() {
    if (!this.editingMessageId) return;

    const el = document.querySelector(
      `.chat-message[data-message-id="${this.editingMessageId}"]`,
    );
    if (el) {
      const editContainer = el.querySelector(".edit-container");
      if (editContainer) {
        editContainer.remove();
      }
      const bodyEl = el.querySelector(".message-body") as HTMLElement;
      if (bodyEl) {
        bodyEl.style.display = "";
      }
    }

    this.editingMessageId = null;
  }

  // =========================================================================
  // Hover toolbar & reaction quick-pick
  // =========================================================================

  /**
   * Build the hover toolbar buttons for a message and append it to the message element.
   * The toolbar is a child of the message, so hovering it doesn't trigger mouseleave.
   */
  buildHoverToolbar(msgEl: HTMLElement) {
    const messageId = msgEl.getAttribute("data-message-id");
    if (!messageId) return;

    // Don't show toolbar on deleted messages or pending messages
    if (
      msgEl.classList.contains("deleted") ||
      msgEl.classList.contains("pending")
    )
      return;

    // Remove any existing toolbar in this message
    msgEl.querySelector(".message-hover-toolbar")?.remove();

    const toolbar = $("div", { class: "message-hover-toolbar" });

    // Reaction button (always shown)
    const reactBtn = $("button", {
      class: "toolbar-btn",
      title: "Add reaction",
      text: "😀",
    });
    reactBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      this.showReactionPicker(messageId, reactBtn);
    });
    toolbar.appendChild(reactBtn);

    // Copy link button (always shown)
    const linkBtn = $("button", {
      class: "toolbar-btn",
      title: "Copy link to message",
      text: "🔗",
    });
    linkBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      this.copyMessageLink(messageId);
    });
    toolbar.appendChild(linkBtn);

    // Edit and delete buttons (only for own messages)
    const roomState = this.state.getCurrentRoomState();
    const msg = roomState?.messages.find((m) => m.id === messageId);
    if (msg && msg.user_id === this.state.user.id && !msg.deleted_at) {
      const editBtn = $("button", {
        class: "toolbar-btn",
        title: "Edit message",
        text: "✏️",
      });
      editBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        this.startEdit(messageId);
      });
      toolbar.appendChild(editBtn);

      const deleteBtn = $("button", {
        class: "toolbar-btn",
        title: "Delete message",
        text: "🗑️",
      });
      deleteBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        this.showDeleteConfirmation(messageId, deleteBtn);
      });
      toolbar.appendChild(deleteBtn);
    }

    msgEl.appendChild(toolbar);
  }

  /**
   * Show the emoji quick-pick bar
   */
  showReactionPicker(messageId: string, anchor: HTMLElement) {
    // Remove any existing picker
    document.querySelector(".reaction-picker")?.remove();

    const quickEmojis = ["👍", "❤️", "😂", "😮", "🎉", "🔥", "👀", "🙏"];

    const picker = $("div", { class: "reaction-picker" });
    for (const emoji of quickEmojis) {
      const btn = $("button", { class: "reaction-picker-btn", text: emoji });
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        this.requestAddReaction(messageId, emoji);
        picker.remove();
      });
      picker.appendChild(btn);
    }

    // Position below the anchor, clamped to viewport (fixed positioning)
    document.body.appendChild(picker);
    const rect = anchor.getBoundingClientRect();
    const pickerWidth = picker.offsetWidth;
    let left = rect.left;
    // Clamp so it doesn't overflow the right edge of the viewport
    if (left + pickerWidth > window.innerWidth - 8) {
      left = window.innerWidth - pickerWidth - 8;
    }
    picker.style.top = `${rect.bottom + 4}px`;
    picker.style.left = `${left}px`;

    // Close when clicking elsewhere
    const closeHandler = (e: MouseEvent) => {
      if (!picker.contains(e.target as Node)) {
        picker.remove();
        document.removeEventListener("click", closeHandler);
      }
    };
    setTimeout(() => document.addEventListener("click", closeHandler), 0);
  }

  /**
   * Show delete confirmation popover
   */
  showDeleteConfirmation(messageId: string, anchor: HTMLElement) {
    // Remove any existing confirmation
    document.querySelector(".delete-confirmation")?.remove();

    const confirm = $("div", { class: "delete-confirmation" });
    confirm.appendChild($("p", { text: "Delete this message?" }));

    const buttonRow = $("div", { class: "confirm-buttons" });
    const deleteBtn = $("button", {
      class: "btn btn-small btn-danger",
      text: "Delete",
    });
    const cancelBtn = $("button", {
      class: "btn btn-small btn-secondary",
      text: "Cancel",
    });

    deleteBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      this.requestDeleteMessage(messageId);
      confirm.remove();
    });

    cancelBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      confirm.remove();
    });

    buttonRow.appendChild(cancelBtn);
    buttonRow.appendChild(deleteBtn);
    confirm.appendChild(buttonRow);

    // Position near the anchor, clamped to viewport (fixed positioning)
    document.body.appendChild(confirm);
    const rect = anchor.getBoundingClientRect();
    const confirmWidth = confirm.offsetWidth;
    let left = rect.left - 100;
    if (left + confirmWidth > window.innerWidth - 8) {
      left = window.innerWidth - confirmWidth - 8;
    }
    if (left < 8) {
      left = 8;
    }
    confirm.style.top = `${rect.bottom + 4}px`;
    confirm.style.left = `${left}px`;

    // Close when clicking elsewhere
    const closeHandler = (e: MouseEvent) => {
      if (!confirm.contains(e.target as Node)) {
        confirm.remove();
        document.removeEventListener("click", closeHandler);
      }
    };
    setTimeout(() => document.addEventListener("click", closeHandler), 0);
  }

  // =========================================================================
  // Reaction display
  // =========================================================================

  /**
   * Render reaction pills for a specific message
   */
  renderReactionsForMessage(messageId: string) {
    const el = document.querySelector(
      `.chat-message[data-message-id="${messageId}"]`,
    );
    if (!el) return;

    const roomState = this.state.getCurrentRoomState();
    const msg = roomState?.messages.find((m) => m.id === messageId);
    if (!msg) return;

    // Remove existing reaction bar
    el.querySelector(".reaction-bar")?.remove();

    if (!msg.reactions || msg.reactions.length === 0) return;

    const bar = this.createReactionBar(messageId, msg.reactions);
    el.appendChild(bar);
  }

  /**
   * Create a reaction bar element
   */
  createReactionBar(messageId: string, reactions: Reaction[]): HTMLElement {
    const bar = $("div", { class: "reaction-bar" });

    for (const reaction of reactions) {
      const isOwn = reaction.user_ids.includes(this.state.user.id);
      const pill = $("button", {
        class: `reaction-pill ${isOwn ? "reaction-own" : ""}`,
        title: reaction.user_ids
          .map((id) => {
            // Try to find username from state
            const roomState = this.state.getCurrentRoomState();
            const msg = roomState?.messages.find((m) => m.user_id === id);
            return msg?.username || id;
          })
          .join(", "),
      });
      pill.appendChild(
        $("span", { class: "reaction-emoji", text: reaction.emoji }),
      );
      pill.appendChild(
        $("span", { class: "reaction-count", text: ` ${reaction.count}` }),
      );

      // Toggle reaction on click
      pill.addEventListener("click", (e) => {
        e.stopPropagation();
        if (isOwn) {
          this.requestRemoveReaction(messageId, reaction.emoji);
        } else {
          this.requestAddReaction(messageId, reaction.emoji);
        }
      });

      bar.appendChild(pill);
    }

    // Add "+" button for adding new reactions
    const addBtn = $("button", {
      class: "reaction-pill reaction-add",
      title: "Add reaction",
      text: "+",
    });
    addBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      this.showReactionPicker(messageId, addBtn);
    });
    bar.appendChild(addBtn);

    return bar;
  }

  /**
   * Request list of public rooms from server
   */
  requestListRooms(query?: string) {
    const request = {
      type: "list_rooms",
      data: {
        query: query || "",
      },
    };
    console.debug("requesting list_rooms", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Request room info from server
   */
  requestRoomInfo(roomId: string) {
    const request = {
      type: "room_info",
      data: {
        room_id: roomId,
      },
    };
    console.debug("requesting room_info", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Request to leave a room
   */
  requestLeaveRoom(roomId: string) {
    const request = {
      type: "leave_room",
      data: {
        room_id: roomId,
      },
    };
    console.debug("requesting leave_room", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Send request to create a new room
   */
  createRoom(name: string, isPrivate: boolean) {
    const request = {
      type: "create_room",
      data: {
        name: name,
        is_private: isPrivate,
      },
    };
    console.debug("creating room", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Join a room the user is not currently a member of
   */
  joinRoom(roomId: string) {
    const request = {
      type: "join_room",
      data: {
        room_id: roomId,
      },
    };
    console.debug("joining room", request);
    this.conn.send(JSON.stringify(request));

    // Switch to the room
    this.switchRoom(roomId);

    // Close the modal
    this.closeModal();
  }

  /**
   * Request to create or find a DM with specified users
   */
  requestCreateDM(userIds: string[]) {
    const request = {
      type: "create_dm",
      data: {
        user_ids: userIds,
      },
    };
    console.debug("creating DM", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Request list of users for user picker
   */
  requestListUsers(query: string) {
    const request = {
      type: "list_users",
      data: {
        query: query,
      },
    };
    console.debug("requesting list_users", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Request a user's profile
   */
  requestProfile(userId: string) {
    const request = {
      type: "get_profile",
      data: {
        user_id: userId,
      },
    };
    console.debug("requesting get_profile", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Request to update current user's profile
   */
  requestUpdateProfile(displayName?: string, status?: string) {
    const data: { display_name?: string; status?: string } = {};
    if (displayName !== undefined) {
      data.display_name = displayName;
    }
    if (status !== undefined) {
      data.status = status;
    }

    const request = {
      type: "update_profile",
      data: data,
    };
    console.debug("requesting update_profile", request);
    this.conn.send(JSON.stringify(request));
  }

  /**
   * Show modal for creating a new channel
   */
  showCreateChannelModal() {
    const modal = this.createModal("Create a channel");

    const form = $("form", { class: "modal-form" });

    const nameLabel = $("label", { text: "Channel name", for: "channel-name" });
    const nameInput = $("input", {
      type: "text",
      id: "channel-name",
      name: "name",
      placeholder: "e.g. project-updates",
      maxlength: "80",
      required: "true",
    }) as HTMLInputElement;

    const privateLabel = $("label", { class: "checkbox-label" });
    const privateCheckbox = $("input", {
      type: "checkbox",
      id: "channel-private",
      name: "is_private",
    }) as HTMLInputElement;
    const privateText = $("span", { text: " Make this channel private" });
    privateLabel.appendChild(privateCheckbox);
    privateLabel.appendChild(privateText);

    const privateHint = $("p", {
      class: "form-hint",
      text: "Private channels are only visible to invited members.",
    });

    const buttonRow = $("div", { class: "button-row" });
    const cancelBtn = $("button", {
      type: "button",
      class: "btn btn-secondary",
      text: "Cancel",
    });
    const createBtn = $("button", {
      type: "submit",
      class: "btn btn-primary",
      text: "Create Channel",
    });

    cancelBtn.addEventListener("click", () => this.closeModal());

    form.addEventListener("submit", (e) => {
      e.preventDefault();
      const name = nameInput.value.trim();
      if (name) {
        this.createRoom(name, privateCheckbox.checked);
      }
    });

    buttonRow.appendChild(cancelBtn);
    buttonRow.appendChild(createBtn);

    form.appendChild(nameLabel);
    form.appendChild(nameInput);
    form.appendChild(privateLabel);
    form.appendChild(privateHint);
    form.appendChild(buttonRow);

    modal.appendChild(form);

    // Focus the input
    nameInput.focus();
  }

  /**
   * Show modal for browsing and joining public channels
   */
  showBrowseChannelsModal(rooms: Room[], isMember: boolean[]) {
    const modal = this.createModal("Browse channels");

    // Search input
    const searchContainer = $("div", { class: "modal-search" });
    const searchInput = $("input", {
      type: "text",
      placeholder: "Search channels...",
      class: "search-input",
    }) as HTMLInputElement;

    let searchTimeout: ReturnType<typeof setTimeout>;
    searchInput.addEventListener("input", () => {
      clearTimeout(searchTimeout);
      searchTimeout = setTimeout(() => {
        this.requestListRooms(searchInput.value.trim());
      }, 300);
    });

    searchContainer.appendChild(searchInput);
    modal.appendChild(searchContainer);

    // Channel list container
    const listContainer = $("div", { class: "channel-list-container" });
    this.renderChannelList(listContainer, rooms, isMember);
    modal.appendChild(listContainer);

    const buttonRow = $("div", { class: "button-row" });
    const closeBtn = $("button", {
      type: "button",
      class: "btn btn-secondary",
      text: "Close",
    });
    closeBtn.addEventListener("click", () => this.closeModal());
    buttonRow.appendChild(closeBtn);
    modal.appendChild(buttonRow);

    // Focus the search input
    searchInput.focus();
  }

  /**
   * Render the channel list (used by browse channels modal)
   */
  renderChannelList(
    container: Element,
    rooms: Room[],
    isMember: boolean[],
  ): void {
    container.innerHTML = "";

    if (rooms.length === 0) {
      const emptyState = $("p", {
        class: "empty-state",
        text: "No channels found. Try a different search or create a new channel.",
      });
      container.appendChild(emptyState);
      return;
    }

    const list = $("ul", { class: "channel-list" });

    for (let i = 0; i < rooms.length; i++) {
      const room = rooms[i];
      const member = isMember[i];

      const li = $("li", { class: "channel-list-item" });
      const nameSpan = $("span", {
        class: "channel-name",
        text: `# ${room.name}`,
      });

      li.appendChild(nameSpan);

      if (member) {
        const badge = $("span", {
          class: "badge badge-member",
          text: "Joined",
        });
        li.appendChild(badge);
      } else {
        const joinBtn = $("button", {
          class: "btn btn-small btn-primary",
          text: "Join",
        });
        joinBtn.addEventListener("click", () => this.joinRoom(room.id));
        li.appendChild(joinBtn);
      }

      list.appendChild(li);
    }

    container.appendChild(list);
  }

  /**
   * Show modal with room info and members
   */
  showRoomInfoModal(info: RoomInfoResponse) {
    const isDM = info.room.room_type === "dm";
    const title = isDM
      ? this.getDMDisplayName(info.room)
      : `# ${info.room.name}`;
    const modal = this.createModal(title);

    const content = $("div", { class: "room-info-content" });

    // Room details section
    const details = $("div", { class: "room-info-details" });

    const createdRow = $("div", { class: "info-row" });
    createdRow.appendChild(
      $("span", { class: "info-label", text: "Created:" }),
    );
    createdRow.appendChild(
      $("span", { class: "info-value", text: formatDate(info.created_at) }),
    );
    details.appendChild(createdRow);

    const memberCountRow = $("div", { class: "info-row" });
    memberCountRow.appendChild(
      $("span", { class: "info-label", text: "Members:" }),
    );
    memberCountRow.appendChild(
      $("span", {
        class: "info-value",
        text: `${info.member_count} member${info.member_count !== 1 ? "s" : ""}`,
      }),
    );
    details.appendChild(memberCountRow);

    if (info.room.is_private) {
      const privateRow = $("div", { class: "info-row" });
      privateRow.appendChild(
        $("span", { class: "badge badge-private", text: "Private" }),
      );
      details.appendChild(privateRow);
    }

    content.appendChild(details);

    // Members section
    const membersSection = $("div", { class: "room-members-section" });
    membersSection.appendChild(
      $("h4", { class: "section-title", text: "Members" }),
    );

    const membersList = $("ul", { class: "members-list" });
    for (const member of info.members) {
      const li = $("li", { class: "member-item clickable" });

      // Avatar
      const avatar = this.createAvatar(member.username);
      avatar.classList.add("member-avatar");
      li.appendChild(avatar);

      // Username (show display_name if available)
      const displayName = member.display_name || member.username;
      const usernameSpan = $("span", {
        class: "member-username",
        text: displayName,
      });
      li.appendChild(usernameSpan);

      // Mark current user
      if (member.id === this.state.user.id) {
        const youBadge = $("span", { class: "badge badge-you", text: "You" });
        li.appendChild(youBadge);
      }

      // Make the whole item clickable to view profile
      li.addEventListener("click", () => {
        this.closeModal();
        this.requestProfile(member.id);
      });

      membersList.appendChild(li);
    }
    membersSection.appendChild(membersList);
    content.appendChild(membersSection);

    modal.appendChild(content);

    // Button row with Leave button (unless it's a 1:1 DM or the default room)
    const buttonRow = $("div", { class: "button-row" });

    // For 1:1 DMs, don't show leave button (server will reject anyway)
    // For group DMs (3+ members), allow leaving
    // For channels, always show leave (server will reject for default room)
    const is1to1DM = isDM && info.member_count === 2;
    if (!is1to1DM) {
      const leaveBtn = $("button", {
        type: "button",
        class: "btn btn-danger",
        text: isDM ? "Leave Conversation" : "Leave Channel",
      });
      leaveBtn.addEventListener("click", () => {
        this.requestLeaveRoom(info.room.id);
      });
      buttonRow.appendChild(leaveBtn);
    }

    const closeBtn = $("button", {
      type: "button",
      class: "btn btn-secondary",
      text: "Close",
    });
    closeBtn.addEventListener("click", () => this.closeModal());
    buttonRow.appendChild(closeBtn);

    modal.appendChild(buttonRow);
  }

  // Track selected users in the new message modal
  private selectedUsersForDM: User[] = [];

  /**
   * Show modal for starting a new DM
   */
  showNewMessageModal() {
    this.selectedUsersForDM = [];

    const modal = this.createModal("New message");

    const content = $("div", { class: "new-message-content" });

    // "To:" row with tag input
    const toRow = $("div", { class: "to-row" });
    const toLabel = $("span", { class: "to-label", text: "To:" });
    toRow.appendChild(toLabel);

    const tagsContainer = $("div", { class: "user-tags-container" });
    const searchInput = $("input", {
      type: "text",
      class: "user-search-input",
      placeholder: "Search for users...",
      id: "dm-user-search",
    }) as HTMLInputElement;

    tagsContainer.appendChild(searchInput);
    toRow.appendChild(tagsContainer);
    content.appendChild(toRow);

    // User search results dropdown (initially hidden)
    const resultsDropdown = $("div", {
      class: "user-search-results",
      id: "dm-user-results",
    });
    resultsDropdown.style.display = "none";
    content.appendChild(resultsDropdown);

    // Search input handler with debounce
    let searchTimeout: ReturnType<typeof setTimeout>;
    searchInput.addEventListener("input", () => {
      clearTimeout(searchTimeout);
      const query = searchInput.value.trim();
      if (query.length > 0) {
        searchTimeout = setTimeout(() => {
          this.requestListUsers(query);
        }, 200);
      } else {
        resultsDropdown.style.display = "none";
      }
    });

    // Handle keyboard navigation
    searchInput.addEventListener("keydown", (e) => {
      if (e.key === "Backspace" && searchInput.value === "") {
        // Remove last selected user
        if (this.selectedUsersForDM.length > 0) {
          this.selectedUsersForDM.pop();
          this.renderSelectedUserTags();
        }
      }
    });

    modal.appendChild(content);

    // Button row
    const buttonRow = $("div", { class: "button-row" });
    const cancelBtn = $("button", {
      type: "button",
      class: "btn btn-secondary",
      text: "Cancel",
    });
    cancelBtn.addEventListener("click", () => this.closeModal());

    const startBtn = $("button", {
      type: "button",
      class: "btn btn-primary",
      text: "Start Conversation",
      id: "start-dm-btn",
    }) as HTMLButtonElement;
    startBtn.disabled = true;
    startBtn.addEventListener("click", () => {
      if (this.selectedUsersForDM.length > 0) {
        const userIds = this.selectedUsersForDM.map((u) => u.id);
        this.requestCreateDM(userIds);
      }
    });

    buttonRow.appendChild(cancelBtn);
    buttonRow.appendChild(startBtn);
    modal.appendChild(buttonRow);

    // Focus the search input
    searchInput.focus();
  }

  /**
   * Update user picker results in the new message modal
   */
  updateUserPickerResults(users: User[]) {
    const resultsDropdown = document.getElementById("dm-user-results");
    if (!resultsDropdown) return;

    resultsDropdown.innerHTML = "";

    // Filter out already selected users and self
    const availableUsers = users.filter(
      (u) =>
        u.id !== this.state.user.id &&
        !this.selectedUsersForDM.some((s) => s.id === u.id),
    );

    if (availableUsers.length === 0) {
      resultsDropdown.style.display = "none";
      return;
    }

    resultsDropdown.style.display = "block";

    for (const user of availableUsers) {
      const item = $("div", { class: "user-result-item" });

      // Avatar
      const avatar = this.createAvatar(user.username);
      avatar.classList.add("user-result-avatar");
      item.appendChild(avatar);

      // Name info
      const nameContainer = $("div", { class: "user-result-name" });
      const displayName = user.display_name || user.username;
      const nameSpan = $("span", {
        class: "user-result-display-name",
        text: displayName,
      });
      nameContainer.appendChild(nameSpan);

      if (user.display_name) {
        const usernameSpan = $("span", {
          class: "user-result-username",
          text: ` @${user.username}`,
        });
        nameContainer.appendChild(usernameSpan);
      }

      item.appendChild(nameContainer);

      // Click handler to select user
      item.addEventListener("click", () => {
        this.selectUserForDM(user);
      });

      resultsDropdown.appendChild(item);
    }
  }

  /**
   * Select a user for the DM
   */
  selectUserForDM(user: User) {
    // Check if already selected
    if (this.selectedUsersForDM.some((u) => u.id === user.id)) {
      return;
    }

    this.selectedUsersForDM.push(user);
    this.renderSelectedUserTags();

    // Clear the search and hide results
    const searchInput = document.getElementById(
      "dm-user-search",
    ) as HTMLInputElement;
    if (searchInput) {
      searchInput.value = "";
      searchInput.focus();
    }
    const resultsDropdown = document.getElementById("dm-user-results");
    if (resultsDropdown) {
      resultsDropdown.style.display = "none";
    }

    // Enable/disable start button
    this.updateStartDMButton();
  }

  /**
   * Remove a selected user from the DM
   */
  removeUserFromDM(userId: string) {
    this.selectedUsersForDM = this.selectedUsersForDM.filter(
      (u) => u.id !== userId,
    );
    this.renderSelectedUserTags();
    this.updateStartDMButton();
  }

  /**
   * Render the selected user tags in the new message modal
   */
  renderSelectedUserTags() {
    const container = document.querySelector(".user-tags-container");
    if (!container) return;

    // Remove existing tags (keep the input)
    const existingTags = container.querySelectorAll(".user-tag");
    for (const tag of existingTags) {
      tag.remove();
    }

    // Add tags for selected users (before the input)
    const input = container.querySelector(".user-search-input");
    for (const user of this.selectedUsersForDM) {
      const tag = $("span", { class: "user-tag" });
      const name = user.display_name || user.username;
      tag.appendChild(document.createTextNode(name));

      const removeBtn = $("button", {
        class: "user-tag-remove",
        text: "×",
        type: "button",
      });
      removeBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        this.removeUserFromDM(user.id);
      });
      tag.appendChild(removeBtn);

      container.insertBefore(tag, input);
    }
  }

  /**
   * Update the Start Conversation button state
   */
  updateStartDMButton() {
    const btn = document.getElementById("start-dm-btn") as HTMLButtonElement;
    if (btn) {
      btn.disabled = this.selectedUsersForDM.length === 0;
    }
  }

  /**
   * Show modal to view a user's profile
   */
  showProfileModal(user: User) {
    const displayName = user.display_name || user.username;
    const modal = this.createModal("Profile");

    const content = $("div", { class: "profile-content" });

    // Avatar and name section
    const header = $("div", { class: "profile-header" });
    const avatar = this.createAvatar(user.username);
    avatar.classList.add("profile-avatar");
    header.appendChild(avatar);

    const nameSection = $("div", { class: "profile-name-section" });
    nameSection.appendChild(
      $("span", { class: "profile-display-name", text: displayName }),
    );
    nameSection.appendChild(
      $("span", { class: "profile-username", text: `@${user.username}` }),
    );
    header.appendChild(nameSection);

    content.appendChild(header);

    // Status (if set)
    if (user.status) {
      const statusSection = $("div", { class: "profile-status-section" });
      statusSection.appendChild(
        $("span", { class: "profile-status-label", text: "Status" }),
      );
      statusSection.appendChild(
        $("span", { class: "profile-status", text: user.status }),
      );
      content.appendChild(statusSection);
    }

    modal.appendChild(content);

    // Only show button row if viewing someone else's profile (for Message button)
    if (user.id !== this.state.user.id) {
      const buttonRow = $("div", { class: "button-row" });
      const messageBtn = $("button", {
        type: "button",
        class: "btn btn-primary",
        text: "Message",
      });
      messageBtn.addEventListener("click", () => {
        this.closeModal();
        this.requestCreateDM([user.id]);
      });
      buttonRow.appendChild(messageBtn);
      modal.appendChild(buttonRow);
    }

    // Mark as a profile modal for styling (no scrollbars, no close button)
    modal.classList.add("modal-profile");
  }

  /**
   * Show modal to edit current user's profile
   */
  showEditProfileModal() {
    const modal = this.createModal("Edit Profile");

    const form = $("form", { class: "modal-form" });

    // Display name
    const nameLabel = $("label", {
      text: "Display name",
      for: "profile-display-name",
    });
    const nameInput = $("input", {
      type: "text",
      id: "profile-display-name",
      name: "display_name",
      placeholder: "Your display name",
      value: this.state.user.display_name || "",
      maxlength: "50",
    }) as HTMLInputElement;

    // Status
    const statusLabel = $("label", { text: "Status", for: "profile-status" });
    const statusInput = $("input", {
      type: "text",
      id: "profile-status",
      name: "status",
      placeholder: "What's on your mind?",
      value: this.state.user.status || "",
      maxlength: "100",
    }) as HTMLInputElement;

    const buttonRow = $("div", { class: "button-row" });
    const cancelBtn = $("button", {
      type: "button",
      class: "btn btn-secondary",
      text: "Cancel",
    });
    const saveBtn = $("button", {
      type: "submit",
      class: "btn btn-primary",
      text: "Save",
    });

    cancelBtn.addEventListener("click", () => this.closeModal());

    form.addEventListener("submit", (e) => {
      e.preventDefault();
      const displayName = nameInput.value.trim();
      const status = statusInput.value.trim();
      this.requestUpdateProfile(displayName, status);
    });

    buttonRow.appendChild(cancelBtn);
    buttonRow.appendChild(saveBtn);

    form.appendChild(nameLabel);
    form.appendChild(nameInput);
    form.appendChild(statusLabel);
    form.appendChild(statusInput);
    form.appendChild(buttonRow);

    modal.appendChild(form);

    // Focus the display name input
    nameInput.focus();
  }

  /**
   * Create a modal container with title.
   * Returns the modal content element (not the overlay) for appending content.
   * The overlay is automatically added to the DOM.
   */
  createModal(title: string): HTMLElement {
    // Close any existing modal first
    this.closeModal();

    const overlay = $("div", { class: "modal-overlay" });
    const modal = $("div", { class: "modal" });
    const header = $("div", { class: "modal-header" });
    const titleEl = $("h3", { text: title });
    const closeBtn = $("button", { class: "modal-close", text: "×" });

    closeBtn.addEventListener("click", () => this.closeModal());
    overlay.addEventListener("click", (e) => {
      if (e.target === overlay) {
        this.closeModal();
      }
    });

    // Close on Escape key
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        this.closeModal();
        document.removeEventListener("keydown", handleEscape);
      }
    };
    document.addEventListener("keydown", handleEscape);

    header.appendChild(titleEl);
    header.appendChild(closeBtn);
    modal.appendChild(header);
    overlay.appendChild(modal);

    // Add overlay to DOM
    document.body.appendChild(overlay);

    // Return the modal content element for appending content
    return modal;
  }

  /**
   * Close any open modal
   */
  closeModal() {
    const modal = document.querySelector(".modal-overlay");
    if (modal) {
      modal.remove();
    }
  }

  updateSidebarHighlight() {
    // Remove active class from all rooms
    const roomItems = document.querySelectorAll(".sidebar-channels li");
    for (const item of roomItems) {
      item.classList.remove("active");
    }

    // Add active class to current room
    const activeItem = document.querySelector(
      `.sidebar-channels li[data-room-id="${this.state.currentRoom}"]`,
    );
    if (activeItem) {
      activeItem.classList.add("active");
    }
  }

  updateChatHeader() {
    const header = document.querySelector(".chat-header");
    if (!header) return;

    const room = this.state.getRoom(this.state.currentRoom || "");
    if (!room) return;

    // Clear header content for fresh render
    header.innerHTML = "";

    // Create header content
    const h2 = $("h2", {}) as HTMLHeadingElement;

    if (room.room_type === "dm") {
      // DM: show user names
      h2.textContent = this.getDMDisplayName(room);
    } else {
      // Channel: show # prefix
      h2.textContent = `# ${room.name}`;
    }
    header.appendChild(h2);

    // Add info button
    const infoBtn = $("button", {
      class: "room-info-btn",
      title:
        room.room_type === "dm"
          ? "View conversation details"
          : "View channel details",
      text: "ℹ️",
    });
    infoBtn.addEventListener("click", () => {
      if (this.state.currentRoom) {
        this.requestRoomInfo(this.state.currentRoom);
      }
    });
    header.appendChild(infoBtn);
  }

  clearMessageUI() {
    const messageWindow = document.querySelector(".chat-messages");
    if (messageWindow) {
      messageWindow.innerHTML = "";
    }
  }

  submitTextbox() {
    const messageBox = document.querySelector(
      "#message",
    ) as HTMLTextAreaElement;
    if (!messageBox) {
      throw new Error("couldn't find message box");
    }
    if (!messageBox.value) {
      console.debug("empty message found, doing nothing");
      return;
    }

    const roomID = this.state.currentRoom;
    if (!roomID) {
      console.error("no current room set");
      return;
    }

    const body = messageBox.value;
    const user = this.state.user;

    const message = {
      type: "message",
      data: {
        body: body,
        room_id: roomID,
      },
    };
    console.debug("sending", message);
    this.conn.send(JSON.stringify(message));

    // Create optimistic message
    const optimisticMsg: Message = {
      id: `pending-${Date.now()}`,
      room_id: roomID,
      user_id: user.id,
      username: user.username,
      body: body,
      created_at: new Date().toISOString(),
      modified_at: new Date().toISOString(),
    };

    // Render immediately
    const element = this.appendOptimisticMessage(optimisticMsg);

    // Track the pending message so we can match it when the server confirms
    const pendingKey = makePendingKey(body, roomID, user.id);
    this.pendingMessages.set(pendingKey, {
      tempId: pendingKey,
      body: body,
      roomId: roomID,
      element: element,
    });

    // Clear the input box and reset height
    messageBox.value = "";
    messageBox.style.height = "auto";
  }

  /**
   * Append an optimistic (pending) message to the UI
   */
  appendOptimisticMessage(msg: Message): HTMLElement {
    const messageWindow = document.querySelector(".chat-messages");
    if (!messageWindow) {
      throw new Error("no message window found");
    }

    // Get previous message for grouping
    const roomState = this.state.getCurrentRoomState();
    const messages = roomState?.messages || [];
    const prevMsg =
      messages.length > 0 ? messages[messages.length - 1] : undefined;

    const isGrouped = this.shouldGroupWithPrevious(msg, prevMsg);
    const element = this.createMessageElement(msg, isGrouped, true);
    element.classList.add("pending");

    messageWindow.appendChild(element);
    messageWindow.scrollTop = messageWindow.scrollHeight;

    return element;
  }

  onSubmit(_evt: MouseEvent) {
    this.submitTextbox();
  }

  onKeydown(e: KeyboardEvent) {
    const messageBox = document.querySelector(
      "#message",
    ) as HTMLTextAreaElement;
    if (!messageBox) return;

    // Let autocomplete handle keys when it's active
    if (this.autocomplete?.isActive) {
      if (["ArrowDown", "ArrowUp", "Enter", "Tab", "Escape"].includes(e.key)) {
        // Autocomplete's own keydown handler will handle these
        return;
      }
    }

    if (e.key === "Enter" && !e.shiftKey) {
      // Enter sends, Shift+Enter inserts newline
      e.preventDefault();
      this.submitTextbox();
    } else if (e.key === "ArrowUp" && messageBox.value === "") {
      // Up arrow in empty input → edit last own message
      e.preventDefault();
      const roomState = this.state.getCurrentRoomState();
      if (!roomState) return;

      for (let i = roomState.messages.length - 1; i >= 0; i--) {
        const msg = roomState.messages[i];
        if (msg.user_id === this.state.user.id && !msg.deleted_at) {
          this.startEdit(msg.id);
          break;
        }
      }
    }
  }

  /**
   * Auto-resize textarea to fit content
   */
  autoResizeInput() {
    const messageBox = document.querySelector(
      "#message",
    ) as HTMLTextAreaElement;
    if (!messageBox) return;
    messageBox.style.height = "auto";
    // Only set explicit height if content exceeds the default single-line height
    if (messageBox.scrollHeight > messageBox.clientHeight) {
      messageBox.style.height = `${Math.min(messageBox.scrollHeight, 200)}px`;
    }
  }
}

function main() {
  const conn = new WebSocket(`ws://${document.location.host}/ws`);
  const client = new Client(conn);

  // Expose WebSocket for e2e testing
  // biome-ignore lint/suspicious/noExplicitAny: needed for e2e test access
  (window as any).__ws = conn;

  // Handle browser back/forward navigation
  window.addEventListener("popstate", (evt) => {
    const roomId = evt.state?.roomId;
    if (roomId && roomId !== client.state.currentRoom) {
      client.switchRoom(roomId);
    }
  });

  // Global keyboard shortcuts (Cmd+K / Ctrl+K for quick-search)
  document.addEventListener("keydown", client.handleGlobalKeydown.bind(client));

  document
    .getElementById("message")
    ?.addEventListener("keydown", client.onKeydown.bind(client));
  document
    .getElementById("message")
    ?.addEventListener("input", client.autoResizeInput.bind(client));
  document
    .getElementById("sendmessage")
    ?.addEventListener("click", client.onSubmit.bind(client));
}

window.addEventListener("DOMContentLoaded", async () => {
  main();
});
