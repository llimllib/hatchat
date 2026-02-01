package db

import (
	"context"
	"testing"

	"github.com/llimllib/hatchat/server/models"
)

func TestLeaveRoom_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "test-channel", false)
	addUserToRoom(t, database, user.ID, room.ID)

	// Verify membership
	isMember, err := IsRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if !isMember {
		t.Fatal("User should be a member before leaving")
	}

	// Leave room
	left, err := LeaveRoom(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}
	if !left {
		t.Error("LeaveRoom should return true for successful leave")
	}

	// Verify no longer a member
	isMember, err = IsRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if isMember {
		t.Error("User should not be a member after leaving")
	}
}

func TestLeaveRoom_NotMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	_ = createTestRoom(t, database, "roo_test12345678", "test-channel", false)
	// Don't add user to room

	left, err := LeaveRoom(ctx, database, user.ID, "roo_test12345678")
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}
	if left {
		t.Error("LeaveRoom should return false when user is not a member")
	}
}

func TestLeaveRoom_DefaultRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoomWithDefault(t, database, "roo_default12345", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	left, err := LeaveRoom(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}
	if left {
		t.Error("LeaveRoom should return false for default room")
	}

	// Verify still a member
	isMember, err := IsRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if !isMember {
		t.Error("User should still be a member of default room")
	}
}

// createTestRoomWithDefault is a helper that creates a test room with explicit default flag
func createTestRoomWithDefault(t *testing.T, database *DB, id, name string, isDefault bool) *models.Room {
	t.Helper()
	ctx := context.Background()
	isDefaultInt := 0
	if isDefault {
		isDefaultInt = 1
	}
	room := &models.Room{
		ID:        id,
		Name:      name,
		IsPrivate: 0,
		IsDefault: isDefaultInt,
		CreatedAt: "2024-01-01T00:00:00Z",
	}
	err := room.Insert(ctx, database)
	if err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}
	return room
}
