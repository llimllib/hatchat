package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

// createTestMessage creates a message in the database for testing
func createTestMessage(t *testing.T, database *db.DB, id, roomID, userID, body string, createdAt time.Time) *models.Message {
	t.Helper()
	msg := &models.Message{
		ID:         id,
		RoomID:     roomID,
		UserID:     userID,
		Body:       body,
		CreatedAt:  createdAt.Format(time.RFC3339),
		ModifiedAt: createdAt.Format(time.RFC3339),
	}
	err := msg.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}
	return msg
}

// TestHistoryMessage_ValidMember tests that a room member can fetch history
func TestHistoryMessage_ValidMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create user and room
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Create some messages
	now := time.Now()
	createTestMessage(t, database, "msg_test1234567", room.ID, user.ID, "Message 1", now.Add(-2*time.Minute))
	createTestMessage(t, database, "msg_test2345678", room.ID, user.ID, "Message 2", now.Add(-1*time.Minute))
	createTestMessage(t, database, "msg_test3456789", room.ID, user.ID, "Message 3", now)

	// Request history
	reqData := HistoryRequest{
		RoomID: room.ID,
		Limit:  50,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.HistoryMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("HistoryMessage failed: %v", err)
	}

	// Verify response
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	if response.Type != "history" {
		t.Errorf("Expected type 'history', got '%s'", response.Type)
	}

	historyResp, ok := response.Data.(HistoryResponse)
	if !ok {
		t.Fatalf("Expected HistoryResponse data type, got %T", response.Data)
	}

	if len(historyResp.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(historyResp.Messages))
	}

	// Messages should be in newest-first order
	if historyResp.Messages[0].Body != "Message 3" {
		t.Errorf("Expected newest message first, got %s", historyResp.Messages[0].Body)
	}
	if historyResp.Messages[2].Body != "Message 1" {
		t.Errorf("Expected oldest message last, got %s", historyResp.Messages[2].Body)
	}

	// Should include username
	if historyResp.Messages[0].Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", historyResp.Messages[0].Username)
	}
}

// TestHistoryMessage_NonMemberRejected tests that a non-member cannot fetch history
// SECURITY: Critical test - users must not be able to read messages from rooms they don't belong to
func TestHistoryMessage_NonMemberRejected(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create user and room, but DON'T add user to room
	user := createTestUser(t, database, "usr_test123456789", "testuser")
	otherUser := createTestUser(t, database, "usr_other12345678", "otheruser")
	room := createTestRoom(t, database, "roo_test12345678", "secret-room", false)
	addUserToRoom(t, database, otherUser.ID, room.ID)

	// Create some messages in the room
	now := time.Now()
	createTestMessage(t, database, "msg_test1234567", room.ID, otherUser.ID, "Secret message", now)

	// Try to fetch history for a room we're not a member of
	reqData := HistoryRequest{
		RoomID: room.ID,
		Limit:  50,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.HistoryMessage(user, reqJSON)

	// Verify the request was rejected
	if err == nil {
		t.Error("SECURITY BREACH: Expected error when non-member fetches history, got nil")
	}
	if response != nil {
		t.Error("SECURITY BREACH: Expected nil response when non-member fetches history")
	}
}

// TestHistoryMessage_Pagination tests cursor-based pagination
func TestHistoryMessage_Pagination(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Create 10 messages
	now := time.Now()
	for i := 0; i < 10; i++ {
		createTestMessage(t, database, models.GenerateMessageID(), room.ID, user.ID,
			"Message "+string(rune('A'+i)), now.Add(time.Duration(-10+i)*time.Minute))
	}

	// First page - request 3 messages
	reqData := HistoryRequest{
		RoomID: room.ID,
		Limit:  3,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.HistoryMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("HistoryMessage failed: %v", err)
	}

	historyResp := response.Data.(HistoryResponse)
	if len(historyResp.Messages) != 3 {
		t.Errorf("Expected 3 messages on first page, got %d", len(historyResp.Messages))
	}
	if !historyResp.HasMore {
		t.Error("Expected has_more to be true")
	}
	if historyResp.NextCursor == "" {
		t.Error("Expected non-empty next_cursor")
	}

	// Second page - use cursor
	reqData = HistoryRequest{
		RoomID: room.ID,
		Cursor: historyResp.NextCursor,
		Limit:  3,
	}
	reqJSON, _ = json.Marshal(reqData)

	response, err = api.HistoryMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("HistoryMessage failed on second page: %v", err)
	}

	historyResp2 := response.Data.(HistoryResponse)
	if len(historyResp2.Messages) != 3 {
		t.Errorf("Expected 3 messages on second page, got %d", len(historyResp2.Messages))
	}

	// Verify no overlap between pages
	firstPageIDs := make(map[string]bool)
	for _, m := range historyResp.Messages {
		firstPageIDs[m.ID] = true
	}
	for _, m := range historyResp2.Messages {
		if firstPageIDs[m.ID] {
			t.Errorf("Message %s appeared in both pages", m.ID)
		}
	}
}

// TestHistoryMessage_EmptyRoom tests history for a room with no messages
func TestHistoryMessage_EmptyRoom(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "empty-room", true)
	addUserToRoom(t, database, user.ID, room.ID)

	reqData := HistoryRequest{
		RoomID: room.ID,
		Limit:  50,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.HistoryMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("HistoryMessage failed: %v", err)
	}

	historyResp := response.Data.(HistoryResponse)
	if len(historyResp.Messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(historyResp.Messages))
	}
	if historyResp.HasMore {
		t.Error("Expected has_more to be false for empty room")
	}
}

