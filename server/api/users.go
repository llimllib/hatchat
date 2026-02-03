package api

import (
	"context"
	"encoding/json"

	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// ListUsers handles a request to search for users (for the DM user picker).
// The requesting user is excluded from results.
func (a *Api) ListUsers(user *models.User, msg json.RawMessage) (*Envelope, error) {
	var req protocol.ListUsersRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Build the search pattern - wrap in % for LIKE matching
	query := "%" + req.Query + "%"

	// Search for users (limit to 20 results)
	dbUsers, err := models.ListUsersByQueryExcludeUserIDLimit(ctx, a.db, query, user.ID, 20)
	if err != nil {
		a.logger.Error("failed to list users", "error", err, "query", req.Query)
		return nil, err
	}

	// Convert to protocol types
	users := make([]protocol.User, len(dbUsers))
	for i, u := range dbUsers {
		users[i] = protocol.User{
			ID:          u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Avatar:      u.Avatar,
		}
	}

	return &Envelope{
		Type: "list_users",
		Data: protocol.ListUsersResponse{
			Users: users,
		},
	}, nil
}
