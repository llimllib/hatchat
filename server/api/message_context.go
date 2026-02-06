package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// GetMessageContext handles a request to get a message and its room for permalink navigation
func (a *Api) GetMessageContext(user *models.User, msg json.RawMessage) (Envelope, error) {
	var req protocol.GetMessageContextRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return *ErrorResponse("invalid get_message_context request"), nil
	}

	if req.MessageID == "" {
		return *ErrorResponse("message_id is required"), nil
	}

	ctx := context.Background()

	// Fetch the message
	message, err := a.getMessageByID(ctx, req.MessageID)
	if err != nil {
		a.logger.Error("failed to get message", "error", err, "message_id", req.MessageID)
		return *ErrorResponse("message not found"), nil
	}

	// Check if user has access to the room
	isMember, err := db.IsRoomMember(ctx, a.db, user.ID, message.RoomID)
	if err != nil {
		a.logger.Error("failed to check room membership", "error", err)
		return *ErrorResponse("failed to check access"), nil
	}
	if !isMember {
		return *ErrorResponse("you don't have access to this message"), nil
	}

	// Convert to protocol.Message
	protoMessage := protocol.Message{
		ID:         message.ID,
		RoomID:     message.RoomID,
		UserID:     message.UserID,
		Username:   message.Username,
		Body:       message.Body,
		CreatedAt:  message.CreatedAt,
		ModifiedAt: message.ModifiedAt,
		DeletedAt:  message.DeletedAt,
	}

	// Handle deleted messages
	if message.DeletedAt != "" {
		protoMessage.Body = ""
	}

	return Envelope{
		Type: "get_message_context",
		Data: protocol.GetMessageContextResponse{
			Message: protoMessage,
			RoomID:  message.RoomID,
		},
	}, nil
}

// MessageWithUsername is a message with the author's username
type MessageWithUsername struct {
	ID         string
	RoomID     string
	UserID     string
	Username   string
	Body       string
	CreatedAt  string
	ModifiedAt string
	DeletedAt  string
}

// getMessageByID fetches a single message by ID with the author's username
func (a *Api) getMessageByID(ctx context.Context, messageID string) (*MessageWithUsername, error) {
	query := `
		SELECT m.id, m.room_id, m.user_id, u.username, m.body, m.created_at, m.modified_at, COALESCE(m.deleted_at, '') as deleted_at
		FROM messages m
		JOIN users u ON m.user_id = u.id
		WHERE m.id = $1
	`

	var msg MessageWithUsername
	err := a.db.QueryRowContext(ctx, query, messageID).Scan(
		&msg.ID,
		&msg.RoomID,
		&msg.UserID,
		&msg.Username,
		&msg.Body,
		&msg.CreatedAt,
		&msg.ModifiedAt,
		&msg.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}
