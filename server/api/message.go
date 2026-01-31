package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/llimllib/hatchat/server/models"
)

type Message struct {
	Body   string `json:"body"`
	RoomID string `json:"room_id"`
}

// MessageResponse contains the message data and the room ID for routing
type MessageResponse struct {
	RoomID  string
	Message []byte
}

// MessageMessage accepts a message from a user that has yet to be unmarshaled,
// writes it to the database and returns a MessageResponse with the message
// JSON and room ID for routing
func (a *Api) MessageMessage(user *models.User, msg json.RawMessage) (*MessageResponse, error) {
	var m Message
	if err := json.Unmarshal(msg, &m); err != nil {
		a.logger.Error("invalid json", "error", err)
		return nil, err
	}

	// if the message is empty or there's no room, error out
	if len(m.Body) < 1 || len(m.RoomID) < 1 {
		a.logger.Error("invalid message", "msg", string(msg))
		return nil, fmt.Errorf("invalid message <%s> <%s>", m.Body, m.RoomID)
	}

	ctx := context.Background()

	// Validate that the user is a member of the room
	isMember, err := models.IsRoomMember(ctx, a.db, user.ID, m.RoomID)
	if err != nil {
		a.logger.Error("failed to check room membership", "error", err, "user", user.ID, "room", m.RoomID)
		return nil, err
	}
	if !isMember {
		a.logger.Warn("user attempted to send message to room they are not a member of", "user", user.ID, "room", m.RoomID)
		return nil, fmt.Errorf("user is not a member of room %s", m.RoomID)
	}

	room, err := models.RoomByID(ctx, a.db, m.RoomID)
	if err != nil {
		a.logger.Error("unable to find room", "error", err, "room", m.RoomID)
		return nil, err
	}

	dbMessage := models.Message{
		ID:         models.GenerateMessageID(),
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       m.Body,
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	if err = dbMessage.Insert(ctx, a.db); err != nil {
		a.logger.Error("unable to insert message", "error", err)
		return nil, err
	}

	msgBytes, err := json.Marshal(&Envelope{
		Type: "message",
		Data: msg,
	})
	if err != nil {
		return nil, err
	}

	return &MessageResponse{
		RoomID:  room.ID,
		Message: msgBytes,
	}, nil
}
