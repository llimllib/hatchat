# Tinychat

Starting from the [gorilla/websocket chat example](https://github.com/gorilla/websocket/tree/main/examples/chat)

## Server

To build a binary: `go build ./cmd/server.go -o tinychat`

If you have `just` installed, you can do `just build`

To start the server: `./tinychat`

Command line flags:

- `addr`: the host and port to listen on; defaults to `localhost:8080`
- `log-level`: the log level. `INFO` is default, other options are `DEBUG`, `WARN`, `ERROR`
- `db`: the location for the chat database. Must be a url like `file:chat.db`

### development

Run `modd` (to install [it](https://github.com/cortesi/modd): `go install github.com/cortesi/modd/cmd/modd`) to have the server rebuilt on every change.

### environment variables

- `LOG_LEVEL` - the log level. Default value `INFO`, valid values `DEBUG`, `INFO`, `WARN`, `ERROR`
