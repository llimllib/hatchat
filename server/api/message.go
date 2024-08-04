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

// MessageMessage accepts a message from a user that has yet to be unmarshaled,
// writes it to the database and returns an Api.Message marshaled to json
func (a *Api) MessageMessage(user *models.User, msg json.RawMessage) ([]byte, error) {
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

	room, err := models.RoomByID(context.Background(), a.db, m.RoomID)
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
	if err = dbMessage.Insert(context.Background(), a.db); err != nil {
		a.logger.Error("unable to find default room", "error", err)
		return nil, err
	}

	return json.Marshal(&Envelope{
		Type: "message",
		Data: msg,
	})
}
