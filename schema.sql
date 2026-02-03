CREATE TABLE IF NOT EXISTS users(
  id TEXT PRIMARY KEY NOT NULL,
  username TEXT NOT NULL,
  password TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '', -- user's display name (shown instead of username if set)
  status TEXT NOT NULL DEFAULT '', -- custom status message
  active INTEGER, -- true if the user has been recently active
  avatar TEXT, -- the URL of an avatar image
  last_room TEXT NOT NULL, -- the id of last room the user was in
  created_at TEXT NOT NULL,
  modified_at TEXT NOT NULL
) STRICT;

CREATE UNIQUE INDEX IF NOT EXISTS users_username ON users(username);

CREATE TABLE IF NOT EXISTS sessions(
  id TEXT PRIMARY KEY NOT NULL,
  user_id TEXT REFERENCES users(id) NOT NULL,
  created_at TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS rooms_members(
  user_id TEXT REFERENCES users(id) NOT NULL,
  room_id TEXT REFERENCES rooms(id) NOT NULL,
  PRIMARY KEY (user_id, room_id)
) STRICT;

CREATE TABLE IF NOT EXISTS rooms(
  id TEXT PRIMARY KEY NOT NULL,
  name TEXT NOT NULL, -- empty for DMs; display name derived from members
  room_type TEXT NOT NULL DEFAULT 'channel', -- 'channel' or 'dm'
  is_private INTEGER NOT NULL,
  is_default INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  last_message_at TEXT -- for sorting DMs by most recent activity; NULL if no messages
) STRICT;

-- Unique room names, but only for channels (DMs can have empty names)
CREATE UNIQUE INDEX IF NOT EXISTS rooms_name ON rooms(name) WHERE room_type = 'channel' AND name != '';

CREATE TABLE IF NOT EXISTS messages(
  id TEXT PRIMARY KEY NOT NULL,
  room_id TEXT REFERENCES rooms(id) NOT NULL,
  user_id TEXT REFERENCES users(id) NOT NULL,
  body TEXT NOT NULL,
  created_at TEXT NOT NULL,
  modified_at TEXT NOT NULL,
  deleted_at TEXT -- NULL = not deleted, RFC3339 timestamp = soft-deleted
) STRICT;

-- Index for fetching messages by room, ordered by creation time (newest first for pagination)
CREATE INDEX IF NOT EXISTS messages_room_created ON messages(room_id, created_at DESC);

CREATE TABLE IF NOT EXISTS reactions(
  message_id TEXT REFERENCES messages(id) NOT NULL,
  user_id TEXT REFERENCES users(id) NOT NULL,
  emoji TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (message_id, user_id, emoji)
) STRICT;

CREATE INDEX IF NOT EXISTS reactions_message ON reactions(message_id);
