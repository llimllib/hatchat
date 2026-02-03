package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// ListRooms handles a request from the client to list public rooms.
// Returns all public rooms along with membership status for the user.
// Optionally filters by a search query.
func (a *Api) ListRooms(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var req protocol.ListRoomsRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, err
	}

	ctx := context.Background()

	rooms, membership, err := db.ListPublicRoomsWithMembership(ctx, a.db, user.ID, req.Query)
	if err != nil {
		a.logger.Error("failed to list public rooms", "error", err)
		return nil, err
	}

	// Convert to protocol types
	protoRooms := make([]*protocol.Room, len(rooms))
	for i, r := range rooms {
		protoRooms[i] = &protocol.Room{
			ID:        r.ID,
			Name:      r.Name,
			RoomType:  r.RoomType,
			IsPrivate: r.IsPrivate != 0,
		}
	}

	return &Envelope{
		Type: "list_rooms",
		Data: protocol.ListRoomsResponse{
			Rooms:    protoRooms,
			IsMember: membership,
		},
	}, nil
}
