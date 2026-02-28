package db

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/models"
)

// testDBWithReadPositions creates a new in-memory database with read_positions table
func testDBWithReadPositions(t *testing.T) *DB {
	t.Helper()
	dbPath := "file::memory:?cache=shared"
	database, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create schema with read_positions table
	schema := `
		DROP TABLE IF EXISTS read_positions;
		DROP TABLE IF EXISTS reactions;
		DROP TABLE IF EXISTS messages;
		DROP TABLE IF EXISTS rooms_members;
		DROP TABLE IF EXISTS sessions;
		DROP TABLE IF EXISTS rooms;
		DROP TABLE IF EXISTS users;

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

		CREATE TABLE IF NOT EXISTS rooms(
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			room_type TEXT NOT NULL DEFAULT 'channel',
			is_private INTEGER NOT NULL,
			is_default INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			last_message_at TEXT
		) STRICT;

		CREATE TABLE IF NOT EXISTS rooms_members(
			user_id TEXT REFERENCES users(id) NOT NULL,
			room_id TEXT REFERENCES rooms(id) NOT NULL,
			PRIMARY KEY (user_id, room_id)
		) STRICT;

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

		CREATE TABLE IF NOT EXISTS read_positions(
			user_id TEXT REFERENCES users(id) NOT NULL,
			room_id TEXT REFERENCES rooms(id) NOT NULL,
			last_read_at TEXT NOT NULL,
			PRIMARY KEY (user_id, room_id)
		) STRICT;
	`
	_, err = database.ExecContext(context.Background(), schema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return database
}

// createUnreadTestUser creates a user in the database for testing
func createUnreadTestUser(t *testing.T, database *DB, id, username string) *models.User {
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

// createUnreadTestRoom creates a room in the database for testing
func createUnreadTestRoom(t *testing.T, database *DB, id, name string) *models.Room {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	room := &models.Room{
		ID:        id,
		Name:      name,
		RoomType:  "channel",
		IsPrivate: models.FALSE,
		IsDefault: models.FALSE,
		CreatedAt: now,
	}
	err := room.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}
	return room
}

// addUserToRoom adds a user to a room
func addUnreadTestUserToRoom(t *testing.T, database *DB, userID, roomID string) {
	t.Helper()
	rm := &models.RoomsMember{
		UserID: userID,
		RoomID: roomID,
	}
	err := rm.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to add user to room: %v", err)
	}
}

// createUnreadTestMessage creates a message in the database
func createUnreadTestMessage(t *testing.T, database *DB, id, roomID, userID, body, createdAt string) *models.Message {
	t.Helper()
	msg := &models.Message{
		ID:         id,
		RoomID:     roomID,
		UserID:     userID,
		Body:       body,
		CreatedAt:  createdAt,
		ModifiedAt: createdAt,
	}
	err := msg.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}
	return msg
}

func TestSetReadPosition(t *testing.T) {
	db := testDBWithReadPositions(t)
	ctx := context.Background()

	user := createUnreadTestUser(t, db, "usr_1", "alice")
	room := createUnreadTestRoom(t, db, "roo_1", "general")
	addUnreadTestUserToRoom(t, db, user.ID, room.ID)

	now := time.Now().Format(time.RFC3339Nano)

	// Set initial read position
	err := db.SetReadPosition(ctx, user.ID, room.ID, now)
	if err != nil {
		t.Fatalf("Failed to set read position: %v", err)
	}

	// Verify it was set
	rp, err := db.GetReadPosition(ctx, user.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to get read position: %v", err)
	}
	if rp == nil {
		t.Fatal("Expected read position to exist")
	}
	if rp.LastReadAt != now {
		t.Errorf("Expected last_read_at=%s, got %s", now, rp.LastReadAt)
	}

	// Update read position (upsert)
	later := time.Now().Add(time.Hour).Format(time.RFC3339Nano)
	err = db.SetReadPosition(ctx, user.ID, room.ID, later)
	if err != nil {
		t.Fatalf("Failed to update read position: %v", err)
	}

	// Verify update
	rp, err = db.GetReadPosition(ctx, user.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to get read position: %v", err)
	}
	if rp.LastReadAt != later {
		t.Errorf("Expected updated last_read_at=%s, got %s", later, rp.LastReadAt)
	}
}

