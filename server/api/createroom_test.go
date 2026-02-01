package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// TestCreateRoom_Success tests that a user can create a new room
func TestCreateRoom_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	reqData := protocol.CreateRoomRequest{
		Name:      "my-new-channel",
		IsPrivate: false,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.CreateRoom(user, reqJSON)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// Verify response
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if response.Envelope.Type != "create_room" {
		t.Errorf("Expected type 'create_room', got '%s'", response.Envelope.Type)
	}

	createResp, ok := response.Envelope.Data.(protocol.CreateRoomResponse)
	if !ok {
		t.Fatalf("Expected protocol.CreateRoomResponse data type, got %T", response.Envelope.Data)
	}

	if createResp.Room.Name != "my-new-channel" {
		t.Errorf("Expected room name 'my-new-channel', got %s", createResp.Room.Name)
	}
	if createResp.Room.IsPrivate {
		t.Error("Expected room to be public")
	}

	// Verify room was created in database
	room, err := models.RoomByID(context.Background(), database, createResp.Room.ID)
	if err != nil {
		t.Fatalf("Failed to fetch created room: %v", err)
	}
	if room.Name != "my-new-channel" {
		t.Errorf("Expected room name in DB to be 'my-new-channel', got %s", room.Name)
	}

	// Verify user is a member
	isMember, err := db.IsRoomMember(context.Background(), database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("Creator should be a member of the room")
	}

	// Verify user's last_room was updated
	updatedUser, err := models.UserByID(context.Background(), database, user.ID)
	if err != nil {
		t.Fatalf("Failed to fetch user: %v", err)
	}
	if updatedUser.LastRoom != room.ID {
		t.Errorf("Expected last_room to be %s, got %s", room.ID, updatedUser.LastRoom)
	}
}

// TestCreateRoom_PrivateRoom tests that a user can create a private room
func TestCreateRoom_PrivateRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	reqData := protocol.CreateRoomRequest{
		Name:      "secret-channel",
		IsPrivate: true,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.CreateRoom(user, reqJSON)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	createResp, ok := response.Envelope.Data.(protocol.CreateRoomResponse)
	if !ok {
		t.Fatalf("Expected protocol.CreateRoomResponse data type, got %T", response.Envelope.Data)
	}

	if !createResp.Room.IsPrivate {
		t.Error("Expected room to be private")
	}

	// Verify in database
	room, err := models.RoomByID(context.Background(), database, createResp.Room.ID)
	if err != nil {
		t.Fatalf("Failed to fetch created room: %v", err)
	}
	if room.IsPrivate != models.TRUE {
		t.Error("Expected room to be private in DB")
	}
}

// TestCreateRoom_EmptyName tests that an empty room name is rejected
func TestCreateRoom_EmptyName(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	reqData := protocol.CreateRoomRequest{
		Name: "",
	}
	reqJSON, _ := json.Marshal(reqData)

	_, err := api.CreateRoom(user, reqJSON)
	if err == nil {
		t.Fatal("Expected error for empty room name")
	}
}

// TestCreateRoom_WhitespaceName tests that a whitespace-only room name is rejected
func TestCreateRoom_WhitespaceName(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	reqData := protocol.CreateRoomRequest{
		Name: "   ",
	}
	reqJSON, _ := json.Marshal(reqData)

	_, err := api.CreateRoom(user, reqJSON)
	if err == nil {
		t.Fatal("Expected error for whitespace-only room name")
	}
}

// TestCreateRoom_LongName tests that a too-long room name is rejected
func TestCreateRoom_LongName(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// 81 characters
	longName := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabc"
	reqData := protocol.CreateRoomRequest{
		Name: longName,
	}
	reqJSON, _ := json.Marshal(reqData)

	_, err := api.CreateRoom(user, reqJSON)
	if err == nil {
		t.Fatal("Expected error for too-long room name")
	}
}

// TestCreateRoom_InvalidJSON tests that invalid JSON is rejected
func TestCreateRoom_InvalidJSON(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	_, err := api.CreateRoom(user, []byte("not valid json"))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

// TestCreateRoom_DuplicateName tests that duplicate room names are rejected
func TestCreateRoom_DuplicateName(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create first room
	reqData := protocol.CreateRoomRequest{
		Name:      "unique-channel",
		IsPrivate: false,
	}
	reqJSON, _ := json.Marshal(reqData)

	_, err := api.CreateRoom(user, reqJSON)
	if err != nil {
		t.Fatalf("First CreateRoom failed: %v", err)
	}

	// Try to create second room with same name
	_, err = api.CreateRoom(user, reqJSON)
	if err == nil {
		t.Fatal("Expected error for duplicate room name")
	}
	if err != ErrRoomNameTaken {
		t.Errorf("Expected ErrRoomNameTaken, got: %v", err)
	}
}

// TestCreateRoom_DuplicateNameCaseInsensitive tests that room names are case-sensitive
// (i.e., "General" and "general" are different rooms - for now)
func TestCreateRoom_DifferentCase(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create first room
	reqData1 := protocol.CreateRoomRequest{
		Name:      "General",
		IsPrivate: false,
	}
	reqJSON1, _ := json.Marshal(reqData1)

	_, err := api.CreateRoom(user, reqJSON1)
	if err != nil {
		t.Fatalf("First CreateRoom failed: %v", err)
	}

	// Create second room with different case - this should succeed
	// Note: If we want case-insensitive uniqueness, we'd need to change the schema
	reqData2 := protocol.CreateRoomRequest{
		Name:      "general",
		IsPrivate: false,
	}
	reqJSON2, _ := json.Marshal(reqData2)

	_, err = api.CreateRoom(user, reqJSON2)
	if err != nil {
		t.Fatalf("Second CreateRoom with different case failed: %v", err)
	}
}

// TestCreateRoom_NameIsTrimmed tests that room names are trimmed
func TestCreateRoom_NameIsTrimmed(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	reqData := protocol.CreateRoomRequest{
		Name: "  trimmed-name  ",
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.CreateRoom(user, reqJSON)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	createResp, ok := response.Envelope.Data.(protocol.CreateRoomResponse)
	if !ok {
		t.Fatalf("Expected protocol.CreateRoomResponse data type, got %T", response.Envelope.Data)
	}

	if createResp.Room.Name != "trimmed-name" {
		t.Errorf("Expected room name to be trimmed to 'trimmed-name', got '%s'", createResp.Room.Name)
	}
}
