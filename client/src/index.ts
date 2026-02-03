import { $ } from "./dom";
import { AppState } from "./state";
import {
  type CreateDMResponse,
  type CreateRoomResponse,
  type GetProfileResponse,
  type HistoryResponse,
  type InitResponse,
  type JoinRoomResponse,
  type LeaveRoomResponse,
  type ListRoomsResponse,
  type ListUsersResponse,
  type Message,
  makePendingKey,
  type PendingMessage,
  parseServerEnvelope,
  type Room,
  type RoomInfoResponse,
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

class Client {
  conn: WebSocket;
  state: AppState;

  // Track pending messages waiting for server confirmation
  pendingMessages: Map<string, PendingMessage> = new Map();

  // Track loading state
  isLoadingHistory: boolean = false;

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

    // Get room ID from URL or use the current_room from init
    const parts = window.location.pathname.split("/");
    const urlRoomID = parts[parts.length - 1];
    this.state.setCurrentRoom(urlRoomID || data.current_room);

    // Render the sidebar with rooms
    this.renderSidebar();

    // Request history for the current room
    if (this.state.currentRoom) {
      this.requestHistory(this.state.currentRoom);
      this.updateChatHeader();
    }
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
    const wrapper = $("div", {
      class: `chat-message ${isGrouped ? "grouped" : ""} ${isOwn ? "own-message" : ""}`,
      "data-message-id": msg.id,
    });

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

      const timestamp = $("span", {
        class: "message-timestamp",
        text: formatTimestamp(msg.created_at),
        title: formatTimestampFull(msg.created_at),
      });

      header.appendChild(usernameEl);
      header.appendChild(timestamp);

      const content = $("div", { class: "message-content" });
      content.appendChild(avatar);

      const textArea = $("div", { class: "message-text-area" });
      textArea.appendChild(header);
      textArea.appendChild($("div", { class: "message-body", text: msg.body }));
      content.appendChild(textArea);

      wrapper.appendChild(content);
    } else {
      // Grouped message - just the body with indent to align with text
      const content = $("div", { class: "message-content grouped-content" });
      const body = $("div", { class: "message-body", text: msg.body });

      // Add timestamp on hover
      const timestamp = $("span", {
        class: "message-timestamp hover-timestamp",
        text: formatTimestamp(msg.created_at),
        title: formatTimestampFull(msg.created_at),
      });

      content.appendChild(timestamp);
      content.appendChild(body);
      wrapper.appendChild(content);
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
    this.showBrowseChannelsModal(response.rooms, response.is_member);
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

    // Button row
    const buttonRow = $("div", { class: "button-row" });

    // Only show "Message" button if viewing someone else's profile
    if (user.id !== this.state.user.id) {
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
    const messageBox = document.querySelector("#message") as HTMLInputElement;
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

    // Clear the input box
    messageBox.value = "";
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

  onKeypress(e: KeyboardEvent) {
    if (e.key === "Enter") {
      this.submitTextbox();
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

  document
    .getElementById("message")
    ?.addEventListener("keypress", client.onKeypress.bind(client));
  document
    .getElementById("sendmessage")
    ?.addEventListener("click", client.onSubmit.bind(client));
}

window.addEventListener("DOMContentLoaded", async () => {
  main();
});
