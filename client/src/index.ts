import { $, text } from "./dom";
import {
  type HistoryResponse,
  type InitialData,
  type Message,
  makePendingKey,
  type PendingMessage,
} from "./types";

class Client {
  conn: WebSocket;

  initialData?: InitialData;
  currentRoom?: string;
  historyCursor?: string;
  hasMoreHistory: boolean = false;
  isLoadingHistory: boolean = false;

  // Track pending messages waiting for server confirmation
  pendingMessages: Map<string, PendingMessage> = new Map();

  constructor(conn: WebSocket) {
    this.conn = conn;

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
          this.initialData = body.Data;
          // Get room ID from URL or use the current_room from init
          const parts = window.location.pathname.split("/");
          const urlRoomID = parts[parts.length - 1];
          this.currentRoom = urlRoomID || body.Data.current_room;

          // Render the sidebar with rooms
          this.renderSidebar();

          // Request history for the current room
          if (this.currentRoom) {
            this.requestHistory(this.currentRoom);
            this.updateChatHeader();
          }
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
    this.hasMoreHistory = response.has_more;
    this.historyCursor = response.next_cursor;

    const messageWindow = document.querySelector(".chat-messages");
    if (!messageWindow) {
      console.error("no message window found");
      return;
    }

    // Messages come in newest-first order, we need to display oldest-first
    // Reverse the array to get chronological order
    const messages = [...response.messages].reverse();

    // If this is the first load (no cursor was used), clear the message window
    // Otherwise, prepend to existing messages
    const isFirstLoad = !this.historyCursor || messages.length === 0;

    if (isFirstLoad && response.messages.length > 0) {
      // Clear any placeholder content
      messageWindow.innerHTML = "";
    }

    // Create a document fragment for efficient DOM manipulation
    const fragment = document.createDocumentFragment();

    for (const msg of messages) {
      fragment.appendChild(this.createMessageElement(msg.username, msg.body));
    }

    if (isFirstLoad) {
      messageWindow.appendChild(fragment);
    } else {
      // Prepend older messages at the top
      messageWindow.insertBefore(fragment, messageWindow.firstChild);
    }

    // Add "Load more" button if there are more messages
    this.updateLoadMoreButton(messageWindow);
  }

  updateLoadMoreButton(messageWindow: Element) {
    // Remove existing load more button if present
    const existingButton = document.querySelector(".load-more-button");
    if (existingButton) {
      existingButton.remove();
    }

    if (this.hasMoreHistory) {
      const loadMoreBtn = $("button", {
        text: "Load older messages",
        class: "load-more-button",
      });
      loadMoreBtn.addEventListener("click", () => {
        if (this.currentRoom && this.historyCursor) {
          this.requestHistory(this.currentRoom, this.historyCursor);
        }
      });
      messageWindow.insertBefore(loadMoreBtn, messageWindow.firstChild);
    }
  }

  createMessageElement(username: string, body: string): HTMLElement {
    return $(
      "div",
      { class: "chat-message" },
      $("span", {
        text: username,
        class: "username",
      }),
      text(": "),
      $("span", {
        text: body,
        class: "message",
      }),
    );
  }

  handleIncomingMessage(msg: Message) {
    // Check if this is a confirmation of our pending message
    // We match by body + room_id + user_id since we don't have the server ID yet
    const pendingKey = makePendingKey(msg.body, msg.room_id, msg.user_id);
    const pending = this.pendingMessages.get(pendingKey);

    if (pending) {
      // This is our message confirmed by the server - update the element with real data
      pending.element.setAttribute("data-message-id", msg.id);
      pending.element.classList.remove("pending");
      this.pendingMessages.delete(pendingKey);
      console.debug("confirmed pending message", msg.id);
    } else {
      // This is a message from someone else - append it
      this.appendMessage(msg.body, msg.username);
    }
  }

