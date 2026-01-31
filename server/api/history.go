package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

// HistoryRequest is the client's request for message history
type HistoryRequest struct {
	RoomID string `json:"room_id"`
	Cursor string `json:"cursor"` // Optional - created_at timestamp for pagination
	Limit  int    `json:"limit"`  // Optional - defaults to 50
}

// HistoryMessage is a single message in the history response
type HistoryMessage struct {
	ID         string `json:"id"`
	RoomID     string `json:"room_id"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	ModifiedAt string `json:"modified_at"`
}

// HistoryResponse is the server's response with message history
type HistoryResponse struct {
	Messages   []*HistoryMessage `json:"messages"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor"` // Pass this as cursor to get older messages
}

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 100
)

// HistoryMessage fetches message history for a room with cursor-based pagination
func (a *Api) HistoryMessage(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var req HistoryRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid json for history request", "error", err)
		return nil, err
	}

	// Validate room ID
	if req.RoomID == "" {
		a.logger.Error("missing room_id in history request")
		return nil, fmt.Errorf("room_id is required")
	}

	// Set default limit
	limit := req.Limit
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}

	ctx := context.Background()

	// Validate that the user is a member of the room
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, req.RoomID)
	if err != nil {
		a.logger.Error("failed to check room membership", "error", err, "user", user.ID, "room", req.RoomID)
		return nil, err
	}
	if !isMember {
		a.logger.Warn("user attempted to fetch history for room they are not a member of", "user", user.ID, "room", req.RoomID)
		return nil, fmt.Errorf("user is not a member of room %s", req.RoomID)
	}

	// Fetch one extra message to determine if there are more
	messages, err := db.GetRoomMessages(ctx, a.db, req.RoomID, req.Cursor, limit+1)
	if err != nil {
		a.logger.Error("failed to get room messages", "error", err, "room", req.RoomID)
		return nil, err
	}

	// Determine if there are more messages
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit] // Trim to requested limit
	}

	// Convert to response format
	historyMessages := make([]*HistoryMessage, len(messages))
	for i, m := range messages {
		historyMessages[i] = &HistoryMessage{
			ID:         m.ID,
			RoomID:     m.RoomID,
			UserID:     m.UserID,
			Username:   m.Username,
			Body:       m.Body,
			CreatedAt:  m.CreatedAt,
			ModifiedAt: m.ModifiedAt,
		}
	}

	// Calculate next cursor (oldest message's created_at)
	var nextCursor string
	if len(messages) > 0 && hasMore {
		nextCursor = messages[len(messages)-1].CreatedAt
	}

	return &Envelope{
		Type: "history",
		Data: HistoryResponse{
			Messages:   historyMessages,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		},
	}, nil
}
