package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// RoomInfo handles a request from the client to get room details and members.
func (a *Api) RoomInfo(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var req protocol.RoomInfoRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, err
	}

	if req.RoomID == "" {
		return ErrorResponse("room_id is required"), nil
	}

	ctx := context.Background()

	// Check that user is a member of this room
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, req.RoomID)
	if err != nil {
		a.logger.Error("failed to check room membership", "error", err)
		return nil, err
	}
	if !isMember {
		return ErrorResponse("not a member of this room"), nil
	}

	// Get room info
	info, err := db.GetRoomInfo(ctx, a.db, req.RoomID)
	if err != nil {
		a.logger.Error("failed to get room info", "error", err, "room_id", req.RoomID)
		return ErrorResponse("room not found"), nil
	}

	// Convert members to protocol type
	members := make([]protocol.RoomMember, len(info.Members))
	for i, m := range info.Members {
		members[i] = protocol.RoomMember{
			ID:          m.ID,
			Username:    m.Username,
			DisplayName: m.DisplayName,
			Avatar:      m.Avatar,
		}
	}

	return &Envelope{
		Type: "room_info",
		Data: protocol.RoomInfoResponse{
			Room: protocol.Room{
				ID:        info.Room.ID,
				Name:      info.Room.Name,
				RoomType:  info.Room.RoomType,
				IsPrivate: info.Room.IsPrivate != 0,
			},
			MemberCount: info.MemberCount,
			Members:     members,
			CreatedAt:   info.Room.CreatedAt,
		},
	}, nil
}
