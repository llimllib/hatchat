package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// InitResult contains the init envelope and the current room ID for client tracking
type InitResult struct {
	Envelope    *Envelope
	CurrentRoom string
}

func (a *Api) InitMessage(user *models.User, msg json.RawMessage) (*InitResult, error) {
	// TODO: does the client need to send any init info in here? Currently we
	// ignore the init message body, which is empty

	ctx := context.Background()

	// Get user's channel rooms (not DMs)
	dbRooms, err := models.UserRoomDetailsByUserID(ctx, a.db, user.ID)
	if err != nil {
		a.logger.Error("failed to get rooms", "error", err)
		return nil, err
	}

	// Convert channel rooms to protocol types
	rooms := make([]*protocol.Room, len(dbRooms))
	for i, r := range dbRooms {
		rooms[i] = &protocol.Room{
			ID:        r.ID,
			Name:      r.Name,
			RoomType:  r.RoomType,
			IsPrivate: r.IsPrivate != 0,
		}
	}

	// Get user's DM rooms (sorted by most recent activity)
	dbDMs, err := models.UserDMsByUserID(ctx, a.db, user.ID)
	if err != nil {
		a.logger.Error("failed to get DMs", "error", err)
		return nil, err
	}

	// Convert DMs to protocol types with members populated
	dms := make([]*protocol.Room, len(dbDMs))
	for i, r := range dbDMs {
		// Get members for this DM room
		members, err := a.getRoomMembers(ctx, r.ID)
		if err != nil {
			a.logger.Error("failed to get DM members", "error", err, "room_id", r.ID)
			return nil, err
		}

		dms[i] = &protocol.Room{
			ID:        r.ID,
			Name:      r.Name,
			RoomType:  r.RoomType,
			IsPrivate: r.IsPrivate != 0,
			Members:   members,
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

	return &InitResult{
		Envelope: &Envelope{
			Type: "init",
			Data: protocol.InitResponse{
				User: protocol.User{
					ID:          user.ID,
					Username:    user.Username,
					DisplayName: user.DisplayName,
					Status:      user.Status,
					Avatar:      user.Avatar.String,
				},
				Rooms:       rooms,
				DMs:         dms,
				CurrentRoom: currentRoom,
			},
		},
		CurrentRoom: currentRoom,
	}, nil
}

// getRoomMembers returns the members of a room as protocol types
func (a *Api) getRoomMembers(ctx context.Context, roomID string) ([]protocol.RoomMember, error) {
	dbMembers, err := models.RoomMembersByRoomID(ctx, a.db, roomID)
	if err != nil {
		return nil, err
	}

	members := make([]protocol.RoomMember, len(dbMembers))
	for i, m := range dbMembers {
		members[i] = protocol.RoomMember{
			ID:          m.ID,
			Username:    m.Username,
			DisplayName: m.DisplayName,
			Avatar:      m.Avatar,
		}
	}
	return members, nil
}
