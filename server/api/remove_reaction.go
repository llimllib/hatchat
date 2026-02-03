package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// RemoveReaction handles a request to remove an emoji reaction from a message.
// Users can only remove their own reactions.
func (a *Api) RemoveReaction(user *models.User, msg json.RawMessage) (*ReactionResponse, error) {
	var req protocol.RemoveReactionRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		a.logger.Error("invalid json for remove_reaction", "error", err)
		return nil, err
	}

	if req.MessageID == "" || req.Emoji == "" {
		return nil, fmt.Errorf("message_id and emoji are required")
	}

	ctx := context.Background()

	// Look up the message (to get room_id for broadcast)
	message, err := models.MessageByID(ctx, a.db, req.MessageID)
	if err != nil {
		a.logger.Error("message not found", "error", err, "message_id", req.MessageID)
		return nil, fmt.Errorf("message not found")
	}

	// Verify room membership
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, message.RoomID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the room")
	}

	// Look up and delete the reaction
	reaction, err := models.ReactionByMessageIDUserIDEmoji(ctx, a.db, req.MessageID, user.ID, req.Emoji)
	if err != nil {
		// Reaction doesn't exist - treat as idempotent success
		a.logger.Debug("reaction not found for removal", "message_id", req.MessageID, "user", user.ID, "emoji", req.Emoji)
	} else {
		if err = reaction.Delete(ctx, a.db); err != nil {
			a.logger.Error("failed to remove reaction", "error", err)
			return nil, err
		}
	}

	// Build broadcast
	broadcast := protocol.ReactionUpdated{
		MessageID: req.MessageID,
		RoomID:    message.RoomID,
		UserID:    user.ID,
		Emoji:     req.Emoji,
		Action:    "remove",
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
