import { $ } from "./dom";
import { AppState } from "./state";
import {
  type CreateRoomResponse,
  type HistoryResponse,
  type InitResponse,
  type JoinRoomResponse,
  type LeaveRoomResponse,
  type ListRoomsResponse,
  type Message,
  makePendingKey,
  type PendingMessage,
  parseServerEnvelope,
  type Room,
  type RoomInfoResponse,
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
    // Only process if it's for the current room
    if (msg.room_id !== this.state.currentRoom) {
      // TODO: Update unread count for other room
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
    const channelList = document.querySelector(".sidebar-channels ul");
    if (!channelList) {
      console.error("no channel list found");
      return;
    }

    // Clear existing placeholder channels
    channelList.innerHTML = "";

    // Render each room
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

    // Add action buttons at the bottom
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
    const modal = this.createModal(`# ${info.room.name}`);

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
      const li = $("li", { class: "member-item" });

      // Avatar
      const avatar = this.createAvatar(member.username);
      avatar.classList.add("member-avatar");
      li.appendChild(avatar);

      // Username
      const usernameSpan = $("span", {
        class: "member-username",
        text: member.username,
      });
      li.appendChild(usernameSpan);

      // Mark current user
      if (member.id === this.state.user.id) {
        const youBadge = $("span", { class: "badge badge-you", text: "You" });
        li.appendChild(youBadge);
      }

      membersList.appendChild(li);
    }
    membersSection.appendChild(membersList);
    content.appendChild(membersSection);

    modal.appendChild(content);

    // Button row with Leave button (unless it's the default room)
    const buttonRow = $("div", { class: "button-row" });

    // Check if this is the default room (we'll need to track this)
    // For now, we don't have a way to know if it's the default room from the response
    // We could add it to the protocol, but for now we'll just always show the leave button
    // and let the server reject it
    const leaveBtn = $("button", {
      type: "button",
      class: "btn btn-danger",
      text: "Leave Channel",
    });
    leaveBtn.addEventListener("click", () => {
      this.requestLeaveRoom(info.room.id);
    });
    buttonRow.appendChild(leaveBtn);

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

    // Update or create the header content
    let h2 = header.querySelector("h2") as HTMLHeadingElement | null;
    if (!h2) {
      h2 = $("h2", {}) as HTMLHeadingElement;
      header.appendChild(h2);
    }
    h2.textContent = `# ${room.name}`;

    // Add info button if not present
    let infoBtn = header.querySelector(".room-info-btn");
    if (!infoBtn) {
      infoBtn = $("button", {
        class: "room-info-btn",
        title: "View channel details",
        text: "ℹ️",
      });
      infoBtn.addEventListener("click", () => {
        if (this.state.currentRoom) {
          this.requestRoomInfo(this.state.currentRoom);
        }
      });
      header.appendChild(infoBtn);
    }
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
