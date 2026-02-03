package api

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

var ErrCannotLeaveDefaultRoom = errors.New("cannot leave the default room")
var ErrCannotLeave1on1DM = errors.New("cannot leave a 1:1 direct message")

// LeaveRoom handles a request from the client to leave a room.
// Users cannot leave the default room or 1:1 DMs.
func (a *Api) LeaveRoom(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var req protocol.LeaveRoomRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, err
	}

	if req.RoomID == "" {
		return ErrorResponse("room_id is required"), nil
	}

	ctx := context.Background()

	// Check if this is the default room
	room, err := models.RoomByID(ctx, a.db, req.RoomID)
	if err != nil {
		return ErrorResponse("room not found"), nil
	}

	if room.IsDefault != 0 {
		return ErrorResponse("cannot leave the default room"), nil
	}

	// Check if this is a 1:1 DM (cannot leave those)
	if room.RoomType == "dm" {
		memberCount, err := models.RoomMemberCountByRoomID(ctx, a.db, req.RoomID)
		if err != nil {
			a.logger.Error("failed to get DM member count", "error", err, "room_id", req.RoomID)
			return nil, err
		}
		// Count comes back as a string from SQLite
		if memberCount.Count == "1" || memberCount.Count == "2" {
			return ErrorResponse("cannot leave a 1:1 direct message"), nil
		}
	}

	// Try to leave the room
	left, err := db.LeaveRoom(ctx, a.db, user.ID, req.RoomID)
	if err != nil {
		a.logger.Error("failed to leave room", "error", err, "room_id", req.RoomID)
		return nil, err
	}

	if !left {
		return ErrorResponse("not a member of this room"), nil
	}

	return &Envelope{
		Type: "leave_room",
		Data: protocol.LeaveRoomResponse(req),
	}, nil
}
