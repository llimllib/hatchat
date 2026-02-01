package api

import (
	"github.com/llimllib/hatchat/server/protocol"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

// testDB creates a new in-memory database with the schema loaded
func testDB(t *testing.T) *db.DB {
	t.Helper()
	dbPath := "file::memory:?cache=shared"
	database, err := db.NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create schema
	schema := `
		CREATE TABLE IF NOT EXISTS users(
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			active INTEGER,
			avatar TEXT,
			last_room TEXT NOT NULL,
			created_at TEXT NOT NULL,
			modified_at TEXT NOT NULL
		) STRICT;

		CREATE UNIQUE INDEX IF NOT EXISTS users_username ON users(username);

		CREATE TABLE IF NOT EXISTS rooms(
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			is_private INTEGER NOT NULL,
			is_default INTEGER NOT NULL,
			created_at TEXT NOT NULL
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
			modified_at TEXT NOT NULL
		) STRICT;
	`
	_, err = database.ExecContext(context.Background(), schema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return database
}

// createTestUser creates a user in the database for testing
func createTestUser(t *testing.T, database *db.DB, id, username string) *models.User {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	user := &models.User{
		ID:         id,
		Username:   username,
		Password:   "hashedpassword",
		LastRoom:   "",
		CreatedAt:  now,
		ModifiedAt: now,
	}
	err := user.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user
}

// createTestRoom creates a room in the database for testing
func createTestRoom(t *testing.T, database *db.DB, id, name string, isDefault bool) *models.Room {
	t.Helper()
	return createTestRoomWithPrivate(t, database, id, name, isDefault, false)
}

// createTestRoomWithPrivate creates a room in the database for testing with explicit private flag
func createTestRoomWithPrivate(t *testing.T, database *db.DB, id, name string, isDefault, isPrivate bool) *models.Room {
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

// addUserToRoom adds a user to a room
func addUserToRoom(t *testing.T, database *db.DB, userID, roomID string) {
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

// TestMessageMessage_ValidMember tests that a room member can send a message
func TestMessageMessage_ValidMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create user and room
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Send a message
	msgData := protocol.SendMessageRequest{
		Body:   "Hello, world!",
		RoomID: room.ID,
	}
	msgJSON, _ := json.Marshal(msgData)

	response, err := api.MessageMessage(user, msgJSON)
	if err != nil {
		t.Fatalf("MessageMessage failed: %v", err)
	}

	// Verify response
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if response.RoomID != room.ID {
		t.Errorf("Expected room ID %s, got %s", room.ID, response.RoomID)
	}

	// Verify message was stored in database
	var count int
	err = database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM messages WHERE room_id = ?", room.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query messages: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 message in database, got %d", count)
	}
}

// TestMessageMessage_NonMemberRejected tests that a non-member cannot send messages
// SECURITY: This is critical - users must not be able to send messages to rooms they don't belong to
func TestMessageMessage_NonMemberRejected(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create user and room, but DON'T add user to room
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "secret-room", false)

	// Try to send a message to a room we're not a member of
	msgData := protocol.SendMessageRequest{
		Body:   "I shouldn't be able to send this!",
		RoomID: room.ID,
	}
	msgJSON, _ := json.Marshal(msgData)

	response, err := api.MessageMessage(user, msgJSON)

	// Verify the request was rejected
	if err == nil {
		t.Error("Expected error when non-member sends message, got nil")
	}
	if response != nil {
		t.Error("Expected nil response when non-member sends message")
	}

	// Verify NO message was stored in database
	var count int
	err = database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM messages WHERE room_id = ?", room.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query messages: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 messages in database (security breach!), got %d", count)
	}
}

// TestMessageMessage_NonExistentRoom tests that messages to non-existent rooms are rejected
func TestMessageMessage_NonExistentRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Try to send a message to a non-existent room
	msgData := protocol.SendMessageRequest{
		Body:   "Message to nowhere",
		RoomID: "roo_nonexistent1",
	}
	msgJSON, _ := json.Marshal(msgData)

	response, err := api.MessageMessage(user, msgJSON)

	if err == nil {
		t.Error("Expected error for non-existent room, got nil")
	}
	if response != nil {
		t.Error("Expected nil response for non-existent room")
	}
}

// TestMessageMessage_EmptyBody tests that empty messages are rejected
func TestMessageMessage_EmptyBody(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Try to send an empty message
	msgData := protocol.SendMessageRequest{
		Body:   "",
		RoomID: room.ID,
	}
	msgJSON, _ := json.Marshal(msgData)

	response, err := api.MessageMessage(user, msgJSON)

	if err == nil {
		t.Error("Expected error for empty message body, got nil")
	}
	if response != nil {
		t.Error("Expected nil response for empty message body")
	}
}

// TestMessageMessage_EmptyRoomID tests that messages without room IDs are rejected
func TestMessageMessage_EmptyRoomID(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Try to send a message without a room ID
	msgData := protocol.SendMessageRequest{
		Body:   "Hello",
		RoomID: "",
	}
	msgJSON, _ := json.Marshal(msgData)

	response, err := api.MessageMessage(user, msgJSON)

	if err == nil {
		t.Error("Expected error for empty room ID, got nil")
	}
	if response != nil {
		t.Error("Expected nil response for empty room ID")
	}
}

// TestMessageMessage_InvalidJSON tests that invalid JSON is rejected
func TestMessageMessage_InvalidJSON(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Try to send invalid JSON
	invalidJSON := json.RawMessage(`{invalid json}`)

	response, err := api.MessageMessage(user, invalidJSON)

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
	if response != nil {
		t.Error("Expected nil response for invalid JSON")
	}
}

