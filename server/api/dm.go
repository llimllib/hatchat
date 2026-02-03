package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// CreateDMResult contains the create_dm envelope and the room ID for client tracking
type CreateDMResult struct {
	Envelope *Envelope
	RoomID   string
	Created  bool
}

// CreateDM handles a request to create (or find existing) a DM room with specified users.
// If a DM with exactly these members already exists, it returns the existing room.
func (a *Api) CreateDM(user *models.User, msg json.RawMessage) (*CreateDMResult, error) {
	var req protocol.CreateDMRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid create_dm json", "error", err)
		return nil, err
	}

	if len(req.UserIDs) == 0 {
		return nil, fmt.Errorf("at least one user_id is required")
	}

	ctx := context.Background()

	// Build the full member set including the requesting user
	memberSet := make(map[string]bool)
	memberSet[user.ID] = true
	for _, uid := range req.UserIDs {
		if uid == "" {
			continue
		}
		// Verify the user exists
		_, err := models.UserByID(ctx, a.db, uid)
		if err != nil {
			return nil, fmt.Errorf("user not found: %s", uid)
		}
		memberSet[uid] = true
	}

	// Convert to sorted slice for consistent comparison
	members := make([]string, 0, len(memberSet))
	for uid := range memberSet {
		members = append(members, uid)
	}
	sort.Strings(members)

	// Need at least 2 members for a DM
	if len(members) < 2 {
		return nil, fmt.Errorf("DM requires at least 2 members")
	}

	// Try to find an existing DM with exactly these members
	existingRoom, err := a.findExistingDM(ctx, members)
	if err != nil {
		a.logger.Error("failed to search for existing DM", "error", err)
		return nil, err
	}

	if existingRoom != nil {
		// Found existing DM - return it with members populated
		roomMembers, err := a.getRoomMembers(ctx, existingRoom.ID)
		if err != nil {
			a.logger.Error("failed to get DM members", "error", err, "room_id", existingRoom.ID)
			return nil, err
		}

		return &CreateDMResult{
			Envelope: &Envelope{
				Type: "create_dm",
				Data: protocol.CreateDMResponse{
					Room: protocol.Room{
						ID:        existingRoom.ID,
						Name:      existingRoom.Name,
						RoomType:  existingRoom.RoomType,
						IsPrivate: existingRoom.IsPrivate != 0,
						Members:   roomMembers,
					},
					Created: false,
				},
			},
			RoomID:  existingRoom.ID,
			Created: false,
		}, nil
	}

	// Create a new DM room
	room := &models.Room{
		ID:        models.GenerateRoomID(),
		Name:      "", // DMs don't have names - display name derived from members
		RoomType:  "dm",
		IsPrivate: models.TRUE,
		IsDefault: models.FALSE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := room.Insert(ctx, a.db); err != nil {
		a.logger.Error("failed to insert DM room", "error", err, "room", room.ID)
		return nil, err
	}

	// Add all members
	for _, uid := range members {
		_, err = db.AddRoomMember(ctx, a.db, uid, room.ID)
		if err != nil {
			a.logger.Error("failed to add DM member", "error", err, "user", uid, "room", room.ID)
			// Try to clean up
			_ = room.Delete(ctx, a.db)
			return nil, err
		}
	}

	// Update requesting user's last_room to the new DM
	user.LastRoom = room.ID
	user.ModifiedAt = time.Now().Format(time.RFC3339)
	if err := user.Update(ctx, a.db); err != nil {
		a.logger.Error("failed to update user last_room", "error", err, "user", user.ID, "room", room.ID)
		// Not fatal
	}

	// Get member info for response
	roomMembers, err := a.getRoomMembers(ctx, room.ID)
	if err != nil {
		a.logger.Error("failed to get new DM members", "error", err, "room_id", room.ID)
		return nil, err
	}

	a.logger.Info("DM created", "room_id", room.ID, "members", members)

	return &CreateDMResult{
		Envelope: &Envelope{
			Type: "create_dm",
			Data: protocol.CreateDMResponse{
				Room: protocol.Room{
					ID:        room.ID,
					Name:      room.Name,
					RoomType:  room.RoomType,
					IsPrivate: room.IsPrivate != 0,
					Members:   roomMembers,
				},
				Created: true,
			},
		},
		RoomID:  room.ID,
		Created: true,
	}, nil
}

// findExistingDM searches for a DM room that has exactly the specified members.
// Returns nil if no matching DM exists.
func (a *Api) findExistingDM(ctx context.Context, wantMembers []string) (*models.Room, error) {
	if len(wantMembers) == 0 {
		return nil, nil
	}

	// Get all DM rooms for the first user
	firstUser := wantMembers[0]
	userDMs, err := models.UserDMsByUserID(ctx, a.db, firstUser)
	if err != nil {
		return nil, err
	}

	// For each DM room, check if members match exactly
	for _, dm := range userDMs {
		// Get members of this DM
		members, err := models.RoomMembersByRoomID(ctx, a.db, dm.ID)
		if err != nil {
			return nil, err
		}

		// Build sorted member list
		dmMembers := make([]string, len(members))
		for i, m := range members {
			dmMembers[i] = m.ID
		}
		sort.Strings(dmMembers)

		// Compare
		if len(dmMembers) != len(wantMembers) {
			continue
		}
		match := true
		for i := range dmMembers {
			if dmMembers[i] != wantMembers[i] {
				match = false
				break
			}
		}
		if match {
			// Found matching DM - fetch full room object
			room, err := models.RoomByID(ctx, a.db, dm.ID)
			if err != nil {
				return nil, err
			}
			return room, nil
		}
	}

	return nil, nil
}
