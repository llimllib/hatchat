package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// TestJoinRoom_ValidMember tests that a room member can switch to a room
func TestJoinRoom_ValidMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create user and two rooms
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room1 := createTestRoom(t, database, "roo_test12345678", "general", true)
	room2 := createTestRoom(t, database, "roo_test87654321", "random", false)
	addUserToRoom(t, database, user.ID, room1.ID)
	addUserToRoom(t, database, user.ID, room2.ID)

	// Set initial last_room
	user.LastRoom = room1.ID
	if err := user.Update(context.Background(), database); err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Join room2
	reqData := protocol.JoinRoomRequest{
		RoomID: room2.ID,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.JoinRoom(user, reqJSON)
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	// Verify response
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if response.RoomID != room2.ID {
		t.Errorf("Expected room ID %s, got %s", room2.ID, response.RoomID)
	}
	if response.Envelope.Type != "join_room" {
		t.Errorf("Expected type 'join_room', got '%s'", response.Envelope.Type)
	}

	joinResp, ok := response.Envelope.Data.(protocol.JoinRoomResponse)
	if !ok {
		t.Fatalf("Expected protocol.JoinRoomResponse data type, got %T", response.Envelope.Data)
	}

	if joinResp.Room.ID != room2.ID {
		t.Errorf("Expected room ID %s, got %s", room2.ID, joinResp.Room.ID)
	}
	if joinResp.Room.Name != "random" {
		t.Errorf("Expected room name 'random', got %s", joinResp.Room.Name)
	}

	// Verify last_room was updated in database
	updatedUser, err := models.UserByID(context.Background(), database, user.ID)
	if err != nil {
		t.Fatalf("Failed to fetch updated user: %v", err)
	}
	if updatedUser.LastRoom != room2.ID {
		t.Errorf("Expected last_room to be %s, got %s", room2.ID, updatedUser.LastRoom)
	}
}

// TestJoinRoom_NonMemberJoinsPublicRoom tests that a non-member can join a public room
func TestJoinRoom_NonMemberJoinsPublicRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create user and two rooms, but only add user to room1
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room1 := createTestRoom(t, database, "roo_test12345678", "general", true)
	room2 := createTestRoom(t, database, "roo_test87654321", "public-room", false) // public room
	addUserToRoom(t, database, user.ID, room1.ID)
	// NOT adding user to room2

	// Attempt to join room2 (should succeed and add as member)
	reqData := protocol.JoinRoomRequest{
		RoomID: room2.ID,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.JoinRoom(user, reqJSON)
	if err != nil {
		t.Fatalf("JoinRoom failed for public room: %v", err)
	}

	// Verify response shows user was joined
	joinResp, ok := response.Envelope.Data.(protocol.JoinRoomResponse)
	if !ok {
		t.Fatalf("Expected protocol.JoinRoomResponse data type, got %T", response.Envelope.Data)
	}
	if !joinResp.Joined {
		t.Error("Expected Joined to be true when joining new room")
	}
	if joinResp.Room.ID != room2.ID {
		t.Errorf("Expected room ID %s, got %s", room2.ID, joinResp.Room.ID)
	}

	// Verify last_room was updated
	updatedUser, err := models.UserByID(context.Background(), database, user.ID)
	if err != nil {
		t.Fatalf("Failed to fetch user: %v", err)
	}
	if updatedUser.LastRoom != room2.ID {
		t.Errorf("Expected last_room to be %s, got %s", room2.ID, updatedUser.LastRoom)
	}
}

// TestJoinRoom_PrivateRoomRejected tests that a non-member cannot join a private room
// SECURITY: Critical test - users must not be able to join private rooms without an invite
func TestJoinRoom_PrivateRoomRejected(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create user and two rooms, but only add user to room1
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room1 := createTestRoom(t, database, "roo_test12345678", "general", true)
	room2 := createTestRoomWithPrivate(t, database, "roo_test87654321", "private-room", false, true) // private!
	addUserToRoom(t, database, user.ID, room1.ID)
	// NOT adding user to room2

	// Attempt to join room2 (should fail because it's private)
	reqData := protocol.JoinRoomRequest{
		RoomID: room2.ID,
	}
	reqJSON, _ := json.Marshal(reqData)

	_, err := api.JoinRoom(user, reqJSON)
	if err == nil {
		t.Fatal("Expected error when non-member attempts to join private room")
	}

	// Verify last_room was NOT updated
	updatedUser, err := models.UserByID(context.Background(), database, user.ID)
	if err != nil {
		t.Fatalf("Failed to fetch user: %v", err)
	}
	if updatedUser.LastRoom == room2.ID {
		t.Error("last_room should not be updated when join fails")
	}
}

// TestJoinRoom_MemberJoinsReturnsJoinedFalse tests that an existing member switching rooms gets Joined=false
func TestJoinRoom_MemberJoinsReturnsJoinedFalse(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create user and room, add user to room
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Join the room (already a member)
	reqData := protocol.JoinRoomRequest{
		RoomID: room.ID,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.JoinRoom(user, reqJSON)
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	// Verify response shows user was NOT newly joined
	joinResp, ok := response.Envelope.Data.(protocol.JoinRoomResponse)
	if !ok {
		t.Fatalf("Expected protocol.JoinRoomResponse data type, got %T", response.Envelope.Data)
	}
	if joinResp.Joined {
		t.Error("Expected Joined to be false when already a member")
	}
}

// TestJoinRoom_EmptyRoomID tests that an empty room ID is rejected
func TestJoinRoom_EmptyRoomID(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	reqData := protocol.JoinRoomRequest{
		RoomID: "",
	}
	reqJSON, _ := json.Marshal(reqData)

	_, err := api.JoinRoom(user, reqJSON)
	if err == nil {
		t.Fatal("Expected error for empty room ID")
	}
}

// TestJoinRoom_NonexistentRoom tests that joining a nonexistent room fails
func TestJoinRoom_NonexistentRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	reqData := protocol.JoinRoomRequest{
		RoomID: "roo_nonexistent1",
	}
	reqJSON, _ := json.Marshal(reqData)

	_, err := api.JoinRoom(user, reqJSON)
	if err == nil {
		t.Fatal("Expected error for nonexistent room")
	}
}

// TestJoinRoom_InvalidJSON tests that invalid JSON is rejected
func TestJoinRoom_InvalidJSON(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	_, err := api.JoinRoom(user, []byte("not valid json"))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}
