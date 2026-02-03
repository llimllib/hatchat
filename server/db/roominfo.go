package db

import (
	"context"

	"github.com/llimllib/hatchat/server/models"
)

// RoomMember represents a member of a room with user details
type RoomMember struct {
	ID          string
	Username    string
	DisplayName string
	Avatar      string
}

// RoomInfo contains room details and its members
type RoomInfo struct {
	Room        *models.Room
	Members     []RoomMember
	MemberCount int
}

// GetRoomInfo fetches a room and its members
func GetRoomInfo(ctx context.Context, db *DB, roomID string) (*RoomInfo, error) {
	// Get the room
	room, err := models.RoomByID(ctx, db, roomID)
	if err != nil {
		return nil, err
	}

	// Get the members with a join query
	const sqlstr = `SELECT u.id, u.username, u.display_name, COALESCE(u.avatar, '') as avatar 
		FROM users u
		JOIN rooms_members rm ON rm.user_id = u.id
		WHERE rm.room_id = $1
		ORDER BY u.username ASC`

	rows, err := db.QueryContext(ctx, sqlstr, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []RoomMember
	for rows.Next() {
		var m RoomMember
		if err := rows.Scan(&m.ID, &m.Username, &m.DisplayName, &m.Avatar); err != nil {
			return nil, err
		}
		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &RoomInfo{
		Room:        room,
		Members:     members,
		MemberCount: len(members),
	}, nil
}
