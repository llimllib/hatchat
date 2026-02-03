// Package testutil provides shared test helper functions for database setup and fixture creation.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

// TestSchema is the database schema for tests - kept in sync with schema.sql
const TestSchema = `
CREATE TABLE IF NOT EXISTS users(
	id TEXT PRIMARY KEY NOT NULL,
	username TEXT NOT NULL,
	password TEXT NOT NULL,
	display_name TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	active INTEGER,
	avatar TEXT,
	last_room TEXT NOT NULL,
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
	room_type TEXT NOT NULL DEFAULT 'channel',
	is_private INTEGER NOT NULL,
	is_default INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	last_message_at TEXT
) STRICT;

CREATE UNIQUE INDEX IF NOT EXISTS rooms_name ON rooms(name) WHERE room_type = 'channel' AND name != '';

CREATE TABLE IF NOT EXISTS messages(
	id TEXT PRIMARY KEY NOT NULL,
	room_id TEXT REFERENCES rooms(id) NOT NULL,
	user_id TEXT REFERENCES users(id) NOT NULL,
	body TEXT NOT NULL,
	created_at TEXT NOT NULL,
	modified_at TEXT NOT NULL,
	deleted_at TEXT
) STRICT;

CREATE INDEX IF NOT EXISTS messages_room_created ON messages(room_id, created_at DESC);

CREATE TABLE IF NOT EXISTS reactions(
	message_id TEXT REFERENCES messages(id) NOT NULL,
	user_id TEXT REFERENCES users(id) NOT NULL,
	emoji TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (message_id, user_id, emoji)
) STRICT;

CREATE INDEX IF NOT EXISTS reactions_message ON reactions(message_id);
`

// CreateTestUser creates a user in the database for testing
func CreateTestUser(t *testing.T, database *db.DB, id, username string) *models.User {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	user := &models.User{
		ID:          id,
		Username:    username,
		Password:    "hashedpassword",
		DisplayName: "",
		Status:      "",
		LastRoom:    "",
		CreatedAt:   now,
		ModifiedAt:  now,
	}
	err := user.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user
}

// CreateTestRoom creates a channel room in the database for testing
func CreateTestRoom(t *testing.T, database *db.DB, id, name string, isDefault bool) *models.Room {
	t.Helper()
	return CreateTestRoomWithPrivate(t, database, id, name, isDefault, false)
}

// CreateTestRoomWithPrivate creates a channel room in the database for testing with explicit private flag
func CreateTestRoomWithPrivate(t *testing.T, database *db.DB, id, name string, isDefault, isPrivate bool) *models.Room {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	isDefaultInt := models.FALSE
	if isDefault {
		isDefaultInt = models.TRUE
	}
	isPrivateInt := models.FALSE
	if isPrivate {
		isPrivateInt = models.TRUE
	}
	room := &models.Room{
		ID:        id,
		Name:      name,
		RoomType:  "channel",
		IsPrivate: isPrivateInt,
		IsDefault: isDefaultInt,
		CreatedAt: now,
	}
	err := room.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}
	return room
}

// CreateTestDM creates a DM room in the database for testing
func CreateTestDM(t *testing.T, database *db.DB, id string) *models.Room {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	room := &models.Room{
		ID:        id,
		Name:      "",
		RoomType:  "dm",
		IsPrivate: models.TRUE,
		IsDefault: models.FALSE,
		CreatedAt: now,
	}
	err := room.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test DM: %v", err)
	}
	return room
}

// AddUserToRoom adds a user to a room
func AddUserToRoom(t *testing.T, database *db.DB, userID, roomID string) {
	t.Helper()
	membership := &models.RoomsMember{
		UserID: userID,
		RoomID: roomID,
	}
	err := membership.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to add user to room: %v", err)
	}
}

// CreateTestMessage creates a message in the database for testing
func CreateTestMessage(t *testing.T, database *db.DB, id, roomID, userID, body string) *models.Message {
	t.Helper()
	now := time.Now().Format(time.RFC3339Nano)
	msg := &models.Message{
		ID:         id,
		RoomID:     roomID,
		UserID:     userID,
		Body:       body,
		CreatedAt:  now,
		ModifiedAt: now,
	}
	err := msg.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}
	return msg
}
