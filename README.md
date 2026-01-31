# Hatchat 🪓

The aim of Hatchat is to build a slack-like chat application on top of SQLite.

The goal is simplicity for running the server over more features.

## Server

To build a binary: `go build ./cmd/server.go -o hatchat`

If you have `just` installed, you can do `just build`

To start the server: `./hatchat`

Command line flags:

- `addr`: the host and port to listen on; defaults to `localhost:8080`
- `db`: the location for the chat database. Must be a url like `file:chat.db`
- `log-level`: the log level. `INFO` is default, other options are `DEBUG`, `WARN`, `ERROR`

### development

Run `modd` (to install [it](https://github.com/cortesi/modd): `go install github.com/cortesi/modd/cmd/modd`) to have the server rebuilt on every change.

### environment variables

- `LOG_LEVEL` - the log level. Default value `INFO`, valid values `DEBUG`, `INFO`, `WARN`, `ERROR`
