import { $ } from "./dom";
import { AppState } from "./state";
import {
  type HistoryResponse,
  type InitialData,
  type InitResponse,
  type Message,
  makePendingKey,
  type PendingMessage,
} from "./types";
import {
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
      const body = JSON.parse(evt.data);
      switch (body.Type) {
        case "init": {
          this.handleInit(body.Data as InitialData);
          break;
        }
        case "history": {
          this.handleHistory(body.Data as HistoryResponse);
          break;
        }
        case "message": {
          // Handle incoming message - could be from us (confirmation) or others
          this.handleIncomingMessage(body.Data as Message);
          break;
        }
        case "error": {
          console.error("server error:", body.Data);
          break;
        }
      }
      console.debug("received: ", body);
    } catch (e) {
      console.error("unable to parse", evt.data, e);
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
    const header = document.querySelector(".chat-header h2");
    if (!header) return;

    const room = this.state.getRoom(this.state.currentRoom || "");
    if (room) {
      header.textContent = `# ${room.name}`;
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
