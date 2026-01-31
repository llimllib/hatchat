package models

import (
	"context"
)

// IsRoomMember checks if a user is a member of a specific room.
func IsRoomMember(ctx context.Context, db DB, userID, roomID string) (bool, error) {
	const sqlstr = `SELECT EXISTS(` +
		`SELECT 1 FROM rooms_members ` +
		`WHERE user_id = $1 AND room_id = $2` +
		`) AS is_member`
	logf(sqlstr, userID, roomID)
	var isMember bool
	if err := db.QueryRowContext(ctx, sqlstr, userID, roomID).Scan(&isMember); err != nil {
		return false, logerror(err)
	}
	return isMember, nil
}
