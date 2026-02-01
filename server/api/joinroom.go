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

// JoinRoomResult contains the join_room envelope and the room ID for client tracking
type JoinRoomResult struct {
	Envelope *Envelope
	RoomID   string
	// Joined is true if the user was added as a new member
	Joined bool
}

// JoinRoom handles a request from the client to switch to a different room.
// If the user is not a member of a public room, they will be added as a member.
// Private rooms require existing membership.
func (a *Api) JoinRoom(user *models.User, msg json.RawMessage) (*JoinRoomResult, error) {
	var req protocol.JoinRoomRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid join_room json", "error", err)
		return nil, err
	}

	if req.RoomID == "" {
		return nil, fmt.Errorf("room_id is required")
	}

	ctx := context.Background()

	// Get the room details first to check if it exists and if it's private
	room, err := models.RoomByID(ctx, a.db, req.RoomID)
	if err != nil {
		a.logger.Error("failed to get room", "error", err, "room", req.RoomID)
		return nil, fmt.Errorf("room not found")
	}

	// Check if user is already a member
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, req.RoomID)
	if err != nil {
		a.logger.Error("failed to check room membership", "error", err, "user", user.ID, "room", req.RoomID)
		return nil, err
	}

	joined := false
	if !isMember {
		// If the room is private, user cannot join without an invite
		if room.IsPrivate != 0 {
			a.logger.Warn("user attempted to join private room they are not a member of", "user", user.ID, "room", req.RoomID)
			return nil, fmt.Errorf("cannot join private room without an invite")
		}

		// Public room - add the user as a member
		joined, err = db.AddRoomMember(ctx, a.db, user.ID, req.RoomID)
		if err != nil {
			a.logger.Error("failed to add room member", "error", err, "user", user.ID, "room", req.RoomID)
			return nil, err
		}
		a.logger.Info("user joined public room", "user", user.ID, "room", req.RoomID)
	}

	// Update the user's last_room
	user.LastRoom = req.RoomID
	user.ModifiedAt = time.Now().Format(time.RFC3339)
	if err := user.Update(ctx, a.db); err != nil {
		a.logger.Error("failed to update user last_room", "error", err, "user", user.ID, "room", req.RoomID)
		return nil, err
	}

	return &JoinRoomResult{
		Envelope: &Envelope{
			Type: "join_room",
			Data: protocol.JoinRoomResponse{
				Room: protocol.Room{
					ID:        room.ID,
					Name:      room.Name,
					IsPrivate: room.IsPrivate != 0,
				},
				Joined: joined,
			},
		},
		RoomID: room.ID,
		Joined: joined,
	}, nil
}
