package db

import (
	"context"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/models"
)

func TestGetRoomMessages_Basic(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create user and room
	user := &models.User{
		ID:          "usr_test123456789",
		Username:    "testuser",
		Password:    "hash",
		DisplayName: "",
		Status:      "",
		LastRoom:    "",
		CreatedAt:   now.Format(time.RFC3339),
		ModifiedAt:  now.Format(time.RFC3339),
	}
	if err := user.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	room := &models.Room{
		ID:        "roo_test12345678",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: models.FALSE,
		IsDefault: models.TRUE,
		CreatedAt: now.Format(time.RFC3339),
	}
	if err := room.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	// Create messages
	for i := 0; i < 5; i++ {
		msg := &models.Message{
			ID:         models.GenerateMessageID(),
			RoomID:     room.ID,
			UserID:     user.ID,
			Body:       "Message " + string(rune('A'+i)),
			CreatedAt:  now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			ModifiedAt: now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
		}
		if err := msg.Insert(ctx, database); err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
	}

	// Fetch messages
	messages, err := GetRoomMessages(ctx, database, room.ID, "", 10)
	if err != nil {
		t.Fatalf("GetRoomMessages failed: %v", err)
	}

	if len(messages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(messages))
	}

	// Verify order (newest first)
	if messages[0].Body != "Message E" {
		t.Errorf("Expected newest message first, got %s", messages[0].Body)
	}
	if messages[4].Body != "Message A" {
		t.Errorf("Expected oldest message last, got %s", messages[4].Body)
	}

	// Verify username is included
	if messages[0].Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", messages[0].Username)
	}
}

func TestGetRoomMessages_Pagination(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create user and room
	user := &models.User{
		ID:          "usr_test123456789",
		Username:    "testuser",
		Password:    "hash",
		DisplayName: "",
		Status:      "",
		LastRoom:    "",
		CreatedAt:   now.Format(time.RFC3339),
		ModifiedAt:  now.Format(time.RFC3339),
	}
	if err := user.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	room := &models.Room{
		ID:        "roo_test12345678",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: models.FALSE,
		IsDefault: models.TRUE,
		CreatedAt: now.Format(time.RFC3339),
	}
	if err := room.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	// Create 10 messages
	for i := 0; i < 10; i++ {
		msg := &models.Message{
			ID:         models.GenerateMessageID(),
			RoomID:     room.ID,
			UserID:     user.ID,
			Body:       "Message " + string(rune('A'+i)),
			CreatedAt:  now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			ModifiedAt: now.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
		}
		if err := msg.Insert(ctx, database); err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
	}

	// First page - limit 3
	page1, err := GetRoomMessages(ctx, database, room.ID, "", 3)
	if err != nil {
		t.Fatalf("GetRoomMessages page 1 failed: %v", err)
	}
	if len(page1) != 3 {
		t.Errorf("Expected 3 messages on page 1, got %d", len(page1))
	}
	// Should be J, I, H (newest 3)
	if page1[0].Body != "Message J" {
		t.Errorf("Expected 'Message J' first, got '%s'", page1[0].Body)
	}

	// Second page - use cursor from last message of page 1
	cursor := page1[2].CreatedAt
	page2, err := GetRoomMessages(ctx, database, room.ID, cursor, 3)
	if err != nil {
		t.Fatalf("GetRoomMessages page 2 failed: %v", err)
	}
	if len(page2) != 3 {
		t.Errorf("Expected 3 messages on page 2, got %d", len(page2))
	}
	// Should be G, F, E
	if page2[0].Body != "Message G" {
		t.Errorf("Expected 'Message G' first on page 2, got '%s'", page2[0].Body)
	}

	// Verify no overlap
	for _, m1 := range page1 {
		for _, m2 := range page2 {
			if m1.ID == m2.ID {
				t.Errorf("Message %s appeared on both pages", m1.ID)
			}
		}
	}
}

func TestGetRoomMessages_EmptyRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create room only
	room := &models.Room{
		ID:        "roo_test12345678",
		Name:      "empty-room",
		RoomType:  "channel",
		IsPrivate: models.FALSE,
		IsDefault: models.TRUE,
		CreatedAt: now.Format(time.RFC3339),
	}
	if err := room.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	// Fetch messages from empty room
	messages, err := GetRoomMessages(ctx, database, room.ID, "", 10)
	if err != nil {
		t.Fatalf("GetRoomMessages failed: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages for empty room, got %d", len(messages))
	}
}

func TestGetRoomMessages_RoomIsolation(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create user
	user := &models.User{
		ID:          "usr_test123456789",
		Username:    "testuser",
		Password:    "hash",
		DisplayName: "",
		Status:      "",
		LastRoom:    "",
		CreatedAt:   now.Format(time.RFC3339),
		ModifiedAt:  now.Format(time.RFC3339),
	}
	if err := user.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create two rooms
	room1 := &models.Room{
		ID:        "roo_room1234567",
		Name:      "room1",
		RoomType:  "channel",
		IsPrivate: models.FALSE,
		IsDefault: models.FALSE,
		CreatedAt: now.Format(time.RFC3339),
	}
	room2 := &models.Room{
		ID:        "roo_room2345678",
		Name:      "room2",
		RoomType:  "channel",
		IsPrivate: models.FALSE,
		IsDefault: models.FALSE,
		CreatedAt: now.Format(time.RFC3339),
	}
	if err := room1.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create room1: %v", err)
	}
	if err := room2.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create room2: %v", err)
	}

	// Create messages in room1
	msg1 := &models.Message{
		ID:         "msg_room1_12345",
		RoomID:     room1.ID,
		UserID:     user.ID,
		Body:       "Message in room1",
		CreatedAt:  now.Format(time.RFC3339),
		ModifiedAt: now.Format(time.RFC3339),
	}
	if err := msg1.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Create messages in room2
	msg2 := &models.Message{
		ID:         "msg_room2_12345",
		RoomID:     room2.ID,
		UserID:     user.ID,
		Body:       "Message in room2",
		CreatedAt:  now.Format(time.RFC3339),
		ModifiedAt: now.Format(time.RFC3339),
	}
	if err := msg2.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Fetch room1 messages - should only get room1 message
	room1Messages, err := GetRoomMessages(ctx, database, room1.ID, "", 10)
	if err != nil {
		t.Fatalf("GetRoomMessages for room1 failed: %v", err)
	}
	if len(room1Messages) != 1 {
		t.Errorf("Expected 1 message for room1, got %d", len(room1Messages))
	}
	if room1Messages[0].Body != "Message in room1" {
		t.Errorf("Wrong message body for room1")
	}

	// Fetch room2 messages - should only get room2 message
	room2Messages, err := GetRoomMessages(ctx, database, room2.ID, "", 10)
	if err != nil {
		t.Fatalf("GetRoomMessages for room2 failed: %v", err)
	}
	if len(room2Messages) != 1 {
		t.Errorf("Expected 1 message for room2, got %d", len(room2Messages))
	}
	if room2Messages[0].Body != "Message in room2" {
		t.Errorf("Wrong message body for room2")
	}
}
