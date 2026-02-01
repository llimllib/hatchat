package db

import (
	"context"
	"testing"
)

func TestListPublicRooms_Empty(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	rooms, err := ListPublicRooms(ctx, database)
	if err != nil {
		t.Fatalf("ListPublicRooms failed: %v", err)
	}
	if len(rooms) != 0 {
		t.Errorf("Expected 0 rooms, got %d", len(rooms))
	}
}

func TestListPublicRooms_OnlyPublic(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// Create one public and one private room
	publicRoom := createTestRoom(t, database, "roo_public123456", "public-channel", false)
	_ = createTestRoomWithPrivate(t, database, "roo_private12345", "private-channel", false, true)

	rooms, err := ListPublicRooms(ctx, database)
	if err != nil {
		t.Fatalf("ListPublicRooms failed: %v", err)
	}
	if len(rooms) != 1 {
		t.Errorf("Expected 1 room, got %d", len(rooms))
	}
	if len(rooms) > 0 && rooms[0].ID != publicRoom.ID {
		t.Errorf("Expected room ID %s, got %s", publicRoom.ID, rooms[0].ID)
	}
}

func TestListPublicRooms_OrderedByName(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// Create rooms in non-alphabetical order
	createTestRoom(t, database, "roo_zzzzzzzzzzzz", "zebra", false)
	createTestRoom(t, database, "roo_aaaaaaaaaaaa", "alpha", false)
	createTestRoom(t, database, "roo_mmmmmmmmmmmm", "middle", false)

	rooms, err := ListPublicRooms(ctx, database)
	if err != nil {
		t.Fatalf("ListPublicRooms failed: %v", err)
	}
	if len(rooms) != 3 {
		t.Errorf("Expected 3 rooms, got %d", len(rooms))
	}

	expectedOrder := []string{"alpha", "middle", "zebra"}
	for i, name := range expectedOrder {
		if rooms[i].Name != name {
			t.Errorf("Expected room %d to be '%s', got '%s'", i, name, rooms[i].Name)
		}
	}
}

func TestListPublicRoomsWithMembership_Empty(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	rooms, membership, err := ListPublicRoomsWithMembership(ctx, database, user.ID)
	if err != nil {
		t.Fatalf("ListPublicRoomsWithMembership failed: %v", err)
	}
	if len(rooms) != 0 {
		t.Errorf("Expected 0 rooms, got %d", len(rooms))
	}
	if len(membership) != 0 {
		t.Errorf("Expected 0 membership flags, got %d", len(membership))
	}
}

func TestListPublicRoomsWithMembership_WithMembership(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create two public rooms, add user to one
	room1 := createTestRoom(t, database, "roo_aaaaaaaaaaaa", "alpha-channel", false)
	room2 := createTestRoom(t, database, "roo_bbbbbbbbbbbb", "beta-channel", false)
	addUserToRoom(t, database, user.ID, room1.ID)

	rooms, membership, err := ListPublicRoomsWithMembership(ctx, database, user.ID)
	if err != nil {
		t.Fatalf("ListPublicRoomsWithMembership failed: %v", err)
	}
	if len(rooms) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(rooms))
	}
	if len(membership) != 2 {
		t.Errorf("Expected 2 membership flags, got %d", len(membership))
	}

	// Rooms are ordered by name (alpha, beta)
	for i, room := range rooms {
		switch room.ID {
		case room1.ID:
			if !membership[i] {
				t.Error("Expected user to be a member of room1")
			}
		case room2.ID:
			if membership[i] {
				t.Error("Expected user to NOT be a member of room2")
			}
		}
	}
}

func TestListPublicRoomsWithMembership_OnlyPublic(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	// Create one public and one private room
	publicRoom := createTestRoom(t, database, "roo_public123456", "public-channel", false)
	privateRoom := createTestRoomWithPrivate(t, database, "roo_private12345", "private-channel", false, true)

	// Add user to both
	addUserToRoom(t, database, user.ID, publicRoom.ID)
	addUserToRoom(t, database, user.ID, privateRoom.ID)

	rooms, membership, err := ListPublicRoomsWithMembership(ctx, database, user.ID)
	if err != nil {
		t.Fatalf("ListPublicRoomsWithMembership failed: %v", err)
	}

	// Only public room should be listed
	if len(rooms) != 1 {
		t.Errorf("Expected 1 room, got %d", len(rooms))
	}
	if len(rooms) > 0 {
		if rooms[0].ID != publicRoom.ID {
			t.Errorf("Expected room ID %s, got %s", publicRoom.ID, rooms[0].ID)
		}
		if !membership[0] {
			t.Error("Expected user to be a member of the public room")
		}
	}
}
