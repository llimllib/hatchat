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

## Database Schema Versioning

The database schema is versioned to ensure safe deployments and rollbacks.

### How it works

- The `schema_version` table tracks the current database schema version
- On startup, the server checks if the database version is compatible
- **If DB version < server version**: Server refuses to start (run migrations first)
- **If DB version > server version**: Server starts with a warning (safe for rollbacks)

### Making schema changes

All schema changes must be **backward-compatible** to allow server rollbacks:

| ✅ Allowed | ❌ Not Allowed |
|-----------|---------------|
| Add columns with DEFAULT values | Remove columns |
| Add new tables | Rename columns |
| Add indexes | Change column types |

When modifying `schema.sql`:

1. Make your backward-compatible change
2. Increment `SchemaVersion` in `server/db/version.go`
3. Update the version in the `schema.sql` INSERT statement
4. Run `just models` to regenerate database models

CI will fail if you change the schema without bumping the version.
