package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// createTestMessageSimple creates a message in the database for testing (simplified helper)
func createTestMessageSimple(t *testing.T, api *Api, user *models.User, roomID, body string) string {
	t.Helper()
	now := time.Now().Format(time.RFC3339Nano)
	msg := models.Message{
		ID:         models.GenerateMessageID(),
		RoomID:     roomID,
		UserID:     user.ID,
		Body:       body,
		CreatedAt:  now,
		ModifiedAt: now,
	}
	err := msg.Insert(context.Background(), api.db)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}
	return msg.ID
}

func TestEditMessage_Success(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_edit1234567890", "editor")
	room := createTestRoom(t, database, "roo_edit12345678", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "original body")

	// Edit the message
	req := protocol.EditMessageRequest{
		MessageID: msgID,
		Body:      "edited body",
	}
	reqJSON, _ := json.Marshal(req)

	res, err := api.EditMessage(user, reqJSON)
	if err != nil {
		t.Fatalf("EditMessage failed: %v", err)
	}

	if res.RoomID != room.ID {
		t.Errorf("expected room ID %s, got %s", room.ID, res.RoomID)
	}

	// Verify the broadcast
	var envelope protocol.Envelope
	err = json.Unmarshal(res.Message, &envelope)
	if err != nil {
		t.Fatalf("Failed to unmarshal broadcast: %v", err)
	}
	if envelope.Type != "message_edited" {
		t.Errorf("expected type 'message_edited', got %s", envelope.Type)
	}

	// Verify DB was updated
	dbMsg, err := models.MessageByID(context.Background(), database, msgID)
	if err != nil {
		t.Fatalf("Failed to load message: %v", err)
	}
	if dbMsg.Body != "edited body" {
		t.Errorf("expected body 'edited body', got '%s'", dbMsg.Body)
	}
}

func TestEditMessage_NotOwner(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	author := createTestUser(t, database, "usr_edit_author_01", "author")
	other := createTestUser(t, database, "usr_edit_other__01", "other")
	room := createTestRoom(t, database, "roo_editown12345", "general", true)
	addUserToRoom(t, database, author.ID, room.ID)
	addUserToRoom(t, database, other.ID, room.ID)

	msgID := createTestMessageSimple(t, api, author, room.ID, "author's message")

	// Other user tries to edit
	req := protocol.EditMessageRequest{MessageID: msgID, Body: "hacked"}
	reqJSON, _ := json.Marshal(req)

	_, err := api.EditMessage(other, reqJSON)
	if err == nil {
		t.Fatal("expected error when editing another user's message")
	}
}

func TestEditMessage_DeletedMessage(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_edit_del12345", "deleter")
	room := createTestRoom(t, database, "roo_editdel12345", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "to be deleted")

	// Soft delete the message first
	delReq := protocol.DeleteMessageRequest{MessageID: msgID}
	delJSON, _ := json.Marshal(delReq)
	_, err := api.DeleteMessage(user, delJSON)
	if err != nil {
		t.Fatalf("DeleteMessage failed: %v", err)
	}

	// Try to edit the deleted message
	req := protocol.EditMessageRequest{MessageID: msgID, Body: "edited"}
	reqJSON, _ := json.Marshal(req)

	_, err = api.EditMessage(user, reqJSON)
	if err == nil {
		t.Fatal("expected error when editing a deleted message")
	}
}

func TestEditMessage_EmptyBody(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(database, logger)

	user := createTestUser(t, database, "usr_edit_empty123", "emptier")
	room := createTestRoom(t, database, "roo_editempty123", "general", true)
	addUserToRoom(t, database, user.ID, room.ID)

	msgID := createTestMessageSimple(t, api, user, room.ID, "non-empty")

	// Try to edit with empty body
	req := protocol.EditMessageRequest{MessageID: msgID, Body: "   "}
	reqJSON, _ := json.Marshal(req)

	_, err := api.EditMessage(user, reqJSON)
	if err == nil {
		t.Fatal("expected error when editing with empty body")
	}
}
