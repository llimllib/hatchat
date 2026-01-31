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
interface InitialData {
  // TODO
  Rooms: any;
  User: {
    id: `usr_${string}`;
    username: string;
    avatar: string;
  };
}

class Client {
  conn: WebSocket;

  initialData?: InitialData;

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
    }
    try {
      const body = JSON.parse(evt.data);
      switch (body.Type) {
        case "init": {
          this.initialData = body.Data;
        }
      }
      console.debug("received: ", body);
    } catch (e) {
      console.error("uanble to parse", evt.data, e);
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
    const messageWindow = document.querySelector(".chat-messages");
    if (!messageWindow) {
      throw new Error("no message window, somehow?");
    }

    // append the message to the window
    messageWindow.appendChild(
      $(
        "div",
        {},
        $("span", {
          text: this.initialData.User.username,
          class: "username",
        }),
        text(":"),
        $("span", {
          text: messageBox.value,
          class: "message",
        }),
      ),
    );
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
