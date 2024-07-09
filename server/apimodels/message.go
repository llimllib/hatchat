package apimodels

import (
	"github.com/llimllib/hatchat/server/models"
)

type Message struct {
	ID         string        `json:"id"`
	RoomID     string        `json:"room_id"`
	UserID     string        `json:"user_id"`
	Body       string        `json:"body"`
	CreatedAt  models.Time `json:"created_at"`
	ModifiedAt models.Time `json:"modified_at"`
}

func NewMessage(ID, roomID, userID, body string, createdAt, modifiedAt models.Time) *Message {
	return &Message{
		ID:         ID,
		RoomID:     roomID,
		UserID:     userID,
		Body:       body,
		CreatedAt:  createdAt,
		ModifiedAt: modifiedAt,
	}
}