func TestGetUnreadCounts_NoReadPosition(t *testing.T) {
	db := testDBWithReadPositions(t)
	ctx := context.Background()

	user := createUnreadTestUser(t, db, "usr_1", "alice")
	room := createUnreadTestRoom(t, db, "roo_1", "general")
	addUnreadTestUserToRoom(t, db, user.ID, room.ID)

	// Add some messages
	baseTime := time.Now()
	createUnreadTestMessage(t, db, "msg_1", room.ID, user.ID, "Hello", baseTime.Format(time.RFC3339Nano))
	createUnreadTestMessage(t, db, "msg_2", room.ID, user.ID, "World", baseTime.Add(time.Second).Format(time.RFC3339Nano))
	createUnreadTestMessage(t, db, "msg_3", room.ID, user.ID, "Test", baseTime.Add(2*time.Second).Format(time.RFC3339Nano))

	// Get unread counts - should be 3 since no read position exists
	counts, err := db.GetUnreadCounts(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get unread counts: %v", err)
	}

	if counts[room.ID] != 3 {
		t.Errorf("Expected 3 unread messages, got %d", counts[room.ID])
	}
}

func TestGetUnreadCounts_WithReadPosition(t *testing.T) {
	db := testDBWithReadPositions(t)
	ctx := context.Background()

	user := createUnreadTestUser(t, db, "usr_1", "alice")
	room := createUnreadTestRoom(t, db, "roo_1", "general")
	addUnreadTestUserToRoom(t, db, user.ID, room.ID)

	// Add some messages
	baseTime := time.Now()
	time1 := baseTime.Format(time.RFC3339Nano)
	time2 := baseTime.Add(time.Second).Format(time.RFC3339Nano)
	time3 := baseTime.Add(2 * time.Second).Format(time.RFC3339Nano)

	createUnreadTestMessage(t, db, "msg_1", room.ID, user.ID, "Hello", time1)
	createUnreadTestMessage(t, db, "msg_2", room.ID, user.ID, "World", time2)
	createUnreadTestMessage(t, db, "msg_3", room.ID, user.ID, "Test", time3)

	// Set read position after first message
	err := db.SetReadPosition(ctx, user.ID, room.ID, time1)
	if err != nil {
		t.Fatalf("Failed to set read position: %v", err)
	}

	// Get unread counts - should be 2 (messages 2 and 3)
	counts, err := db.GetUnreadCounts(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get unread counts: %v", err)
	}

	if counts[room.ID] != 2 {
		t.Errorf("Expected 2 unread messages, got %d", counts[room.ID])
	}
}

func TestGetUnreadCounts_AllRead(t *testing.T) {
	db := testDBWithReadPositions(t)
	ctx := context.Background()

	user := createUnreadTestUser(t, db, "usr_1", "alice")
	room := createUnreadTestRoom(t, db, "roo_1", "general")
	addUnreadTestUserToRoom(t, db, user.ID, room.ID)

	// Add some messages
	baseTime := time.Now()
	time1 := baseTime.Format(time.RFC3339Nano)
	time2 := baseTime.Add(time.Second).Format(time.RFC3339Nano)
	time3 := baseTime.Add(2 * time.Second).Format(time.RFC3339Nano)

	createUnreadTestMessage(t, db, "msg_1", room.ID, user.ID, "Hello", time1)
	createUnreadTestMessage(t, db, "msg_2", room.ID, user.ID, "World", time2)
	createUnreadTestMessage(t, db, "msg_3", room.ID, user.ID, "Test", time3)

	// Set read position to latest message
	err := db.SetReadPosition(ctx, user.ID, room.ID, time3)
	if err != nil {
		t.Fatalf("Failed to set read position: %v", err)
	}

	// Get unread counts - should be 0
	counts, err := db.GetUnreadCounts(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get unread counts: %v", err)
	}

	if counts[room.ID] != 0 {
		t.Errorf("Expected 0 unread messages, got %d", counts[room.ID])
	}
}

