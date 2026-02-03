package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

func TestDeleteMessage_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_del1234567890", "deleter")
	room := createTestRoom(t, database, "roo_del123456789", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "to be deleted")

	req := protocol.DeleteMessageRequest{MessageID: msgID}
	reqJSON, _ := json.Marshal(req)

	res, err := api.DeleteMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("DeleteMessage failed: %v", err)
	}

	if res.RoomID != room.ID {
		t.Errorf("expected room ID %s, got %s", room.ID, res.RoomID)
	}

	// Verify broadcast type
	var envelope protocol.Envelope
	err = json.Unmarshal(res.Message, &envelope)
	if err != nil {
		t.Fatalf("Failed to unmarshal broadcast: %v", err)
	}
	if envelope.Type != "message_deleted" {
		t.Errorf("expected type 'message_deleted', got %s", envelope.Type)
	}

	// Verify DB: body cleared, deleted_at set
	dbMsg, err := models.MessageByID(context.Background(), database, msgID)
	if err != nil {
		t.Fatalf("Failed to load message: %v", err)
	}
	if dbMsg.Body != "" {
		t.Errorf("expected empty body after delete, got '%s'", dbMsg.Body)
	}
	if !dbMsg.DeletedAt.Valid || dbMsg.DeletedAt.String == "" {
		t.Error("expected deleted_at to be set")
	}
}

func TestDeleteMessage_NotOwner(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	author := createTestUser(t, database, "usr_del_auth__123", "author")
	other := createTestUser(t, database, "usr_del_other_123", "other")
	room := createTestRoom(t, database, "roo_delown123456", "general", true)
	addUserToRoom(t, database, author.ID, room.ID)
	addUserToRoom(t, database, other.ID, room.ID)

	msgID := createTestMessageSimple(t, api, author, room.ID, "author's message")

	req := protocol.DeleteMessageRequest{MessageID: msgID}
	reqJSON, _ := json.Marshal(req)

	_, err := api.DeleteMessage(other, reqJSON)
	if err == nil {
		t.Fatal("expected error when deleting another user's message")
	}
}

func TestDeleteMessage_Idempotent(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_del_idem12345", "idempotent")
	room := createTestRoom(t, database, "roo_delidem12345", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "delete me twice")

	req := protocol.DeleteMessageRequest{MessageID: msgID}
	reqJSON, _ := json.Marshal(req)

	// First delete
	_, err := api.DeleteMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("First delete failed: %v", err)
	}

	// Second delete should also succeed (idempotent)
	_, err = api.DeleteMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("Second delete should be idempotent but failed: %v", err)
	}
}
