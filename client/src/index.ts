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
    const message = evt.target.parentElement?.querySelector(
      "#message",
    ) as HTMLInputElement;
    if (!message.value) {
      console.debug("empty message found, doing nothing");
      return;
    }
    this.conn.send(
      JSON.stringify({
        type: "message",
        data: {
          body: message,
        },
      }),
    );
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
