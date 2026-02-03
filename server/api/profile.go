package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// GetProfile handles a request to get a user's profile.
func (a *Api) GetProfile(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var req protocol.GetProfileRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, err
	}

	if req.UserID == "" {
		return ErrorResponse("user_id is required"), nil
	}

	ctx := context.Background()

	// Get the user's profile
	targetUser, err := models.UserByID(ctx, a.db, req.UserID)
	if err != nil {
		a.logger.Error("failed to get user", "error", err, "user_id", req.UserID)
		return ErrorResponse("user not found"), nil
	}

	return &Envelope{
		Type: "get_profile",
		Data: protocol.GetProfileResponse{
			User: protocol.User{
				ID:          targetUser.ID,
				Username:    targetUser.Username,
				DisplayName: targetUser.DisplayName,
				Status:      targetUser.Status,
				Avatar:      targetUser.Avatar.String,
			},
		},
	}, nil
}

// UpdateProfile handles a request to update the current user's profile.
func (a *Api) UpdateProfile(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var req protocol.UpdateProfileRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Update fields if provided
	updated := false
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
		updated = true
	}
	if req.Status != nil {
		user.Status = *req.Status
		updated = true
	}

	if updated {
		user.ModifiedAt = time.Now().Format(time.RFC3339)
		if err := user.Update(ctx, a.db); err != nil {
			a.logger.Error("failed to update user profile", "error", err, "user_id", user.ID)
			return nil, err
		}
		a.logger.Info("user profile updated", "user_id", user.ID)
	}

	return &Envelope{
		Type: "update_profile",
		Data: protocol.UpdateProfileResponse{
			User: protocol.User{
				ID:          user.ID,
				Username:    user.Username,
				DisplayName: user.DisplayName,
				Status:      user.Status,
				Avatar:      user.Avatar.String,
			},
		},
	}, nil
}
