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

// MessageResponse contains the message data and the room ID for routing
type MessageResponse struct {
	RoomID  string
	Message []byte
}

// MessageMessage accepts a message from a user that has yet to be unmarshaled,
// writes it to the database and returns a MessageResponse with the message
// JSON and room ID for routing
func (a *Api) MessageMessage(user *models.User, msg json.RawMessage) (*MessageResponse, error) {
	var req protocol.SendMessageRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid json", "error", err)
		return nil, err
	}

	// if the message is empty or there's no room, error out
	if len(req.Body) < 1 || len(req.RoomID) < 1 {
		a.logger.Error("invalid message", "msg", string(msg))
		return nil, fmt.Errorf("invalid message <%s> <%s>", req.Body, req.RoomID)
	}

	ctx := context.Background()

	// Validate that the user is a member of the room
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, req.RoomID)
	if err != nil {
		a.logger.Error("failed to check room membership", "error", err, "user", user.ID, "room", req.RoomID)
		return nil, err
	}
	if !isMember {
		a.logger.Warn("user attempted to send message to room they are not a member of", "user", user.ID, "room", req.RoomID)
		return nil, fmt.Errorf("user is not a member of room %s", req.RoomID)
	}

	room, err := models.RoomByID(ctx, a.db, req.RoomID)
	if err != nil {
		a.logger.Error("unable to find room", "error", err, "room", req.RoomID)
		return nil, err
	}

	now := time.Now().Format(time.RFC3339Nano)
	dbMessage := models.Message{
		ID:         models.GenerateMessageID(),
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       req.Body,
		CreatedAt:  now,
		ModifiedAt: now,
	}
	if err = dbMessage.Insert(ctx, a.db); err != nil {
		a.logger.Error("unable to insert message", "error", err)
		return nil, err
	}

	// Update room's last_message_at for DM ordering
	room.LastMessageAt.String = now
	room.LastMessageAt.Valid = true
	if err = room.Update(ctx, a.db); err != nil {
		// Log but don't fail - the message was already sent
		a.logger.Error("failed to update room last_message_at", "error", err, "room", room.ID)
	}

	// Create broadcast message with full message details using protocol.Message
	broadcastMsg := protocol.Message{
		ID:         dbMessage.ID,
		Body:       dbMessage.Body,
		RoomID:     dbMessage.RoomID,
		UserID:     dbMessage.UserID,
		Username:   user.Username,
		CreatedAt:  dbMessage.CreatedAt,
		ModifiedAt: dbMessage.ModifiedAt,
	}

	msgBytes, err := json.Marshal(&Envelope{
		Type: "message",
		Data: broadcastMsg,
	})
	if err != nil {
		return nil, err
	}

	return &MessageResponse{
		RoomID:  room.ID,
		Message: msgBytes,
	}, nil
}
