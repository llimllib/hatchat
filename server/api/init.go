package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/apimodels"
	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

type Init struct {
	User        *apimodels.User
	Rooms       []*apimodels.Room
	CurrentRoom string `json:"current_room"` // The room ID the user should start in
}

// InitResponse contains the init envelope and the current room ID for client tracking
type InitResponse struct {
	Envelope    *Envelope
	CurrentRoom string
}

func (a *Api) InitMessage(user *models.User, msg json.RawMessage) (*InitResponse, error) {
	// TODO: does the client need to send any init info in here? Currently we
	// ignore the init message body, which is empty

	ctx := context.Background()

	// Return the user's info
	// Return the room the user starts in
	// Return the rooms that are available to the user
	rooms, err := apimodels.UserRooms(ctx, a.db, user.ID)
	if err != nil {
		a.logger.Error("failed to get rooms", "error", err)
		return nil, err
	}

	// Determine the user's current room - use last_room if valid, otherwise default room
	currentRoom := user.LastRoom

	// Verify the user is still a member of their last room
	if currentRoom != "" {
		isMember, err := db.IsRoomMember(ctx, a.db, user.ID, currentRoom)
		if err != nil || !isMember {
			// Fall back to default room if last room is invalid
			currentRoom = ""
		}
	}

	// If no valid current room, use the default room
	if currentRoom == "" {
		defaultRoom, err := models.GetDefaultRoom(ctx, a.db)
		if err != nil {
			a.logger.Error("failed to get default room", "error", err)
			return nil, err
		}
		currentRoom = defaultRoom.ID
	}

	return &InitResponse{
		Envelope: &Envelope{
			Type: "init",
			Data: Init{
				User:        apimodels.NewUser(user.ID, user.Username, user.Avatar),
				Rooms:       rooms,
				CurrentRoom: currentRoom,
			},
		},
		CurrentRoom: currentRoom,
	}, nil
}
