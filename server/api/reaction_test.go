package api

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/llimllib/hatchat/server/protocol"
)

func TestAddReaction_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_react123456789", "reactor")
	room := createTestRoom(t, database, "roo_react1234567", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "react to me")

	req := protocol.AddReactionRequest{MessageID: msgID, Emoji: "👍"}
	reqJSON, _ := json.Marshal(req)

	res, err := api.AddReaction(user, reqJSON)
	if err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}

	if res.RoomID != room.ID {
		t.Errorf("expected room ID %s, got %s", room.ID, res.RoomID)
	}

	// Verify broadcast
	var envelope protocol.Envelope
	err = json.Unmarshal(res.Message, &envelope)
	if err != nil {
		t.Fatalf("Failed to unmarshal broadcast: %v", err)
	}
	if envelope.Type != "reaction_updated" {
		t.Errorf("expected type 'reaction_updated', got %s", envelope.Type)
	}
}

func TestAddReaction_Idempotent(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_reactidem1234", "reactor")
	room := createTestRoom(t, database, "roo_reactidem123", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "react twice")

	req := protocol.AddReactionRequest{MessageID: msgID, Emoji: "👍"}
	reqJSON, _ := json.Marshal(req)

	// First reaction
	_, err := api.AddReaction(user, reqJSON)
	if err != nil {
		t.Fatalf("First AddReaction failed: %v", err)
	}

	// Second reaction (same emoji) should succeed (upsert)
	_, err = api.AddReaction(user, reqJSON)
	if err != nil {
		t.Fatalf("Second AddReaction should be idempotent but failed: %v", err)
	}
}

func TestAddReaction_DeletedMessage(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_reactdel12345", "reactor")
	room := createTestRoom(t, database, "roo_reactdel1234", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "will be deleted")

	// Delete the message
	delReq := protocol.DeleteMessageRequest{MessageID: msgID}
	delJSON, _ := json.Marshal(delReq)
	_, err := api.DeleteMessage(user, delJSON)
	if err != nil {
		t.Fatalf("DeleteMessage failed: %v", err)
	}

	// Try to react to deleted message
	req := protocol.AddReactionRequest{MessageID: msgID, Emoji: "👍"}
	reqJSON, _ := json.Marshal(req)
	_, err = api.AddReaction(user, reqJSON)
	if err == nil {
		t.Fatal("expected error when reacting to a deleted message")
	}
}

func TestAddReaction_NonMember(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	member := createTestUser(t, database, "usr_reactmem12345", "member")
	nonMember := createTestUser(t, database, "usr_reactnon12345", "nonmember")
	room := createTestRoom(t, database, "roo_reactnon1234", "general", true)
	addUserToRoom(t, database, member.ID, room.ID)
	// nonMember is NOT added to the room

	msgID := createTestMessageSimple(t, api, member, room.ID, "can't react to this")

	req := protocol.AddReactionRequest{MessageID: msgID, Emoji: "👍"}
	reqJSON, _ := json.Marshal(req)
	_, err := api.AddReaction(nonMember, reqJSON)
	if err == nil {
		t.Fatal("expected error when non-member tries to react")
	}
}

func TestRemoveReaction_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_unreact1234567", "unreactor")
	room := createTestRoom(t, database, "roo_unreact12345", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "react then unreact")

	// Add a reaction
	addReq := protocol.AddReactionRequest{MessageID: msgID, Emoji: "👍"}
	addJSON, _ := json.Marshal(addReq)
	_, err := api.AddReaction(user, addJSON)
	if err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}

	// Remove the reaction
	removeReq := protocol.RemoveReactionRequest{MessageID: msgID, Emoji: "👍"}
	removeJSON, _ := json.Marshal(removeReq)
	res, err := api.RemoveReaction(user, removeJSON)
	if err != nil {
		t.Fatalf("RemoveReaction failed: %v", err)
	}

	// Verify broadcast
	var envelope protocol.Envelope
	err = json.Unmarshal(res.Message, &envelope)
	if err != nil {
		t.Fatalf("Failed to unmarshal broadcast: %v", err)
	}
	if envelope.Type != "reaction_updated" {
		t.Errorf("expected type 'reaction_updated', got %s", envelope.Type)
	}
}

func TestRemoveReaction_Idempotent(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_unreactidem12", "unreactor")
	room := createTestRoom(t, database, "roo_unreactidem1", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "remove nonexistent")

	// Remove a reaction that doesn't exist
	req := protocol.RemoveReactionRequest{MessageID: msgID, Emoji: "👍"}
	reqJSON, _ := json.Marshal(req)
	_, err := api.RemoveReaction(user, reqJSON)
	if err != nil {
		t.Fatalf("RemoveReaction should be idempotent but failed: %v", err)
	}
}
