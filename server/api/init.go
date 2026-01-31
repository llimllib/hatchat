package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

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
	dbRooms, err := models.UserRoomDetailsByUserID(ctx, a.db, user.ID)
	if err != nil {
		a.logger.Error("failed to get rooms", "error", err)
		return nil, err
	}

	// Convert to protocol types
	rooms := make([]*protocol.Room, len(dbRooms))
	for i, r := range dbRooms {
		rooms[i] = &protocol.Room{
			ID:        r.ID,
			Name:      r.Name,
			IsPrivate: r.IsPrivate != 0,
		}
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
			Data: protocol.InitResponse{
				User: protocol.User{
					ID:       user.ID,
					Username: user.Username,
					Avatar:   user.Avatar.String,
				},
				Rooms:       rooms,
				CurrentRoom: currentRoom,
			},
		},
		CurrentRoom: currentRoom,
	}, nil
}
