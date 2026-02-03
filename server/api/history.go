package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 100
)

// HistoryMessage fetches message history for a room with cursor-based pagination
func (a *Api) HistoryMessage(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var req protocol.HistoryRequest
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

	// Collect message IDs for batch-loading reactions
	messageIDs := make([]string, len(messages))
	for i, m := range messages {
		messageIDs[i] = m.ID
	}

	// Batch-load reactions for all messages
	reactionsMap, err := db.GetReactionsForMessages(ctx, a.db, messageIDs)
	if err != nil {
		a.logger.Error("failed to get reactions", "error", err)
		// Don't fail the whole request — just continue without reactions
		reactionsMap = make(map[string][]protocol.Reaction)
	}

	// Convert to protocol.Message format
	historyMessages := make([]*protocol.Message, len(messages))
	for i, m := range messages {
		historyMessages[i] = &protocol.Message{
			ID:         m.ID,
			RoomID:     m.RoomID,
			UserID:     m.UserID,
			Username:   m.Username,
			Body:       m.Body,
			CreatedAt:  m.CreatedAt,
			ModifiedAt: m.ModifiedAt,
			DeletedAt:  m.DeletedAt,
			Reactions:  reactionsMap[m.ID],
		}
	}

	// Calculate next cursor (oldest message's created_at)
	var nextCursor string
	if len(messages) > 0 && hasMore {
		nextCursor = messages[len(messages)-1].CreatedAt
	}

	return &Envelope{
		Type: "history",
		Data: protocol.HistoryResponse{
			Messages:   historyMessages,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		},
	}, nil
}
