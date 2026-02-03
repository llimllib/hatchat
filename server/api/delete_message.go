package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// DeleteMessageResponse contains the broadcast data and room ID for routing
type DeleteMessageResponse struct {
	RoomID  string
	Message []byte
}

// DeleteMessage handles a request to soft-delete a message.
// Only the message author can delete. Returns a broadcast message for the room.
func (a *Api) DeleteMessage(user *models.User, msg json.RawMessage) (*DeleteMessageResponse, error) {
	var req protocol.DeleteMessageRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid json for delete_message", "error", err)
		return nil, err
	}

	if req.MessageID == "" {
		return nil, fmt.Errorf("message_id is required")
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
		a.logger.Warn("user attempted to delete another user's message", "user", user.ID, "message_owner", message.UserID)
		return nil, fmt.Errorf("can only delete your own messages")
	}

	// If already deleted, treat as idempotent success
	if message.DeletedAt.Valid && message.DeletedAt.String != "" {
		broadcast := protocol.MessageDeleted{
			MessageID: message.ID,
			RoomID:    message.RoomID,
		}
		msgBytes, err := json.Marshal(&Envelope{
			Type: "message_deleted",
			Data: broadcast,
		})
		if err != nil {
			return nil, err
		}
		return &DeleteMessageResponse{
			RoomID:  message.RoomID,
			Message: msgBytes,
		}, nil
	}

	// Verify room membership
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, message.RoomID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the room")
	}

	// Soft delete: clear body and set deleted_at
	now := time.Now().Format(time.RFC3339Nano)
	message.Body = ""
	message.DeletedAt.String = now
	message.DeletedAt.Valid = true
	message.ModifiedAt = now
	if err = message.Update(ctx, a.db); err != nil {
		a.logger.Error("failed to soft-delete message", "error", err)
		return nil, err
	}

	// Build broadcast
	broadcast := protocol.MessageDeleted{
		MessageID: message.ID,
		RoomID:    message.RoomID,
	}

	msgBytes, err := json.Marshal(&Envelope{
		Type: "message_deleted",
		Data: broadcast,
	})
	if err != nil {
		return nil, err
	}

	return &DeleteMessageResponse{
		RoomID:  message.RoomID,
		Message: msgBytes,
	}, nil
}
