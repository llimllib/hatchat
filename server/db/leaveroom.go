package db

import (
	"context"

	"github.com/llimllib/hatchat/server/models"
)

// LeaveRoom removes a user from a room's membership.
// Returns true if the user was removed, false if they weren't a member.
// This will not remove a user from the default room.
func LeaveRoom(ctx context.Context, db *DB, userID, roomID string) (bool, error) {
	// Check if they are a member
	member, err := models.RoomsMemberByUserIDRoomID(ctx, db, userID, roomID)
	if err != nil {
		// Not a member
		return false, nil
	}

	// Check if this is the default room (users cannot leave the default room)
	room, err := models.RoomByID(ctx, db, roomID)
	if err != nil {
		return false, err
	}
	if room.IsDefault != 0 {
		return false, nil
	}

	// Delete the membership
	if err := member.Delete(ctx, db); err != nil {
		return false, err
	}

	return true, nil
}
