CREATE TABLE IF NOT EXISTS users(
  id TEXT PRIMARY KEY NOT NULL,
  username TEXT NOT NULL,
  password TEXT NOT NULL,
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
  name TEXT NOT NULL,
  is_private INTEGER NOT NULL,
  is_default INTEGER NOT NULL,
  created_at TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS messages(
  id TEXT PRIMARY KEY NOT NULL,
  room_id TEXT REFERENCES rooms(id) NOT NULL,
  user_id TEXT REFERENCES users(id) NOT NULL,
  body TEXT NOT NULL,
  created_at TEXT NOT NULL,
  modified_at TEXT NOT NULL
) STRICT;
