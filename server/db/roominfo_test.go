package db

import (
	"context"
	"testing"
)

func TestGetRoomInfo_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	user1 := createTestUser(t, database, "usr_test123456789", "testuser1")
	user2 := createTestUser(t, database, "usr_test987654321", "testuser2")
	room := createTestRoom(t, database, "roo_test12345678", "test-channel", false)
	addUserToRoom(t, database, user1.ID, room.ID)
	addUserToRoom(t, database, user2.ID, room.ID)

	info, err := GetRoomInfo(ctx, database, room.ID)
	if err != nil {
		t.Fatalf("GetRoomInfo failed: %v", err)
	}

	if info.Room.ID != room.ID {
		t.Errorf("Expected room ID '%s', got '%s'", room.ID, info.Room.ID)
	}

	if info.Room.Name != "test-channel" {
		t.Errorf("Expected room name 'test-channel', got '%s'", info.Room.Name)
	}

	if info.MemberCount != 2 {
		t.Errorf("Expected member count 2, got %d", info.MemberCount)
	}

	if len(info.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(info.Members))
	}

	// Check that members are sorted by username
	if len(info.Members) >= 2 {
		if info.Members[0].Username != "testuser1" {
			t.Errorf("Expected first member to be 'testuser1', got '%s'", info.Members[0].Username)
		}
		if info.Members[1].Username != "testuser2" {
			t.Errorf("Expected second member to be 'testuser2', got '%s'", info.Members[1].Username)
		}
	}
}

func TestGetRoomInfo_RoomNotFound(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	_, err := GetRoomInfo(ctx, database, "roo_nonexistent1")
	if err == nil {
		t.Error("GetRoomInfo should fail for non-existent room")
	}
}

func TestGetRoomInfo_NoMembers(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	room := createTestRoom(t, database, "roo_test12345678", "test-channel", false)

	info, err := GetRoomInfo(ctx, database, room.ID)
	if err != nil {
		t.Fatalf("GetRoomInfo failed: %v", err)
	}

	if info.MemberCount != 0 {
		t.Errorf("Expected member count 0, got %d", info.MemberCount)
	}

	if len(info.Members) != 0 {
		t.Errorf("Expected 0 members, got %d", len(info.Members))
	}
}
