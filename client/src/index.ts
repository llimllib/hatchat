function $(
  tagName: string,
  attributes?: Record<string, string>,
  ...children: (HTMLElement | Text)[]
): HTMLElement {
  const elt = document.createElement(tagName);
  if (attributes) {
    for (const [key, val] of Object.entries(attributes)) {
      switch (key) {
        case "text":
          elt.innerText = val;
          break;
        default:
          elt.setAttribute(key, val || "");
      }
    }
  }
  for (const child of children) {
    elt.appendChild(child);
  }

  return elt;
}

function text(s: string): Text {
  return document.createTextNode(s);
}

interface Room {
  ID: string;
}

interface InitialData {
  Rooms: Room[];
  User: {
    id: `usr_${string}`;
    username: string;
    avatar: string;
  };
  current_room: string;
}

interface HistoryMessage {
  id: string;
  room_id: string;
  user_id: string;
  username: string;
  body: string;
  created_at: string;
  modified_at: string;
}

interface HistoryResponse {
  messages: HistoryMessage[];
  has_more: boolean;
  next_cursor: string;
}

class Client {
  conn: WebSocket;

  initialData?: InitialData;
  currentRoom?: string;
  historyCursor?: string;
  hasMoreHistory: boolean = false;
  isLoadingHistory: boolean = false;

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

          // Request history for the current room
          if (this.currentRoom) {
            this.requestHistory(this.currentRoom);
          }
          break;
        }
        case "history": {
          this.handleHistory(body.Data as HistoryResponse);
          break;
        }
        case "message": {
          // Handle incoming message from other users
          this.appendMessage(body.Data.body, body.Data.username || "unknown");
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

  appendMessage(body: string, username: string) {
    const messageWindow = document.querySelector(".chat-messages");
    if (!messageWindow) {
      console.error("no message window found");
      return;
    }
    messageWindow.appendChild(this.createMessageElement(username, body));

    // Scroll to the bottom to show the new message
    messageWindow.scrollTop = messageWindow.scrollHeight;
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

    const message = {
      type: "message",
      data: {
        body: messageBox.value,
        room_id: roomID,
      },
    };
    console.debug("sending", message);
    this.conn.send(JSON.stringify(message));

    // Optimistically insert chat message into the chat window
    this.appendMessage(messageBox.value, this.initialData.User.username);

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