  appendMessage(body: string, username: string): HTMLElement {
    const messageWindow = document.querySelector(".chat-messages");
    if (!messageWindow) {
      console.error("no message window found");
      throw new Error("no message window found");
    }
    const element = this.createMessageElement(username, body);
    messageWindow.appendChild(element);

    // Scroll to the bottom to show the new message
    messageWindow.scrollTop = messageWindow.scrollHeight;

    return element;
  }

  renderSidebar() {
    if (!this.initialData) {
      return;
    }

    const channelList = document.querySelector(".sidebar-channels ul");
    if (!channelList) {
      console.error("no channel list found");
      return;
    }

    // Clear existing placeholder channels
    channelList.innerHTML = "";

    // Render each room
    for (const room of this.initialData.Rooms) {
      const li = $("li", { "data-room-id": room.id });
      const link = $("a", {
        href: `/chat/${room.id}`,
        text: `# ${room.name}`,
      });

      // Mark the active room
      if (room.id === this.currentRoom) {
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
    if (roomId === this.currentRoom) {
      return;
    }

    // Update current room
    this.currentRoom = roomId;

    // Update URL without reload
    window.history.pushState({}, "", `/chat/${roomId}`);

    // Update sidebar highlighting
    this.updateSidebarHighlight();

    // Update chat header
    this.updateChatHeader();

    // Clear messages and reset pagination state
    this.clearMessages();
    this.historyCursor = undefined;
    this.hasMoreHistory = false;

    // Request history for new room
    this.requestHistory(roomId);
  }

  updateSidebarHighlight() {
    // Remove active class from all rooms
    const roomItems = document.querySelectorAll(".sidebar-channels li");
    for (const item of roomItems) {
      item.classList.remove("active");
    }

    // Add active class to current room
    const activeItem = document.querySelector(
      `.sidebar-channels li[data-room-id="${this.currentRoom}"]`,
    );
    if (activeItem) {
      activeItem.classList.add("active");
    }
  }

  updateChatHeader() {
    const header = document.querySelector(".chat-header h2");
    if (!header || !this.initialData) {
      return;
    }

    const room = this.initialData.Rooms.find((r) => r.id === this.currentRoom);
    if (room) {
      header.textContent = `# ${room.name}`;
    }
  }

  clearMessages() {
    const messageWindow = document.querySelector(".chat-messages");
    if (messageWindow) {
      messageWindow.innerHTML = "";
    }
    // Clear pending messages for the old room
    this.pendingMessages.clear();
  }

  submitTextbox() {
    // What is the appropriate thing to do if we haven't yet gotten the
    // initialize data, so we don't even know who the user is?
    if (!this.initialData) {
      // placeholder for sensible error handling
      throw new Error("Not yet initialized");
    }

    // get the message from the input box
    const messageBox = document.querySelector("#message") as HTMLInputElement;
    if (!messageBox) {
      throw new Error("couldn't find message box");
    }
    if (!messageBox.value) {
      console.debug("empty message found, doing nothing");
      return;
    }

    // get the room ID from the URL
    const parts = window.location.pathname.split("/");
    const roomID = parts[parts.length - 1];
    const body = messageBox.value;

    const message = {
      type: "message",
      data: {
        body: body,
        room_id: roomID,
      },
    };
    console.debug("sending", message);
    this.conn.send(JSON.stringify(message));

    // Optimistically insert chat message into the chat window
    const element = this.appendMessage(body, this.initialData.User.username);
    element.classList.add("pending");

    // Track the pending message so we can match it when the server confirms
    const pendingKey = makePendingKey(body, roomID, this.initialData.User.id);
    this.pendingMessages.set(pendingKey, {
      tempId: pendingKey,
      body: body,
      roomId: roomID,
      element: element,
    });

    // Clear the input box
    messageBox.value = "";
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

  document
    .getElementById("sendmessage")
    ?.addEventListener("keypress", client.onKeypress.bind(client));
  document
    .getElementById("sendmessage")
    ?.addEventListener("click", client.onSubmit.bind(client));
}

window.addEventListener("DOMContentLoaded", async () => {
  main();
});
