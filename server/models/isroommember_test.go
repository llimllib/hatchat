package models

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/db"
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
func createTestUser(t *testing.T, database *db.DB, id, username string) *User {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	user := &User{
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
func createTestRoom(t *testing.T, database *db.DB, id, name string, isDefault bool) *Room {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	isDefaultInt := FALSE
	if isDefault {
		isDefaultInt = TRUE
	}
	room := &Room{
		ID:        id,
		Name:      name,
		IsPrivate: FALSE,
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
	membership := &RoomsMember{
		UserID: userID,
		RoomID: roomID,
	}
	err := membership.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to add user to room: %v", err)
	}
}

// TestIsRoomMember_UserIsMember tests that IsRoomMember returns true for members
func TestIsRoomMember_UserIsMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	isMember, err := IsRoomMember(context.Background(), database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if !isMember {
		t.Error("Expected user to be a member of the room")
	}
}

// TestIsRoomMember_UserIsNotMember tests that IsRoomMember returns false for non-members
func TestIsRoomMember_UserIsNotMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	// Note: NOT adding user to room

	isMember, err := IsRoomMember(context.Background(), database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if isMember {
		t.Error("Expected user NOT to be a member of the room")
	}
}

// TestIsRoomMember_NonExistentUser tests behavior with non-existent user
func TestIsRoomMember_NonExistentUser(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	room := createTestRoom(t, database, "roo_test12345678", "general", true)

	isMember, err := IsRoomMember(context.Background(), database, "usr_nonexistent1", room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if isMember {
		t.Error("Expected non-existent user NOT to be a member")
	}
}

// TestIsRoomMember_NonExistentRoom tests behavior with non-existent room
func TestIsRoomMember_NonExistentRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	isMember, err := IsRoomMember(context.Background(), database, user.ID, "roo_nonexistent1")
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if isMember {
		t.Error("Expected user NOT to be a member of non-existent room")
	}
}

// TestIsRoomMember_MultipleRooms tests membership across multiple rooms
func TestIsRoomMember_MultipleRooms(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room1 := createTestRoom(t, database, "roo_room1234567", "room1", true)
	room2 := createTestRoom(t, database, "roo_room2345678", "room2", false)
	room3 := createTestRoom(t, database, "roo_room3456789", "room3", false)

	// Add user to room1 and room2, but NOT room3
	addUserToRoom(t, database, user.ID, room1.ID)
	addUserToRoom(t, database, user.ID, room2.ID)

	// Check room1 - should be member
	isMember, err := IsRoomMember(context.Background(), database, user.ID, room1.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed for room1: %v", err)
	}
	if !isMember {
		t.Error("Expected user to be a member of room1")
	}

	// Check room2 - should be member
	isMember, err = IsRoomMember(context.Background(), database, user.ID, room2.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed for room2: %v", err)
	}
	if !isMember {
		t.Error("Expected user to be a member of room2")
	}

	// Check room3 - should NOT be member
	isMember, err = IsRoomMember(context.Background(), database, user.ID, room3.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed for room3: %v", err)
	}
	if isMember {
		t.Error("Expected user NOT to be a member of room3")
	}
}

// TestIsRoomMember_MultipleUsers tests membership with multiple users
func TestIsRoomMember_MultipleUsers(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	user1 := createTestUser(t, database, "usr_user100000001", "alice")
	user2 := createTestUser(t, database, "usr_user200000002", "bob")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)

	// Add only user1 to room
	addUserToRoom(t, database, user1.ID, room.ID)

	// Check user1 - should be member
	isMember, err := IsRoomMember(context.Background(), database, user1.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed for user1: %v", err)
	}
	if !isMember {
		t.Error("Expected user1 to be a member")
	}

	// Check user2 - should NOT be member
	isMember, err = IsRoomMember(context.Background(), database, user2.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed for user2: %v", err)
	}
	if isMember {
		t.Error("Expected user2 NOT to be a member")
	}
}

// TestIsRoomMember_AfterRemoval tests that membership check is accurate after removal
func TestIsRoomMember_AfterRemoval(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Verify membership
	isMember, err := IsRoomMember(context.Background(), database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if !isMember {
		t.Error("Expected user to be a member initially")
	}

	// Remove membership
	_, err = database.ExecContext(context.Background(),
		"DELETE FROM rooms_members WHERE user_id = ? AND room_id = ?", user.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to remove membership: %v", err)
	}

	// Verify no longer a member
	isMember, err = IsRoomMember(context.Background(), database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed after removal: %v", err)
	}
	if isMember {
		t.Error("Expected user NOT to be a member after removal")
	}
}

// TestIsRoomMember_EmptyStrings tests behavior with empty string inputs
func TestIsRoomMember_EmptyStrings(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Empty user ID
	isMember, err := IsRoomMember(context.Background(), database, "", room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed with empty user ID: %v", err)
	}
	if isMember {
		t.Error("Expected empty user ID NOT to be a member")
	}

	// Empty room ID
	isMember, err = IsRoomMember(context.Background(), database, user.ID, "")
	if err != nil {
		t.Fatalf("IsRoomMember failed with empty room ID: %v", err)
	}
	if isMember {
		t.Error("Expected empty room ID NOT to return membership")
	}

	// Both empty
	isMember, err = IsRoomMember(context.Background(), database, "", "")
	if err != nil {
		t.Fatalf("IsRoomMember failed with both empty: %v", err)
	}
	if isMember {
		t.Error("Expected both empty NOT to return membership")
	}
}