// TestMessageMessage_MultipleRoomsSecurity tests that users can only send to their own rooms
// SECURITY: Critical test - verifies room isolation
func TestMessageMessage_MultipleRoomsSecurity(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create two users
	user1 := createTestUser(t, database, "usr_user100000001", "alice")
	user2 := createTestUser(t, database, "usr_user200000002", "bob")

	// Create two rooms
	room1 := createTestRoom(t, database, "roo_room1234567", "alice-room", false)
	room2 := createTestRoom(t, database, "roo_room2345678", "bob-room", false)

	// Add user1 to room1 only, user2 to room2 only
	addUserToRoom(t, database, user1.ID, room1.ID)
	addUserToRoom(t, database, user2.ID, room2.ID)

	// User1 tries to send to room2 (should fail)
	msgData := protocol.SendMessageRequest{
		Body:   "Trying to infiltrate Bob's room!",
		RoomID: room2.ID,
	}
	msgJSON, _ := json.Marshal(msgData)

	response, err := api.MessageMessage(user1, msgJSON)
	if err == nil {
		t.Error("SECURITY BREACH: User1 was able to send message to room2 they don't belong to")
	}
	if response != nil {
		t.Error("Expected nil response when sending to unauthorized room")
	}

	// User2 tries to send to room1 (should fail)
	msgData = protocol.SendMessageRequest{
		Body:   "Trying to infiltrate Alice's room!",
		RoomID: room1.ID,
	}
	msgJSON, _ = json.Marshal(msgData)

	response, err = api.MessageMessage(user2, msgJSON)
	if err == nil {
		t.Error("SECURITY BREACH: User2 was able to send message to room1 they don't belong to")
	}
	if response != nil {
		t.Error("Expected nil response when sending to unauthorized room")
	}

	// Verify no unauthorized messages exist
	var count int
	err = database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM messages").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query messages: %v", err)
	}
	if count != 0 {
		t.Errorf("SECURITY BREACH: Expected 0 messages, but found %d unauthorized messages", count)
	}
}

// TestMessageMessage_MembershipRevokedDuringSession simulates a scenario where
// a user's membership is revoked after they've joined but before they send a message
// SECURITY: This tests that membership is checked at message send time, not just at connect time
func TestMessageMessage_MembershipRevokedDuringSession(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)

	// Add user to room
	addUserToRoom(t, database, user.ID, room.ID)

	// First message should succeed
	msgData := protocol.SendMessageRequest{
		Body:   "First message",
		RoomID: room.ID,
	}
	msgJSON, _ := json.Marshal(msgData)

	response, err := api.MessageMessage(user, msgJSON)
	if err != nil {
		t.Fatalf("First message should succeed: %v", err)
	}
	if response == nil {
		t.Fatal("Expected non-nil response for first message")
	}

	// Now remove user from room (simulating admin action during session)
	_, err = database.ExecContext(context.Background(),
		"DELETE FROM rooms_members WHERE user_id = ? AND room_id = ?", user.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to remove user from room: %v", err)
	}

	// Second message should fail
	msgData = protocol.SendMessageRequest{
		Body:   "Second message after revocation",
		RoomID: room.ID,
	}
	msgJSON, _ = json.Marshal(msgData)

	response, err = api.MessageMessage(user, msgJSON)
	if err == nil {
		t.Error("SECURITY BREACH: User was able to send message after membership was revoked")
	}
	if response != nil {
		t.Error("Expected nil response after membership revocation")
	}
}

// TestMessageMessage_ResponseContainsCorrectRoomID verifies that the response
// contains the correct room ID for routing
func TestMessageMessage_ResponseContainsCorrectRoomID(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create multiple rooms and add user to all
	room1 := createTestRoom(t, database, "roo_room1234567", "room1", false)
	room2 := createTestRoom(t, database, "roo_room2345678", "room2", false)
	addUserToRoom(t, database, user.ID, room1.ID)
	addUserToRoom(t, database, user.ID, room2.ID)

	// Send to room1
	msgData := protocol.SendMessageRequest{Body: "Hello room1", RoomID: room1.ID}
	msgJSON, _ := json.Marshal(msgData)
	response, err := api.MessageMessage(user, msgJSON)
	if err != nil {
		t.Fatalf("MessageMessage failed: %v", err)
	}
	if response.RoomID != room1.ID {
		t.Errorf("Expected room ID %s, got %s", room1.ID, response.RoomID)
	}

	// Send to room2
	msgData = protocol.SendMessageRequest{Body: "Hello room2", RoomID: room2.ID}
	msgJSON, _ = json.Marshal(msgData)
	response, err = api.MessageMessage(user, msgJSON)
	if err != nil {
		t.Fatalf("MessageMessage failed: %v", err)
	}
	if response.RoomID != room2.ID {
		t.Errorf("Expected room ID %s, got %s", room2.ID, response.RoomID)
	}
}

// TestMessageMessage_ResponseEnvelopeFormat verifies the response envelope structure
func TestMessageMessage_ResponseEnvelopeFormat(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgData := protocol.SendMessageRequest{Body: "Test message", RoomID: room.ID}
	msgJSON, _ := json.Marshal(msgData)
	response, err := api.MessageMessage(user, msgJSON)
	if err != nil {
		t.Fatalf("MessageMessage failed: %v", err)
	}

	// Parse the response message to verify envelope format
	var envelope Envelope
	err = json.Unmarshal(response.Message, &envelope)
	if err != nil {
		t.Fatalf("Failed to unmarshal response envelope: %v", err)
	}

	if envelope.Type != "message" {
		t.Errorf("Expected envelope type 'message', got '%s'", envelope.Type)
	}
}
