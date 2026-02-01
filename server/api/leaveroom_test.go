package api

import (
	"log/slog"
	"os"
	"testing"

	"github.com/llimllib/hatchat/server/protocol"
)

func TestLeaveRoom_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "test-channel", false)
	addUserToRoom(t, database, user.ID, room.ID)

	response, err := api.LeaveRoom(user, []byte(`{"room_id": "roo_test12345678"}`))
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	if response.Type != "leave_room" {
		t.Errorf("Expected type 'leave_room', got '%s'", response.Type)
	}

	data, ok := response.Data.(protocol.LeaveRoomResponse)
	if !ok {
		t.Fatalf("Expected LeaveRoomResponse, got %T", response.Data)
	}

	if data.RoomID != room.ID {
		t.Errorf("Expected room_id '%s', got '%s'", room.ID, data.RoomID)
	}
}

func TestLeaveRoom_NotMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	_ = createTestRoom(t, database, "roo_test12345678", "test-channel", false)
	// Don't add user to room

	response, err := api.LeaveRoom(user, []byte(`{"room_id": "roo_test12345678"}`))
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	if response.Type != "error" {
		t.Errorf("Expected error response, got '%s'", response.Type)
	}
}

func TestLeaveRoom_DefaultRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_default12345", "general", true) // true = isDefault
	addUserToRoom(t, database, user.ID, room.ID)

	response, err := api.LeaveRoom(user, []byte(`{"room_id": "roo_default12345"}`))
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	if response.Type != "error" {
		t.Errorf("Expected error response for default room, got '%s'", response.Type)
	}

	data, ok := response.Data.(*protocol.ErrorResponse)
	if !ok {
		t.Fatalf("Expected ErrorResponse, got %T", response.Data)
	}

	if data.Message != "cannot leave the default room" {
		t.Errorf("Unexpected error message: %s", data.Message)
	}
}

func TestLeaveRoom_RoomNotFound(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	response, err := api.LeaveRoom(user, []byte(`{"room_id": "roo_nonexistent1"}`))
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	if response.Type != "error" {
		t.Errorf("Expected error response, got '%s'", response.Type)
	}
}

func TestLeaveRoom_EmptyRoomID(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	response, err := api.LeaveRoom(user, []byte(`{"room_id": ""}`))
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}

	if response.Type != "error" {
		t.Errorf("Expected error response, got '%s'", response.Type)
	}
}

func TestLeaveRoom_InvalidJSON(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	_, err := api.LeaveRoom(user, []byte("not valid json"))
	if err == nil {
		t.Fatal("LeaveRoom should fail on invalid JSON")
	}
}
