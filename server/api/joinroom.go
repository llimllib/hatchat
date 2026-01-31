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

// JoinRoomResponse contains the join_room envelope and the room ID for client tracking
type JoinRoomResponse struct {
	Envelope *Envelope
	RoomID   string
}

// JoinRoom handles a request from the client to switch to a different room.
// It validates that the user is a member of the room and updates their last_room.
func (a *Api) JoinRoom(user *models.User, msg json.RawMessage) (*JoinRoomResponse, error) {
	var req protocol.JoinRoomRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid join_room json", "error", err)
		return nil, err
	}

	if req.RoomID == "" {
		return nil, fmt.Errorf("room_id is required")
	}

	ctx := context.Background()

	// Validate that the user is a member of the room
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, req.RoomID)
	if err != nil {
		a.logger.Error("failed to check room membership", "error", err, "user", user.ID, "room", req.RoomID)
		return nil, err
	}
	if !isMember {
		a.logger.Warn("user attempted to join room they are not a member of", "user", user.ID, "room", req.RoomID)
		return nil, fmt.Errorf("user is not a member of room %s", req.RoomID)
	}

	// Get the room details
	room, err := models.RoomByID(ctx, a.db, req.RoomID)
	if err != nil {
		a.logger.Error("failed to get room", "error", err, "room", req.RoomID)
		return nil, err
	}

	// Update the user's last_room
	user.LastRoom = req.RoomID
	user.ModifiedAt = time.Now().Format(time.RFC3339)
	if err := user.Update(ctx, a.db); err != nil {
		a.logger.Error("failed to update user last_room", "error", err, "user", user.ID, "room", req.RoomID)
		return nil, err
	}

	return &JoinRoomResponse{
		Envelope: &Envelope{
			Type: "join_room",
			Data: protocol.JoinRoomResponse{
				Room: protocol.Room{
					ID:        room.ID,
					Name:      room.Name,
					IsPrivate: room.IsPrivate != 0,
				},
			},
		},
		RoomID: room.ID,
	}, nil
}
