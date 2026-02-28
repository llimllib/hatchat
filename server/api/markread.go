package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// MarkRead handles the "mark_read" message type.
// It updates the user's last read position in a room and returns the new unread count.
func (a *Api) MarkRead(ctx context.Context, user *models.User, msg json.RawMessage) (Envelope, error) {
	var req protocol.MarkReadRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return *ErrorResponse("invalid mark_read request"), nil
	}

	// Validate required fields
	if req.RoomID == "" {
		return *ErrorResponse("room_id is required"), nil
	}
	if req.ReadAt == "" {
		return *ErrorResponse("read_at is required"), nil
	}

	// Check if user is a member of the room
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, req.RoomID)
	if err != nil {
		a.logger.Error("failed to check room membership", "err", err, "user_id", user.ID, "room_id", req.RoomID)
		return *ErrorResponse("failed to check room membership"), nil
	}
	if !isMember {
		return *ErrorResponse("you are not a member of this room"), nil
	}

	// Update read position
	if err := a.db.SetReadPosition(ctx, user.ID, req.RoomID, req.ReadAt); err != nil {
		a.logger.Error("failed to set read position", "err", err, "user_id", user.ID, "room_id", req.RoomID)
		return *ErrorResponse("failed to mark room as read"), nil
	}

	// Get updated unread count (should be 0 if marked to the latest message)
	unreadCount, err := a.db.GetUnreadCount(ctx, user.ID, req.RoomID)
	if err != nil {
		a.logger.Error("failed to get unread count", "err", err, "user_id", user.ID, "room_id", req.RoomID)
		// Don't fail the request, just return 0
		unreadCount = 0
	}

	resp := protocol.MarkReadResponse{
		RoomID:      req.RoomID,
		ReadAt:      req.ReadAt,
		UnreadCount: unreadCount,
	}

	return Envelope{Type: "mark_read", Data: resp}, nil
}
