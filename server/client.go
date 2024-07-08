package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/llimllib/hatchat/server/middleware"
	"github.com/llimllib/hatchat/server/xomodels"
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

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

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
	user *xomodels.User
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

type Envelope struct {
	Type string
	Data any
}

type Init struct {
	User *xomodels.User
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
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

		var msg json.RawMessage
		env := Envelope{Data: &msg}
		if err := json.Unmarshal(message, &env); err != nil {
			c.logger.Debug("invalid json", "message", string(message))
			return
		}

		switch env.Type {
		case "init":
			// Return the user's info
			// Return the room the user starts in
			// Return the rooms that are available to the user
			c.conn.WriteJSON(Envelope{
				Type: "init",
				Data: Init{c.user},
			})
		}

		c.logger.Debug("received ws", "message", string(message))
		// c.hub.broadcast <- message
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
		c.conn.Close()
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
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	userid := middleware.GetUserID(r.Context())
	user, err := xomodels.UserByID(r.Context(), hub.db, userid)
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
	}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
