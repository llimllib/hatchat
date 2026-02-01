package db

import (
	"context"

	"github.com/llimllib/hatchat/server/models"
)

// AddRoomMember adds a user as a member of a room.
// Returns true if the user was added, false if they were already a member.
func AddRoomMember(ctx context.Context, db *DB, userID, roomID string) (bool, error) {
	// Check if already a member
	isMember, err := IsRoomMember(ctx, db, userID, roomID)
	if err != nil {
		return false, err
	}
	if isMember {
		return false, nil
	}

	// Add the membership
	member := &models.RoomsMember{
		UserID: userID,
		RoomID: roomID,
	}
	if err := member.Insert(ctx, db); err != nil {
		return false, err
	}

	return true, nil
}
