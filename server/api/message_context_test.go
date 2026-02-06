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

func TestGetMessageContext_ReturnsMessageAndRoom(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	// Create user and room
	user := &models.User{
		ID:         "usr_test123456789a",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_general1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	_ = user.Insert(ctx, testDB)

	room := &models.Room{
		ID:        "roo_general1234",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 1,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	_ = room.Insert(ctx, testDB)

	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", user.ID, room.ID)

	// Create a message
	msg := &models.Message{
		ID:         "msg_test12345678",
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       "Hello world",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = msg.Insert(ctx, testDB)

	// Request message context
	req := protocol.GetMessageContextRequest{MessageID: msg.ID}
	reqData, _ := json.Marshal(req)

	resp, err := api.GetMessageContext(user, reqData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Type != "get_message_context" {
		t.Fatalf("expected get_message_context response, got %s", resp.Type)
	}

	contextResp, ok := resp.Data.(protocol.GetMessageContextResponse)
	if !ok {
		t.Fatalf("expected GetMessageContextResponse, got %T", resp.Data)
	}

	if contextResp.Message.ID != msg.ID {
		t.Errorf("expected message ID %s, got %s", msg.ID, contextResp.Message.ID)
	}
	if contextResp.RoomID != room.ID {
		t.Errorf("expected room ID %s, got %s", room.ID, contextResp.RoomID)
	}
	if contextResp.Message.Body != "Hello world" {
		t.Errorf("expected body 'Hello world', got '%s'", contextResp.Message.Body)
	}
}

func TestGetMessageContext_MessageNotFound(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	user := &models.User{ID: "usr_test123456789a"}

	req := protocol.GetMessageContextRequest{MessageID: "msg_nonexistent1"}
	reqData, _ := json.Marshal(req)

	resp, err := api.GetMessageContext(user, reqData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Type != "error" {
		t.Errorf("expected error response, got %s", resp.Type)
	}
}

func TestGetMessageContext_NoRoomAccess(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	// Create two users
	alice := &models.User{
		ID:         "usr_alice12345678",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_private1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	bob := &models.User{
		ID:         "usr_bob1234567890",
		Username:   "bob",
		Password:   "hash",
		LastRoom:   "roo_private1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	_ = alice.Insert(ctx, testDB)
	_ = bob.Insert(ctx, testDB)

	// Create private room - only alice is a member
	room := &models.Room{
		ID:        "roo_private1234",
		Name:      "private",
		RoomType:  "channel",
		IsPrivate: 1,
		IsDefault: 0,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	_ = room.Insert(ctx, testDB)

	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", alice.ID, room.ID)

	// Alice creates a message
	msg := &models.Message{
		ID:         "msg_private12345",
		RoomID:     room.ID,
		UserID:     alice.ID,
		Body:       "Secret message",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = msg.Insert(ctx, testDB)

	// Bob tries to access the message
	req := protocol.GetMessageContextRequest{MessageID: msg.ID}
	reqData, _ := json.Marshal(req)

	resp, err := api.GetMessageContext(bob, reqData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Type != "error" {
		t.Errorf("expected error response for unauthorized access, got %s", resp.Type)
	}

	errorResp, ok := resp.Data.(*protocol.ErrorResponse)
	if !ok {
		t.Fatalf("expected *ErrorResponse, got %T", resp.Data)
	}

	if errorResp.Message != "you don't have access to this message" {
		t.Errorf("unexpected error message: %s", errorResp.Message)
	}
}

func TestGetMessageContext_DeletedMessage(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	user := &models.User{
		ID:         "usr_test123456789a",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_general1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	_ = user.Insert(ctx, testDB)

	room := &models.Room{
		ID:        "roo_general1234",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 1,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	_ = room.Insert(ctx, testDB)

	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", user.ID, room.ID)

	// Create and delete a message
	msg := &models.Message{
		ID:         "msg_deleted12345",
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       "This will be deleted",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = msg.Insert(ctx, testDB)

	// Soft delete
	_, _ = testDB.ExecContext(ctx, "UPDATE messages SET deleted_at = $1 WHERE id = $2",
		time.Now().Format(time.RFC3339), msg.ID)

	// Request context for deleted message
	req := protocol.GetMessageContextRequest{MessageID: msg.ID}
	reqData, _ := json.Marshal(req)

	resp, err := api.GetMessageContext(user, reqData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still return the message (for navigation), but with empty body
	if resp.Type != "get_message_context" {
		t.Fatalf("expected get_message_context response, got %s", resp.Type)
	}

	contextResp := resp.Data.(protocol.GetMessageContextResponse)

	if contextResp.Message.Body != "" {
		t.Errorf("expected empty body for deleted message, got '%s'", contextResp.Message.Body)
	}
	if contextResp.Message.DeletedAt == "" {
		t.Errorf("expected deleted_at to be set")
	}
	if contextResp.RoomID != room.ID {
		t.Errorf("should still return room ID for navigation")
	}
}

func TestGetMessageContext_EmptyMessageID(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	user := &models.User{ID: "usr_test123456789a"}

	req := protocol.GetMessageContextRequest{MessageID: ""}
	reqData, _ := json.Marshal(req)

	resp, err := api.GetMessageContext(user, reqData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Type != "error" {
		t.Errorf("expected error response for empty message_id, got %s", resp.Type)
	}
}
