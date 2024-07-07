function wsClose(_: CloseEvent) {
  // TODO: try to reconnect
  console.warn("connection closed", _);
}

function wsReceive(evt: MessageEvent) {
  const body = JSON.parse(evt.data);
  console.log(body);
}

function wsOpen(evt: Event) {
  console.log("connection opened", evt);
}

function onSubmit(conn: WebSocket) {
  return (evt: MouseEvent) => {
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
    conn.send(
      JSON.stringify({
        type: "message",
        data: {
          body: message,
        },
      }),
    );
  };
}

function main() {
  const conn = new WebSocket(`ws://${document.location.host}/ws`);
  conn.addEventListener("open", wsOpen);
  conn.addEventListener("message", wsReceive);
  conn.addEventListener("close", wsClose);

  document
    .getElementById("sendmessage")
    ?.addEventListener("click", onSubmit(conn));
}

window.addEventListener("DOMContentLoaded", async () => {
  main();
});
