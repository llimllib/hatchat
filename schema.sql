CREATE TABLE IF NOT EXISTS users(
  id TEXT PRIMARY KEY NOT NULL,
  username TEXT NOT NULL,
  password TEXT NOT NULL,
  active BOOL, -- true if the user has been recently active
  avatar TEXT, -- the URL of an avatar image
  created_at TIMESTAMP NOT NULL,
  modified_at TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS users_username ON users(username);

CREATE TABLE IF NOT EXISTS sessions(
  id TEXT PRIMARY KEY NOT NULL,
  user_id TEXT REFERENCES users(id) NOT NULL,
  created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS room_members(
  user_id TEXT REFERENCES users(id) NOT NULL,
  room_id TEXT REFERENCES rooms(id) NOT NULL,
  PRIMARY KEY (user_id, room_id)
);

CREATE TABLE IF NOT EXISTS rooms(
  id TEXT PRIMARY KEY NOT NULL,
  name TEXT NOT NULL,
  is_private BOOL NOT NULL,
  created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS messages(
  id TEXT PRIMARY KEY NOT NULL,
  room_id REFERENCES rooms(id) NOT NULL,
  user_id REFERENCES users(id) NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  modified_at TIMESTAMP NOT NULL
);
