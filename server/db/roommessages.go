package db

import (
	"context"

	"github.com/llimllib/hatchat/server/models"
)

// RoomMessage is a unified type for message history responses.
// It wraps the dbtpl-generated types (RoomMessagesFirstPage and RoomMessagesWithCursor)
// to provide a single interface for the API layer.
type RoomMessage struct {
	ID         string `json:"id"`
	RoomID     string `json:"room_id"`
	UserID     string `json:"user_id"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	ModifiedAt string `json:"modified_at"`
	Username   string `json:"username"`
}

// GetRoomMessages returns messages for a room with cursor-based pagination.
// Messages are ordered by created_at DESC (newest first).
// The cursor is a created_at timestamp - pass empty string for first page.
// Returns messages older than the cursor.
func GetRoomMessages(ctx context.Context, db *DB, roomID string, cursor string, limit int) ([]*RoomMessage, error) {
	if cursor == "" {
		// First page - no cursor
		results, err := models.RoomMessagesFirstPagesByRoomIDLimit(ctx, db, roomID, limit)
		if err != nil {
			return nil, err
		}
		// Convert to unified type
		messages := make([]*RoomMessage, len(results))
		for i, r := range results {
			messages[i] = &RoomMessage{
				ID:         r.ID,
				RoomID:     r.RoomID,
				UserID:     r.UserID,
				Body:       r.Body,
				CreatedAt:  r.CreatedAt,
				ModifiedAt: r.ModifiedAt,
				Username:   r.Username,
			}
		}
		return messages, nil
	}

	// Subsequent pages - use cursor
	results, err := models.RoomMessagesWithCursorsByRoomIDCursorLimit(ctx, db, roomID, cursor, limit)
	if err != nil {
		return nil, err
	}
	// Convert to unified type
	messages := make([]*RoomMessage, len(results))
	for i, r := range results {
		messages[i] = &RoomMessage{
			ID:         r.ID,
			RoomID:     r.RoomID,
			UserID:     r.UserID,
			Body:       r.Body,
			CreatedAt:  r.CreatedAt,
			ModifiedAt: r.ModifiedAt,
			Username:   r.Username,
		}
	}
	return messages, nil
}
