package api

import (
	"log/slog"
	"os"
	"testing"

	"github.com/llimllib/hatchat/server/protocol"
)

func TestRoomInfo_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	user2 := createTestUser(t, database, "usr_test987654321", "otheruser")
	room := createTestRoom(t, database, "roo_test12345678", "test-channel", false)
	addUserToRoom(t, database, user.ID, room.ID)
	addUserToRoom(t, database, user2.ID, room.ID)

	response, err := api.RoomInfo(user, []byte(`{"room_id": "roo_test12345678"}`))
	if err != nil {
		t.Fatalf("RoomInfo failed: %v", err)
	}

	if response.Type != "room_info" {
		t.Errorf("Expected type 'room_info', got '%s'", response.Type)
	}

	data, ok := response.Data.(protocol.RoomInfoResponse)
	if !ok {
		t.Fatalf("Expected RoomInfoResponse, got %T", response.Data)
	}

	if data.Room.ID != room.ID {
		t.Errorf("Expected room ID '%s', got '%s'", room.ID, data.Room.ID)
	}

	if data.Room.Name != "test-channel" {
		t.Errorf("Expected room name 'test-channel', got '%s'", data.Room.Name)
	}

	if data.MemberCount != 2 {
		t.Errorf("Expected member count 2, got %d", data.MemberCount)
	}

	if len(data.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(data.Members))
	}
}

func TestRoomInfo_NotMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	_ = createTestRoom(t, database, "roo_test12345678", "test-channel", false)
	// Don't add user to room

	response, err := api.RoomInfo(user, []byte(`{"room_id": "roo_test12345678"}`))
	if err != nil {
		t.Fatalf("RoomInfo failed: %v", err)
	}

	if response.Type != "error" {
		t.Errorf("Expected error response, got '%s'", response.Type)
	}
}

func TestRoomInfo_RoomNotFound(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	response, err := api.RoomInfo(user, []byte(`{"room_id": "roo_nonexistent1"}`))
	if err != nil {
		t.Fatalf("RoomInfo failed: %v", err)
	}

	if response.Type != "error" {
		t.Errorf("Expected error response, got '%s'", response.Type)
	}
}

func TestRoomInfo_EmptyRoomID(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	response, err := api.RoomInfo(user, []byte(`{"room_id": ""}`))
	if err != nil {
		t.Fatalf("RoomInfo failed: %v", err)
	}

	if response.Type != "error" {
		t.Errorf("Expected error response, got '%s'", response.Type)
	}
}

func TestRoomInfo_InvalidJSON(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	_, err := api.RoomInfo(user, []byte("not valid json"))
	if err == nil {
		t.Fatal("RoomInfo should fail on invalid JSON")
	}
}

func TestRoomInfo_PrivateRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoomWithPrivate(t, database, "roo_private12345", "private-channel", false, true)
	addUserToRoom(t, database, user.ID, room.ID)

	response, err := api.RoomInfo(user, []byte(`{"room_id": "roo_private12345"}`))
	if err != nil {
		t.Fatalf("RoomInfo failed: %v", err)
	}

	if response.Type != "room_info" {
		t.Errorf("Expected type 'room_info', got '%s'", response.Type)
	}

	data, ok := response.Data.(protocol.RoomInfoResponse)
	if !ok {
		t.Fatalf("Expected RoomInfoResponse, got %T", response.Data)
	}

	if !data.Room.IsPrivate {
		t.Error("Expected room to be private")
	}
}
