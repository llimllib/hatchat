package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/apimodels"
	"github.com/llimllib/hatchat/server/models"
)

type Init struct {
	User  *apimodels.User
	Rooms []*apimodels.Room
}

func (a *Api) InitMessage(user *models.User, msg json.RawMessage) (*Envelope, error) {
	// TODO: does the client need to send any init info in here? Currently we
	// ignore the init message body, which is empty

	// Return the user's info
	// Return the room the user starts in
	// Return the rooms that are available to the user
	rooms, err := apimodels.UserRooms(context.Background(), a.db, user.ID)
	if err != nil {
		a.logger.Error("failed to get rooms", "error", err)
		return nil, err
	}
	return &Envelope{
		Type: "init",
		Data: Init{
			User:  apimodels.NewUser(user.ID, user.Username, user.Avatar),
			Rooms: rooms,
		},
	}, nil
}
