package db

import (
	"context"
	"testing"
)

func TestAddRoomMember_NewMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// Create user and room
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", false)

	// Add user to room
	added, err := AddRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("AddRoomMember failed: %v", err)
	}
	if !added {
		t.Error("Expected added=true for new member")
	}

	// Verify membership
	isMember, err := IsRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if !isMember {
		t.Error("User should be a member after AddRoomMember")
	}
}

func TestAddRoomMember_AlreadyMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// Create user and room, add membership directly
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", false)
	addUserToRoom(t, database, user.ID, room.ID)

	// Try to add again
	added, err := AddRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("AddRoomMember failed: %v", err)
	}
	if added {
		t.Error("Expected added=false for existing member")
	}
}

func TestAddRoomMember_Idempotent(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// Create user and room
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", false)

	// Add twice
	added1, err := AddRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("First AddRoomMember failed: %v", err)
	}
	if !added1 {
		t.Error("Expected added=true for first call")
	}

	added2, err := AddRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("Second AddRoomMember failed: %v", err)
	}
	if added2 {
		t.Error("Expected added=false for second call")
	}

	// Verify still a member
	isMember, err := IsRoomMember(ctx, database, user.ID, room.ID)
	if err != nil {
		t.Fatalf("IsRoomMember failed: %v", err)
	}
	if !isMember {
		t.Error("User should still be a member")
	}
}
