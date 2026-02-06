package api

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// Search handles a search request for messages
func (a *Api) Search(user *models.User, msg json.RawMessage) (Envelope, error) {
	var req protocol.SearchRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return *ErrorResponse("invalid search request"), nil
	}

	// Validate query
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return *ErrorResponse("search query cannot be empty"), nil
	}

	ctx := context.Background()

	// Perform search
	results, nextCursor, err := a.db.SearchMessages(
		ctx,
		user.ID,
		query,
		req.RoomID,
		req.UserID,
		req.Cursor,
		req.Limit,
	)
	if err != nil {
		a.logger.Error("search failed", "error", err, "user_id", user.ID, "query", query)
		return *ErrorResponse("search failed"), nil
	}

	// Return empty array instead of nil for consistency
	if results == nil {
		results = []protocol.SearchResult{}
	}

	return Envelope{
		Type: "search",
		Data: protocol.SearchResponse{
			Results:    results,
			NextCursor: nextCursor,
			Total:      0, // We don't compute total for now (expensive with FTS5)
		},
	}, nil
}
