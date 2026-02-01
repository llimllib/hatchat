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
func (a *Api) ListRooms(user *models.User, msg json.RawMessage) (*Envelope, error) {
	// msg is currently unused but included for consistency with other handlers
	_ = msg

	ctx := context.Background()

	rooms, membership, err := db.ListPublicRoomsWithMembership(ctx, a.db, user.ID)
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
