package api

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/llimllib/hatchat/server/protocol"
)

// TestListRooms_Empty tests listing rooms when there are none
func TestListRooms_Empty(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	response, err := api.ListRooms(user, []byte("{}"))
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	if response.Type != "list_rooms" {
		t.Errorf("Expected type 'list_rooms', got '%s'", response.Type)
	}

	listResp, ok := response.Data.(protocol.ListRoomsResponse)
	if !ok {
		t.Fatalf("Expected protocol.ListRoomsResponse data type, got %T", response.Data)
	}

	if len(listResp.Rooms) != 0 {
		t.Errorf("Expected 0 rooms, got %d", len(listResp.Rooms))
	}
	if len(listResp.IsMember) != 0 {
		t.Errorf("Expected 0 membership flags, got %d", len(listResp.IsMember))
	}
}

// TestListRooms_OnlyPublicRooms tests that only public rooms are listed
func TestListRooms_OnlyPublicRooms(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create one public and one private room
	publicRoom := createTestRoomWithPrivate(t, database, "roo_public123456", "public-channel", false, false)
	_ = createTestRoomWithPrivate(t, database, "roo_private12345", "private-channel", false, true)

	response, err := api.ListRooms(user, []byte("{}"))
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	listResp, ok := response.Data.(protocol.ListRoomsResponse)
	if !ok {
		t.Fatalf("Expected protocol.ListRoomsResponse data type, got %T", response.Data)
	}

	// Only public room should be listed
	if len(listResp.Rooms) != 1 {
		t.Errorf("Expected 1 room, got %d", len(listResp.Rooms))
	}
	if len(listResp.Rooms) > 0 && listResp.Rooms[0].ID != publicRoom.ID {
		t.Errorf("Expected public room ID %s, got %s", publicRoom.ID, listResp.Rooms[0].ID)
	}
}

// TestListRooms_WithMembership tests that membership status is correctly reported
func TestListRooms_WithMembership(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create two public rooms, add user to one
	room1 := createTestRoom(t, database, "roo_aaaaaaaaaaaa", "alpha-channel", false)
	room2 := createTestRoom(t, database, "roo_bbbbbbbbbbbb", "beta-channel", false)
	addUserToRoom(t, database, user.ID, room1.ID) // member of room1 only

	response, err := api.ListRooms(user, []byte("{}"))
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	listResp, ok := response.Data.(protocol.ListRoomsResponse)
	if !ok {
		t.Fatalf("Expected protocol.ListRoomsResponse data type, got %T", response.Data)
	}

	if len(listResp.Rooms) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(listResp.Rooms))
	}
	if len(listResp.IsMember) != 2 {
		t.Errorf("Expected 2 membership flags, got %d", len(listResp.IsMember))
	}

	// Rooms are ordered by name (alpha, beta)
	// room1 (alpha) should be first and user is a member
	// room2 (beta) should be second and user is NOT a member
	for i, room := range listResp.Rooms {
		switch room.ID {
		case room1.ID:
			if !listResp.IsMember[i] {
				t.Error("Expected user to be a member of room1")
			}
		case room2.ID:
			if listResp.IsMember[i] {
				t.Error("Expected user to NOT be a member of room2")
			}
		}
	}
}

// TestListRooms_OrderedByName tests that rooms are ordered by name
func TestListRooms_OrderedByName(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create rooms in non-alphabetical order
	createTestRoom(t, database, "roo_zzzzzzzzzzzz", "zebra", false)
	createTestRoom(t, database, "roo_aaaaaaaaaaaa", "alpha", false)
	createTestRoom(t, database, "roo_mmmmmmmmmmmm", "middle", false)

	response, err := api.ListRooms(user, []byte("{}"))
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	listResp, ok := response.Data.(protocol.ListRoomsResponse)
	if !ok {
		t.Fatalf("Expected protocol.ListRoomsResponse data type, got %T", response.Data)
	}

	if len(listResp.Rooms) != 3 {
		t.Errorf("Expected 3 rooms, got %d", len(listResp.Rooms))
	}

	// Should be ordered: alpha, middle, zebra
	expectedOrder := []string{"alpha", "middle", "zebra"}
	for i, name := range expectedOrder {
		if listResp.Rooms[i].Name != name {
			t.Errorf("Expected room %d to be '%s', got '%s'", i, name, listResp.Rooms[i].Name)
		}
	}
}

// TestListRooms_InvalidJSON tests that the handler fails on invalid JSON
func TestListRooms_InvalidJSON(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Invalid JSON should return an error
	_, err := api.ListRooms(user, []byte("not valid json"))
	if err == nil {
		t.Fatal("ListRooms should fail on invalid JSON")
	}
}

// TestListRooms_WithQuery tests the search functionality
func TestListRooms_WithQuery(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create rooms with different names
	createTestRoom(t, database, "roo_general12345", "general", false)
	createTestRoom(t, database, "roo_random123456", "random", false)
	createTestRoom(t, database, "roo_generalann12", "general-announcements", false)

	// Search for "general"
	response, err := api.ListRooms(user, []byte(`{"query": "general"}`))
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	data, ok := response.Data.(protocol.ListRoomsResponse)
	if !ok {
		t.Fatalf("Expected ListRoomsResponse, got %T", response.Data)
	}

	if len(data.Rooms) != 2 {
		t.Errorf("Expected 2 rooms matching 'general', got %d", len(data.Rooms))
	}

	// Search for "random"
	response, err = api.ListRooms(user, []byte(`{"query": "random"}`))
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	data, ok = response.Data.(protocol.ListRoomsResponse)
	if !ok {
		t.Fatalf("Expected ListRoomsResponse, got %T", response.Data)
	}

	if len(data.Rooms) != 1 {
		t.Errorf("Expected 1 room matching 'random', got %d", len(data.Rooms))
	}
}

// TestListRooms_ViaCreateRoom tests the full flow: create a room and see it in list
func TestListRooms_ViaCreateRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create a room via the API
	createReq := protocol.CreateRoomRequest{
		Name:      "new-channel",
		IsPrivate: false,
	}
	createJSON, _ := json.Marshal(createReq)
	createResp, err := api.CreateRoom(user, createJSON)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	// List rooms and verify it appears
	listResp, err := api.ListRooms(user, []byte("{}"))
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	listData, ok := listResp.Data.(protocol.ListRoomsResponse)
	if !ok {
		t.Fatalf("Expected protocol.ListRoomsResponse data type, got %T", listResp.Data)
	}

	createData := createResp.Envelope.Data.(protocol.CreateRoomResponse)
	found := false
	for i, room := range listData.Rooms {
		if room.ID == createData.Room.ID {
			found = true
			if !listData.IsMember[i] {
				t.Error("Creator should be a member of the room")
			}
			break
		}
	}
	if !found {
		t.Error("Created room should appear in list")
	}
}
