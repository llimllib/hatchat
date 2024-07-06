CREATE TABLE IF NOT EXISTS users(
	id INT PRIMARY KEY NOT NULL,
	username TEXT,
	password TEXT,
	created_at TIMESTAMP,
	modified_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions(
	id TEXT PRIMARY KEY NOT NULL,
	username TEXT,
	created_at TIMESTAMP
);
