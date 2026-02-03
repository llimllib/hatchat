package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// ReactionResponse contains the broadcast data and room ID for routing
type ReactionResponse struct {
	RoomID  string
	Message []byte
}

// AddReaction handles a request to add an emoji reaction to a message.
// Any room member can react. Duplicate reactions are idempotent.
func (a *Api) AddReaction(user *models.User, msg json.RawMessage) (*ReactionResponse, error) {
	var req protocol.AddReactionRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid json for add_reaction", "error", err)
		return nil, err
	}

	if req.MessageID == "" || req.Emoji == "" {
		return nil, fmt.Errorf("message_id and emoji are required")
	}

	ctx := context.Background()

	// Look up the message
	message, err := models.MessageByID(ctx, a.db, req.MessageID)
	if err != nil {
		a.logger.Error("message not found", "error", err, "message_id", req.MessageID)
		return nil, fmt.Errorf("message not found")
	}

	// Check if message is deleted
	if message.DeletedAt.Valid && message.DeletedAt.String != "" {
		return nil, fmt.Errorf("cannot react to a deleted message")
	}

	// Verify room membership
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, message.RoomID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the room")
	}

	// Insert reaction (upsert to handle duplicates idempotently)
	reaction := models.Reaction{
		MessageID: req.MessageID,
		UserID:    user.ID,
		Emoji:     req.Emoji,
		CreatedAt: time.Now().Format(time.RFC3339Nano),
	}
	if err = reaction.Upsert(ctx, a.db); err != nil {
		a.logger.Error("failed to add reaction", "error", err)
		return nil, err
	}

	// Build broadcast
	broadcast := protocol.ReactionUpdated{
		MessageID: req.MessageID,
		RoomID:    message.RoomID,
		UserID:    user.ID,
		Emoji:     req.Emoji,
		Action:    "add",
	}

	msgBytes, err := json.Marshal(&Envelope{
		Type: "reaction_updated",
		Data: broadcast,
	})
	if err != nil {
		return nil, err
	}

	return &ReactionResponse{
		RoomID:  message.RoomID,
		Message: msgBytes,
	}, nil
}
