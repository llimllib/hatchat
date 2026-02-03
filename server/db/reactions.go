package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/llimllib/hatchat/server/protocol"
)

// GetReactionsForMessages batch-loads reactions for a set of messages and returns
// them pre-aggregated by emoji. The result maps message ID to a slice of aggregated
// reactions (each with emoji, count, and user IDs).
func GetReactionsForMessages(ctx context.Context, db *DB, messageIDs []string) (map[string][]protocol.Reaction, error) {
	if len(messageIDs) == 0 {
		return make(map[string][]protocol.Reaction), nil
	}

	// Build parameterized IN clause
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := `SELECT message_id, user_id, emoji
		FROM reactions
		WHERE message_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY message_id, emoji, created_at`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Aggregate: group by (message_id, emoji) → count + user_ids
	type key struct {
		messageID string
		emoji     string
	}
	// Use ordered slices to preserve emoji order per message
	type emojiAgg struct {
		emoji   string
		userIDs []string
	}
	messageEmojis := make(map[string][]*emojiAgg) // message_id → ordered emoji aggregations
	emojiIndex := make(map[key]*emojiAgg)          // for quick lookup

	for rows.Next() {
		var messageID, userID, emoji string
		if err := rows.Scan(&messageID, &userID, &emoji); err != nil {
			return nil, err
		}
		k := key{messageID, emoji}
		agg, exists := emojiIndex[k]
		if !exists {
			agg = &emojiAgg{emoji: emoji}
			emojiIndex[k] = agg
			messageEmojis[messageID] = append(messageEmojis[messageID], agg)
		}
		agg.userIDs = append(agg.userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert to protocol.Reaction
	result := make(map[string][]protocol.Reaction, len(messageEmojis))
	for msgID, aggs := range messageEmojis {
		reactions := make([]protocol.Reaction, len(aggs))
		for i, agg := range aggs {
			reactions[i] = protocol.Reaction{
				Emoji:   agg.emoji,
				Count:   len(agg.userIDs),
				UserIDs: agg.userIDs,
			}
		}
		result[msgID] = reactions
	}

	return result, nil
}