// TestHistoryMessage_MissingRoomID tests that requests without room_id are rejected
func TestHistoryMessage_MissingRoomID(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	reqData := HistoryRequest{
		RoomID: "",
		Limit:  50,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.HistoryMessage(user, reqJSON)
	if err == nil {
		t.Error("Expected error for missing room_id, got nil")
	}
	if response != nil {
		t.Error("Expected nil response for missing room_id")
	}
}

// TestHistoryMessage_DefaultLimit tests that limit defaults to 50
func TestHistoryMessage_DefaultLimit(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Create 60 messages
	now := time.Now()
	for i := 0; i < 60; i++ {
		createTestMessage(t, database, models.GenerateMessageID(), room.ID, user.ID,
			"Message", now.Add(time.Duration(-60+i)*time.Minute))
	}

	// Request without specifying limit
	reqData := HistoryRequest{
		RoomID: room.ID,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.HistoryMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("HistoryMessage failed: %v", err)
	}

	historyResp := response.Data.(HistoryResponse)
	if len(historyResp.Messages) != 50 {
		t.Errorf("Expected default limit of 50 messages, got %d", len(historyResp.Messages))
	}
	if !historyResp.HasMore {
		t.Error("Expected has_more to be true with 60 messages and limit 50")
	}
}

// TestHistoryMessage_MaxLimit tests that limit is capped at 100
func TestHistoryMessage_MaxLimit(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")
	room := createTestRoom(t, database, "roo_test12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	// Create 150 messages
	now := time.Now()
	for i := 0; i < 150; i++ {
		createTestMessage(t, database, models.GenerateMessageID(), room.ID, user.ID,
			"Message", now.Add(time.Duration(-150+i)*time.Minute))
	}

	// Request with limit exceeding max
	reqData := HistoryRequest{
		RoomID: room.ID,
		Limit:  500,
	}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.HistoryMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("HistoryMessage failed: %v", err)
	}

	historyResp := response.Data.(HistoryResponse)
	if len(historyResp.Messages) != 100 {
		t.Errorf("Expected max limit of 100 messages, got %d", len(historyResp.Messages))
	}
}

// TestHistoryMessage_InvalidJSON tests that invalid JSON is rejected
func TestHistoryMessage_InvalidJSON(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_test123456789", "testuser")

	invalidJSON := json.RawMessage(`{invalid json}`)

	response, err := api.HistoryMessage(user, invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
	if response != nil {
		t.Error("Expected nil response for invalid JSON")
	}
}

// TestHistoryMessage_MultipleRoomsSecurity tests that users can only fetch history from their own rooms
// SECURITY: Critical test - verifies room isolation for history
func TestHistoryMessage_MultipleRoomsSecurity(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	// Create two users
	alice := createTestUser(t, database, "usr_alice1234567", "alice")
	bob := createTestUser(t, database, "usr_bob12345678", "bob")

	// Create two rooms
	aliceRoom := createTestRoom(t, database, "roo_alice123456", "alice-room", false)
	bobRoom := createTestRoom(t, database, "roo_bob1234567", "bob-room", false)

	// Add users to their respective rooms
	addUserToRoom(t, database, alice.ID, aliceRoom.ID)
	addUserToRoom(t, database, bob.ID, bobRoom.ID)

	// Create messages in each room
	now := time.Now()
	createTestMessage(t, database, "msg_alice123456", aliceRoom.ID, alice.ID, "Alice's secret", now)
	createTestMessage(t, database, "msg_bob12345678", bobRoom.ID, bob.ID, "Bob's secret", now)

	// Alice tries to fetch Bob's room history (should fail)
	reqData := HistoryRequest{RoomID: bobRoom.ID, Limit: 50}
	reqJSON, _ := json.Marshal(reqData)

	response, err := api.HistoryMessage(alice, reqJSON)
	if err == nil {
		t.Error("SECURITY BREACH: Alice was able to fetch Bob's room history")
	}
	if response != nil {
		t.Error("Expected nil response when fetching unauthorized room history")
	}

	// Bob tries to fetch Alice's room history (should fail)
	reqData = HistoryRequest{RoomID: aliceRoom.ID, Limit: 50}
	reqJSON, _ = json.Marshal(reqData)

	response, err = api.HistoryMessage(bob, reqJSON)
	if err == nil {
		t.Error("SECURITY BREACH: Bob was able to fetch Alice's room history")
	}
	if response != nil {
		t.Error("Expected nil response when fetching unauthorized room history")
	}

	// Alice can fetch her own room (should succeed)
	reqData = HistoryRequest{RoomID: aliceRoom.ID, Limit: 50}
	reqJSON, _ = json.Marshal(reqData)

	response, err = api.HistoryMessage(alice, reqJSON)
	if err != nil {
		t.Errorf("Alice should be able to fetch her own room history: %v", err)
	}
	if response == nil {
		t.Error("Expected non-nil response for authorized room")
	}

	// Bob can fetch his own room (should succeed)
	reqData = HistoryRequest{RoomID: bobRoom.ID, Limit: 50}
	reqJSON, _ = json.Marshal(reqData)

	response, err = api.HistoryMessage(bob, reqJSON)
	if err != nil {
		t.Errorf("Bob should be able to fetch his own room history: %v", err)
	}
	if response == nil {
		t.Error("Expected non-nil response for authorized room")
	}
}
