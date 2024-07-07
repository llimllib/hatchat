function wsClosed(_: CloseEvent) {
  // TODO: try to reconnect
  console.warn("connection closed");
}

function wsReceive(evt: MessageEvent) {
  var body = JSON.parse(evt.data);
  console.log(body);
}

function main() {
  const conn = new WebSocket(`ws://${document.location.host}/ws`);
  conn.onclose = wsClosed;
  conn.onmessage = wsReceive;
}

window.addEventListener("DOMContentLoaded", async () => {
  main();
});
