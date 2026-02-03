package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// EditMessageResponse contains the broadcast data and room ID for routing
type EditMessageResponse struct {
	RoomID  string
	Message []byte
}

// EditMessage handles a request to edit a message's body.
// Only the message author can edit. Returns a broadcast message for the room.
func (a *Api) EditMessage(user *models.User, msg json.RawMessage) (*EditMessageResponse, error) {
	var req protocol.EditMessageRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid json for edit_message", "error", err)
		return nil, err
	}

	if req.MessageID == "" {
		return nil, fmt.Errorf("message_id is required")
	}
	if len(strings.TrimSpace(req.Body)) == 0 {
		return nil, fmt.Errorf("body must not be empty")
	}

	ctx := context.Background()

	// Look up the message
	message, err := models.MessageByID(ctx, a.db, req.MessageID)
	if err != nil {
		a.logger.Error("message not found", "error", err, "message_id", req.MessageID)
		return nil, fmt.Errorf("message not found")
	}

	// Check ownership
	if message.UserID != user.ID {
		a.logger.Warn("user attempted to edit another user's message", "user", user.ID, "message_owner", message.UserID)
		return nil, fmt.Errorf("can only edit your own messages")
	}

	// Check if deleted
	if message.DeletedAt.Valid && message.DeletedAt.String != "" {
		return nil, fmt.Errorf("cannot edit a deleted message")
	}

	// Verify room membership
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, message.RoomID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the room")
	}

	// Update the message
	now := time.Now().Format(time.RFC3339Nano)
	message.Body = req.Body
	message.ModifiedAt = now
	if err = message.Update(ctx, a.db); err != nil {
		a.logger.Error("failed to update message", "error", err)
		return nil, err
	}

	// Build broadcast
	broadcast := protocol.MessageEdited{
		MessageID:  message.ID,
		Body:       message.Body,
		RoomID:     message.RoomID,
		ModifiedAt: now,
	}

	msgBytes, err := json.Marshal(&Envelope{
		Type: "message_edited",
		Data: broadcast,
	})
	if err != nil {
		return nil, err
	}

	return &EditMessageResponse{
		RoomID:  message.RoomID,
		Message: msgBytes,
	}, nil
}
