package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/protocol"
)

// RoomMessage wraps a message with its target room ID for routing
type RoomMessage struct {
	RoomID  string
	Message []byte
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Track which users have active connections (user ID → set of clients).
	// Used for presence tracking.
	userClients map[string]map[*Client]bool

	// Inbound messages from the clients, scoped to a room.
	broadcast chan RoomMessage

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// queryOnline receives channels that will be sent the list of online user IDs.
	// This allows safe cross-goroutine querying of online status.
	queryOnline chan chan []string

	logger *slog.Logger

	db *db.DB
}

func newHub(db *db.DB, logger *slog.Logger) *Hub {
	return &Hub{
		broadcast:   make(chan RoomMessage),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		userClients: make(map[string]map[*Client]bool),
		queryOnline: make(chan chan []string),
		logger:      logger,
		db:          db,
	}
}

// OnlineUserIDs returns a slice of all currently online user IDs.
// Safe to call from any goroutine.
func (h *Hub) OnlineUserIDs() []string {
	ch := make(chan []string, 1)
	h.queryOnline <- ch
	return <-ch
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.handleClientConnected(client)
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.handleClientDisconnected(client)
			}
		case roomMsg := <-h.broadcast:
			// Only send to clients viewing the same room
			for client := range h.clients {
				if client.currentRoom != roomMsg.RoomID {
					continue
				}
				select {
				case client.send <- roomMsg.Message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		case ch := <-h.queryOnline:
			ids := make([]string, 0, len(h.userClients))
			for uid, clients := range h.userClients {
				if len(clients) > 0 {
					ids = append(ids, uid)
				}
			}
			ch <- ids
		}
	}
}

// handleClientConnected tracks user presence when a new client connects.
// Called from the hub's run goroutine.
func (h *Hub) handleClientConnected(client *Client) {
	if client.user == nil {
		return
	}
	userID := client.user.ID

	// Initialize the client set for this user if needed
	if h.userClients[userID] == nil {
		h.userClients[userID] = make(map[*Client]bool)
	}

	wasOnline := len(h.userClients[userID]) > 0
	h.userClients[userID][client] = true

	// If user just came online (first connection), broadcast presence
	if !wasOnline {
		h.logger.Debug("user came online", "user_id", userID)
		h.broadcastPresence(userID, true)
	}
}

// handleClientDisconnected tracks user presence when a client disconnects.
// Called from the hub's run goroutine.
func (h *Hub) handleClientDisconnected(client *Client) {
	if client.user == nil {
		return
	}
	userID := client.user.ID

	// Remove client from user's set
	if clients, ok := h.userClients[userID]; ok {
		delete(clients, client)

		// If user has no more connections, they went offline
		if len(clients) == 0 {
			delete(h.userClients, userID)
			h.logger.Debug("user went offline", "user_id", userID)

			// Update last_seen_at in database
			now := time.Now().Format(time.RFC3339)
			ctx := context.Background()
			_, err := h.db.ExecContext(ctx,
				"UPDATE users SET last_seen_at = ? WHERE id = ?",
				now, userID)
			if err != nil {
				h.logger.Error("failed to update last_seen_at", "user_id", userID, "error", err)
			}

			h.broadcastPresence(userID, false)
		}
	}
}

// broadcastPresence sends a presence update to all users who share a room
// with the given user. Called from the hub's run goroutine.
func (h *Hub) broadcastPresence(userID string, online bool) {
	update := protocol.PresenceUpdate{
		UserID: userID,
		Online: online,
	}

	// If going offline, include last_seen_at
	if !online {
		update.LastSeenAt = time.Now().Format(time.RFC3339)
	}

	envelope := protocol.Envelope{
		Type: "presence",
		Data: update,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		h.logger.Error("failed to marshal presence update", "error", err)
		return
	}

	// Get all rooms the user is a member of
	ctx := context.Background()
	roomIDs, err := h.getUserRoomIDs(ctx, userID)
	if err != nil {
		h.logger.Error("failed to get user rooms for presence broadcast", "user_id", userID, "error", err)
		return
	}

	// Build set of user IDs who share rooms with this user
	peerIDs := make(map[string]bool)
	for _, roomID := range roomIDs {
		memberIDs, err := h.getRoomMemberIDs(ctx, roomID)
		if err != nil {
			h.logger.Error("failed to get room members for presence", "room_id", roomID, "error", err)
			continue
		}
		for _, memberID := range memberIDs {
			if memberID != userID {
				peerIDs[memberID] = true
			}
		}
	}

	// Send to all connected clients of those users
	for client := range h.clients {
		if client.user != nil && peerIDs[client.user.ID] {
			select {
			case client.send <- data:
			default:
				// Client buffer full, skip
			}
		}
	}
}

// getUserRoomIDs returns all room IDs a user is a member of.
func (h *Hub) getUserRoomIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := h.db.QueryContext(ctx,
		"SELECT room_id FROM rooms_members WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roomIDs []string
	for rows.Next() {
		var roomID string
		if err := rows.Scan(&roomID); err != nil {
			return nil, err
		}
		roomIDs = append(roomIDs, roomID)
	}
	return roomIDs, rows.Err()
}

// getRoomMemberIDs returns all user IDs in a room.
func (h *Hub) getRoomMemberIDs(ctx context.Context, roomID string) ([]string, error) {
	rows, err := h.db.QueryContext(ctx,
		"SELECT user_id FROM rooms_members WHERE room_id = ?", roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}
