package server

import (
	"log/slog"

	"github.com/llimllib/hatchat/server/db"
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

	// Inbound messages from the clients, scoped to a room.
	broadcast chan RoomMessage

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	logger *slog.Logger

	db *db.DB
}

func newHub(db *db.DB, logger *slog.Logger) *Hub {
	return &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		logger:     logger,
		db:         db,
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
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
		}
	}
}
