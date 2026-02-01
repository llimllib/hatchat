package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// ErrRoomNameTaken is returned when trying to create a room with a name that already exists
var ErrRoomNameTaken = fmt.Errorf("a channel with that name already exists")

// CreateRoomResult contains the create_room envelope and the room ID for client tracking
type CreateRoomResult struct {
	Envelope *Envelope
	RoomID   string
}

// CreateRoom handles a request from the client to create a new room.
// The user is automatically added as a member of the room.
func (a *Api) CreateRoom(user *models.User, msg json.RawMessage) (*CreateRoomResult, error) {
	var req protocol.CreateRoomRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid create_room json", "error", err)
		return nil, err
	}

	// Validate room name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("room name is required")
	}
	if len(name) > 80 {
		return nil, fmt.Errorf("room name must be 80 characters or less")
	}

	ctx := context.Background()

	// Check if a room with this name already exists
	exists, err := db.RoomExistsByName(ctx, a.db, name)
	if err != nil {
		a.logger.Error("failed to check room name", "error", err, "name", name)
		return nil, err
	}
	if exists {
		return nil, ErrRoomNameTaken
	}

	// Create the room
	room := &models.Room{
		ID:        models.GenerateRoomID(),
		Name:      name,
		IsPrivate: boolToInt(req.IsPrivate),
		IsDefault: models.FALSE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := room.Insert(ctx, a.db); err != nil {
		// Check for unique constraint violation as a fallback (race condition)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrRoomNameTaken
		}
		a.logger.Error("failed to insert room", "error", err, "room", room.ID)
		return nil, err
	}

	// Add the creator as a member
	_, err = db.AddRoomMember(ctx, a.db, user.ID, room.ID)
	if err != nil {
		a.logger.Error("failed to add room creator as member", "error", err, "user", user.ID, "room", room.ID)
		// Try to clean up the room we just created
		_ = room.Delete(ctx, a.db)
		return nil, err
	}

	// Update the user's last_room to the new room
	user.LastRoom = room.ID
	user.ModifiedAt = time.Now().Format(time.RFC3339)
	if err := user.Update(ctx, a.db); err != nil {
		a.logger.Error("failed to update user last_room", "error", err, "user", user.ID, "room", room.ID)
		// Not fatal - room is still created
	}

	a.logger.Info("room created", "room_id", room.ID, "name", room.Name, "creator", user.ID)

	return &CreateRoomResult{
		Envelope: &Envelope{
			Type: "create_room",
			Data: protocol.CreateRoomResponse{
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

// boolToInt converts a boolean to SQLite's 0/1 representation
func boolToInt(b bool) int {
	if b {
		return models.TRUE
	}
	return models.FALSE
}
