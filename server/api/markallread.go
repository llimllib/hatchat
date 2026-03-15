package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// MarkAllRead handles the "mark_all_read" message type.
// It marks all rooms the user is a member of as read at the current time.
func (a *Api) MarkAllRead(ctx context.Context, user *models.User, msg json.RawMessage) (Envelope, error) {
	readAt := time.Now().UTC().Format(time.RFC3339Nano)

	// Mark all rooms as read
	if err := a.db.MarkAllRead(ctx, user.ID, readAt); err != nil {
		a.logger.Error("failed to mark all rooms as read", "err", err, "user_id", user.ID)
		return *ErrorResponse("failed to mark all rooms as read"), nil
	}

	resp := protocol.MarkAllReadResponse{
		ReadAt: readAt,
	}

	return Envelope{Type: "mark_all_read", Data: resp}, nil
}
