package db

import (
	"context"

	"github.com/llimllib/hatchat/server/models"
)

// ListPublicRooms returns all public (non-private) rooms.
func ListPublicRooms(ctx context.Context, db *DB) ([]*models.Room, error) {
	const sqlstr = `SELECT ` +
		`id, name, is_private, is_default, created_at ` +
		`FROM rooms ` +
		`WHERE is_private = 0 ` +
		`ORDER BY name ASC`

	rows, err := db.QueryContext(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*models.Room
	for rows.Next() {
		r := &models.Room{}
		if err := rows.Scan(&r.ID, &r.Name, &r.IsPrivate, &r.IsDefault, &r.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rooms, nil
}

// ListPublicRoomsWithMembership returns all public rooms along with whether the user is a member.
// If query is non-empty, it filters rooms by name (case-insensitive contains match).
func ListPublicRoomsWithMembership(ctx context.Context, db *DB, userID string, query string) ([]*models.Room, []bool, error) {
	var sqlstr string
	var args []any

	if query == "" {
		sqlstr = `SELECT ` +
			`r.id, r.name, r.is_private, r.is_default, r.created_at, ` +
			`CASE WHEN rm.user_id IS NOT NULL THEN 1 ELSE 0 END AS is_member ` +
			`FROM rooms r ` +
			`LEFT JOIN rooms_members rm ON r.id = rm.room_id AND rm.user_id = $1 ` +
			`WHERE r.is_private = 0 ` +
			`ORDER BY r.name ASC`
		args = []any{userID}
	} else {
		sqlstr = `SELECT ` +
			`r.id, r.name, r.is_private, r.is_default, r.created_at, ` +
			`CASE WHEN rm.user_id IS NOT NULL THEN 1 ELSE 0 END AS is_member ` +
			`FROM rooms r ` +
			`LEFT JOIN rooms_members rm ON r.id = rm.room_id AND rm.user_id = $1 ` +
			`WHERE r.is_private = 0 AND r.name LIKE '%' || $2 || '%' COLLATE NOCASE ` +
			`ORDER BY r.name ASC`
		args = []any{userID, query}
	}

	rows, err := db.QueryContext(ctx, sqlstr, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var rooms []*models.Room
	var membership []bool
	for rows.Next() {
		r := &models.Room{}
		var isMember int
		if err := rows.Scan(&r.ID, &r.Name, &r.IsPrivate, &r.IsDefault, &r.CreatedAt, &isMember); err != nil {
			return nil, nil, err
		}
		rooms = append(rooms, r)
		membership = append(membership, isMember == 1)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return rooms, membership, nil
}
