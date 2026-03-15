package db

import (
	"context"

	"github.com/llimllib/hatchat/server/models"
)

// SetReadPosition updates the user's last read position in a room.
// Uses UPSERT to create or update the read position.
func (d *DB) SetReadPosition(ctx context.Context, userID, roomID, readAt string) error {
	rp := &models.ReadPosition{
		UserID:     userID,
		RoomID:     roomID,
		LastReadAt: readAt,
	}
	return rp.Upsert(ctx, d)
}

// GetUnreadCounts returns a map of room ID to unread message count for all rooms
// the user is a member of. A room has unread messages if:
// - The user has no read position (never read) and the room has any messages
// - The user's last_read_at is older than some messages in the room
func (d *DB) GetUnreadCounts(ctx context.Context, userID string) (map[string]int, error) {
	const sqlstr = `
		SELECT rm.room_id, COUNT(m.id) as unread_count
		FROM rooms_members rm
		LEFT JOIN read_positions rp ON rm.room_id = rp.room_id AND rm.user_id = rp.user_id
		LEFT JOIN messages m ON m.room_id = rm.room_id AND m.deleted_at IS NULL
			AND (rp.last_read_at IS NULL OR m.created_at > rp.last_read_at)
		WHERE rm.user_id = $1
		GROUP BY rm.room_id
	`

	rows, err := d.QueryContext(ctx, sqlstr, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var roomID string
		var count int
		if err := rows.Scan(&roomID, &count); err != nil {
			return nil, err
		}
		counts[roomID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return counts, nil
}

// GetUnreadCount returns the unread message count for a specific room.
func (d *DB) GetUnreadCount(ctx context.Context, userID, roomID string) (int, error) {
	const sqlstr = `
		SELECT COUNT(m.id)
		FROM messages m
		LEFT JOIN read_positions rp ON m.room_id = rp.room_id AND rp.user_id = $1
		WHERE m.room_id = $2 AND m.deleted_at IS NULL
			AND (rp.last_read_at IS NULL OR m.created_at > rp.last_read_at)
	`

	var count int
	err := d.QueryRowContext(ctx, sqlstr, userID, roomID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetReadPosition returns the user's read position in a room, or nil if none exists.
func (d *DB) GetReadPosition(ctx context.Context, userID, roomID string) (*models.ReadPosition, error) {
	rp, err := models.ReadPositionByUserIDRoomID(ctx, d, userID, roomID)
	if err != nil {
		// sql.ErrNoRows is not an error - just means no read position exists
		return nil, nil
	}
	return rp, nil
}

// GetReadPositions returns a map of room ID to last_read_at timestamp for all rooms
// the user is a member of. Rooms with no read position have empty string values.
func (d *DB) GetReadPositions(ctx context.Context, userID string) (map[string]string, error) {
	const sqlstr = `
		SELECT rm.room_id, COALESCE(rp.last_read_at, '') as last_read_at
		FROM rooms_members rm
		LEFT JOIN read_positions rp ON rm.room_id = rp.room_id AND rm.user_id = rp.user_id
		WHERE rm.user_id = $1
	`

	rows, err := d.QueryContext(ctx, sqlstr, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := make(map[string]string)
	for rows.Next() {
		var roomID, lastReadAt string
		if err := rows.Scan(&roomID, &lastReadAt); err != nil {
			return nil, err
		}
		positions[roomID] = lastReadAt
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return positions, nil
}

// MarkAllRead marks all rooms the user is a member of as read at the given timestamp.
func (d *DB) MarkAllRead(ctx context.Context, userID, readAt string) error {
	const sqlstr = `
		INSERT INTO read_positions (user_id, room_id, last_read_at)
		SELECT $1, rm.room_id, $2
		FROM rooms_members rm
		WHERE rm.user_id = $1
		ON CONFLICT (user_id, room_id) DO UPDATE SET last_read_at = excluded.last_read_at
	`

	_, err := d.ExecContext(ctx, sqlstr, userID, readAt)
	return err
}
