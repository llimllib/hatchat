package db

import (
	"context"

	"github.com/llimllib/hatchat/server/models"
)

// RoomMember represents a member of a room with user details
type RoomMember struct {
	ID       string
	Username string
	Avatar   string
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
	const sqlstr = `SELECT u.id, u.username, u.avatar 
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
		var avatar *string
		if err := rows.Scan(&m.ID, &m.Username, &avatar); err != nil {
			return nil, err
		}
		if avatar != nil {
			m.Avatar = *avatar
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