func TestGetUnreadCounts_MultipleRooms(t *testing.T) {
	db := testDBWithReadPositions(t)
	ctx := context.Background()

	user := createUnreadTestUser(t, db, "usr_1", "alice")
	room1 := createUnreadTestRoom(t, db, "roo_1", "general")
	room2 := createUnreadTestRoom(t, db, "roo_2", "random")
	addUnreadTestUserToRoom(t, db, user.ID, room1.ID)
	addUnreadTestUserToRoom(t, db, user.ID, room2.ID)

	// Add messages to both rooms
	baseTime := time.Now()
	time1 := baseTime.Format(time.RFC3339Nano)
	time2 := baseTime.Add(time.Second).Format(time.RFC3339Nano)

	createUnreadTestMessage(t, db, "msg_1", room1.ID, user.ID, "Hello", time1)
	createUnreadTestMessage(t, db, "msg_2", room1.ID, user.ID, "World", time2)
	createUnreadTestMessage(t, db, "msg_3", room2.ID, user.ID, "Hi", time1)

	// Mark room1 as read
	err := db.SetReadPosition(ctx, user.ID, room1.ID, time2)
	if err != nil {
		t.Fatalf("Failed to set read position: %v", err)
	}

	// Get unread counts
	counts, err := db.GetUnreadCounts(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get unread counts: %v", err)
	}

	if counts[room1.ID] != 0 {
		t.Errorf("Expected 0 unread in room1, got %d", counts[room1.ID])
	}
	if counts[room2.ID] != 1 {
		t.Errorf("Expected 1 unread in room2, got %d", counts[room2.ID])
	}
}

func TestGetUnreadCount_SingleRoom(t *testing.T) {
	db := testDBWithReadPositions(t)
	ctx := context.Background()

	user := createUnreadTestUser(t, db, "usr_1", "alice")
	room := createUnreadTestRoom(t, db, "roo_1", "general")
	addUnreadTestUserToRoom(t, db, user.ID, room.ID)

	// Add some messages
	baseTime := time.Now()
	time1 := baseTime.Format(time.RFC3339Nano)
	time2 := baseTime.Add(time.Second).Format(time.RFC3339Nano)

	createUnreadTestMessage(t, db, "msg_1", room.ID, user.ID, "Hello", time1)
	createUnreadTestMessage(t, db, "msg_2", room.ID, user.ID, "World", time2)

	// Get unread count for specific room
	count, err := db.GetUnreadCount(ctx, user.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to get unread count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 unread messages, got %d", count)
	}

	// Mark as read
	err = db.SetReadPosition(ctx, user.ID, room.ID, time2)
	if err != nil {
		t.Fatalf("Failed to set read position: %v", err)
	}

	// Get unread count again
	count, err = db.GetUnreadCount(ctx, user.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to get unread count: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 unread messages after marking read, got %d", count)
	}
}

func TestGetUnreadCounts_DeletedMessages(t *testing.T) {
	db := testDBWithReadPositions(t)
	ctx := context.Background()

	user := createUnreadTestUser(t, db, "usr_1", "alice")
	room := createUnreadTestRoom(t, db, "roo_1", "general")
	addUnreadTestUserToRoom(t, db, user.ID, room.ID)

	// Add messages, including a deleted one
	baseTime := time.Now()
	time1 := baseTime.Format(time.RFC3339Nano)
	time2 := baseTime.Add(time.Second).Format(time.RFC3339Nano)
	time3 := baseTime.Add(2 * time.Second).Format(time.RFC3339Nano)

	createUnreadTestMessage(t, db, "msg_1", room.ID, user.ID, "Hello", time1)
	createUnreadTestMessage(t, db, "msg_2", room.ID, user.ID, "World", time2)
	msg3 := createUnreadTestMessage(t, db, "msg_3", room.ID, user.ID, "Deleted", time3)

	// Soft-delete message 3
	msg3.DeletedAt.String = time3
	msg3.DeletedAt.Valid = true
	err := msg3.Update(ctx, db)
	if err != nil {
		t.Fatalf("Failed to delete message: %v", err)
	}

	// Get unread counts - should be 2 (deleted message shouldn't count)
	counts, err := db.GetUnreadCounts(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get unread counts: %v", err)
	}

	if counts[room.ID] != 2 {
		t.Errorf("Expected 2 unread messages (excluding deleted), got %d", counts[room.ID])
	}
}
