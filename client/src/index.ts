class Client {
  conn: WebSocket;

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
    const body = JSON.parse(evt.data);
    console.log("received: ", body);
  }

  wsOpen(evt: Event) {
    console.log("opened", evt);
    // TODO: type the stuff the websocket connection sends/receives
    this.conn.send(
      JSON.stringify({
        type: "init",
        data: {},
      }),
    );
  }

  onSubmit(evt: MouseEvent) {
    if (!(evt.target instanceof HTMLElement)) {
      return;
    }

    // get the message from the input box
    const messageText = evt.target.parentElement?.querySelector(
      "#message",
    ) as HTMLInputElement;
    if (!messageText.value) {
      console.debug("empty message found, doing nothing");
      return;
    }

    // get the room ID from the URL
    const parts = window.location.pathname.split("/");
    const roomID = parts[parts.length - 1];

    const message = {
      type: "message",
      data: {
        body: messageText.value,
        room: roomID,
      },
    };
    console.debug("sending", message);
    this.conn.send(JSON.stringify(message));
  }
}

function main() {
  const conn = new WebSocket(`ws://${document.location.host}/ws`);
  const client = new Client(conn);

  document
    .getElementById("sendmessage")
    ?.addEventListener("click", client.onSubmit.bind(client));
}

window.addEventListener("DOMContentLoaded", async () => {
  main();
});
