package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/llimllib/hatchat/server/api"
	"github.com/llimllib/hatchat/server/middleware"
	"github.com/llimllib/hatchat/server/models"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var newline = []byte{'\n'}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	logger *slog.Logger

	// The user who created the client; it's critical that we don't trust the
	// client to say who they are
	user *models.User

	// The room the client is currently viewing. Messages will only be sent to
	// clients viewing the same room.
	currentRoom string

	api *api.Api
}

// TODO: handle panics gracefully; rn a panic in here kills the whole app
func must(e error) {
	if e != nil {
		panic(e)
	}
}

func mustV[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	must(c.conn.SetReadDeadline(time.Now().Add(pongWait)))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("", "err", err)
			}
			break
		}

		t := time.Now()

		var msg json.RawMessage
		env := api.Envelope{Data: &msg}
		if err = json.Unmarshal(message, &env); err != nil {
			c.logger.Error("invalid json", "message", string(message))
			return
		}

		switch env.Type {
		case "init":
			res, err := c.api.InitMessage(c.user, msg)
			if err != nil {
				c.logger.Error("failed to generate init json", "error", err)
				return
			}

			// Set the client's current room for message routing
			c.currentRoom = res.CurrentRoom

			err = c.conn.WriteJSON(res.Envelope)
			if err != nil {
				c.logger.Error("failed to write init json", "error", err)
				return
			}
		case "history":
			res, err := c.api.HistoryMessage(c.user, msg)
			if err != nil {
				c.logger.Error("failed to handle history request", "error", err, "msg", msg)
				must(c.conn.WriteJSON(c.api.ErrorMessage("failed to fetch history")))
			} else {
				err = c.conn.WriteJSON(res)
				if err != nil {
					c.logger.Error("failed to write history json", "error", err)
					return
				}
			}
		case "message":
			res, err := c.api.MessageMessage(c.user, msg)
			if err != nil {
				c.logger.Error("failed to handle message", "error", err, "msg", msg)
				must(c.conn.WriteJSON(c.api.ErrorMessage("failed to handle message")))
			} else {
				// Update the client's current room and broadcast to room members only
				c.currentRoom = res.RoomID
				c.hub.broadcast <- RoomMessage{
					RoomID:  res.RoomID,
					Message: res.Message,
				}
			}
		case "join_room":
			res, err := c.api.JoinRoom(c.user, msg)
			if err != nil {
				c.logger.Error("failed to handle join_room", "error", err, "msg", msg)
				must(c.conn.WriteJSON(c.api.ErrorMessage("failed to join room")))
			} else {
				// Update the client's current room
				c.currentRoom = res.RoomID
				err = c.conn.WriteJSON(res.Envelope)
				if err != nil {
					c.logger.Error("failed to write join_room json", "error", err)
					return
				}
			}
		case "create_room":
			res, err := c.api.CreateRoom(c.user, msg)
			if err != nil {
				c.logger.Error("failed to handle create_room", "error", err, "msg", msg)
				must(c.conn.WriteJSON(c.api.ErrorMessage("failed to create room")))
			} else {
				// Update the client's current room to the new room
				c.currentRoom = res.RoomID
				err = c.conn.WriteJSON(res.Envelope)
				if err != nil {
					c.logger.Error("failed to write create_room json", "error", err)
					return
				}
			}
		case "list_rooms":
			res, err := c.api.ListRooms(c.user, msg)
			if err != nil {
				c.logger.Error("failed to handle list_rooms", "error", err, "msg", msg)
				must(c.conn.WriteJSON(c.api.ErrorMessage("failed to list rooms")))
			} else {
				err = c.conn.WriteJSON(res)
				if err != nil {
					c.logger.Error("failed to write list_rooms json", "error", err)
					return
				}
			}
		case "leave_room":
			res, err := c.api.LeaveRoom(c.user, msg)
			if err != nil {
				c.logger.Error("failed to handle leave_room", "error", err, "msg", msg)
				must(c.conn.WriteJSON(c.api.ErrorMessage("failed to leave room")))
			} else {
				err = c.conn.WriteJSON(res)
				if err != nil {
					c.logger.Error("failed to write leave_room json", "error", err)
					return
				}
			}
		case "room_info":
			res, err := c.api.RoomInfo(c.user, msg)
			if err != nil {
				c.logger.Error("failed to handle room_info", "error", err, "msg", msg)
				must(c.conn.WriteJSON(c.api.ErrorMessage("failed to get room info")))
			} else {
				err = c.conn.WriteJSON(res)
				if err != nil {
					c.logger.Error("failed to write room_info json", "error", err)
					return
				}
			}
		}

		c.logger.Debug("handled ws", "message", string(message), "duration", time.Since(t))
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			must(c.conn.SetWriteDeadline(time.Now().Add(writeWait)))
			if !ok {
				// The hub closed the channel.
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					c.logger.Debug("Unable to send close message. Is this WriteMessage call necessary?", "err", err)
				}
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			mustV(w.Write(message))

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				mustV(w.Write(newline))
				mustV(w.Write(<-c.send))
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			must(c.conn.SetWriteDeadline(time.Now().Add(writeWait)))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, api *api.Api, w http.ResponseWriter, r *http.Request) {
	userid := middleware.GetUserID(r.Context())
	user, err := models.UserByID(r.Context(), hub.db, userid)
	if err != nil {
		hub.logger.Error("Unable to find user", "userid", userid)
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		hub.logger.Error("Unable to upgrade connection", "err", err)
		return
	}

	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		logger: hub.logger,
		user:   user,
		api:    api,
	}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
