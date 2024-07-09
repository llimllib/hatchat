package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/llimllib/hatchat/server/models"
)

type Message struct {
	Body string
}

// We've received a message:
// - unmarshal it
// - save it to the database
// - return it, with an ID, to the sender for display
func (a *Api) MessageMessage(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var m Message
	if err := json.Unmarshal(msg, &m); err != nil {
		a.logger.Error("invalid json", "error", err)
		return nil, err
	}

	room, err := models.GetDefaultRoom(context.Background(), a.db)
	if err != nil {
		a.logger.Error("unable to find default room", "error", err)
		return nil, err
	}

	dbMessage := models.Message{
		ID:         models.GenerateMessageID(),
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       m.Body,
		CreatedAt:  models.NewTime(time.Now()),
		ModifiedAt: models.NewTime(time.Now()),
	}
	if err = dbMessage.Insert(context.Background(), a.db); err != nil {
		a.logger.Error("unable to find default room", "error", err)
		return nil, err
	}

	return &Envelope{
		Type: "message",
		Data: msg,
	}, nil
}
