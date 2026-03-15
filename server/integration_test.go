// # Test Infrastructure
//
// testServer wraps httptest.Server and wires up all the real components: an
// in-memory SQLite database, the Hub, and HTTP handlers. It exposes helpers to
// create authenticated users (createUser) and connect them via WebSocket
// (connectWebSocket).
//
// testClient wraps a WebSocket connection and runs a background goroutine that
// reads messages into a buffered channel. This allows tests to send messages
// and then check what was received using waitForMessage or expectNoMessage.
//
// # Test Pattern
//
// Each test follows the same pattern:
//  1. Create a testServer
//  2. Create users and connect them as WebSocket clients
//  3. Send init messages to join rooms
//  4. Send chat messages and verify delivery/isolation
//
// # Skipping
//
// These tests can be skipped with: go test ./... -short

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/llimllib/hatchat/server/api"
	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/middleware"
	"github.com/llimllib/hatchat/server/models"
)

// testServer wraps a ChatServer with test utilities
type testServer struct {
	server     *httptest.Server
	chatServer *ChatServer
	hub        *Hub
	api        *api.Api
	t          *testing.T
}

// newTestServer creates a new test server with an in-memory database
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	// Create a silent logger for tests
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create in-memory database
	testDB, err := db.NewDB("file::memory:?cache=shared", logger)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Initialize schema
	err = testDB.RunSQLFile("../schema.sql")
	if err != nil {
		t.Fatalf("Failed to run schema: %v", err)
	}

	// Create default room
	room := models.Room{
		ID:        models.GenerateRoomID(),
		Name:      "main",
		IsPrivate: models.FALSE,
		IsDefault: models.TRUE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := room.Insert(context.Background(), testDB); err != nil {
		t.Fatalf("Failed to create default room: %v", err)
	}

	chatServer := &ChatServer{
		db:         testDB,
		logger:     logger,
		sessionKey: "hatchat-session-key",
	}

	hub := newHub(testDB, logger)
	go hub.run()

	apiHandler := api.NewApi(testDB, logger, hub)

	// Create HTTP mux with all routes
	mux := http.NewServeMux()

	// Note: For testing, we serve static files from current directory
	// In production, they'd be served from ./static/
	mux.HandleFunc("/register", chatServer.register)
	mux.HandleFunc("/login", chatServer.login)
	mux.HandleFunc("/logout", chatServer.logout)
	authRequired := middleware.AuthMiddleware(testDB, logger, chatServer.sessionKey)
	mux.HandleFunc("/ws", authRequired(func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		user, err := models.UserByID(r.Context(), testDB, userID)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := &Client{
			hub:    hub,
			conn:   conn,
			send:   make(chan []byte, 256),
			logger: logger,
			user:   user,
			api:    apiHandler,
		}
		client.hub.register <- client

		go client.writePump()
		go client.readPump()
	}))

	server := httptest.NewServer(mux)

	return &testServer{
		server:     server,
		chatServer: chatServer,
		hub:        hub,
		api:        apiHandler,
		t:          t,
	}
}

func (ts *testServer) close() {
	ts.server.Close()
	_ = ts.chatServer.db.Close()
}

// testClient represents a WebSocket client for testing
type testClient struct {
	conn      *websocket.Conn
	httpClient *http.Client
	username  string
	messages  chan []byte
	done      chan struct{}
	t         *testing.T
}

// createUser registers a new user and logs them in, returning an authenticated HTTP client
func (ts *testServer) createUser(username, password string) *http.Client {
	ts.t.Helper()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	// Register user
	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)

	resp, err := client.PostForm(ts.server.URL+"/register", form)
	if err != nil {
		ts.t.Fatalf("Failed to register user %s: %v", username, err)
	}
	_ = resp.Body.Close()

	// Login
	resp, err = client.PostForm(ts.server.URL+"/login", form)
	if err != nil {
		ts.t.Fatalf("Failed to login user %s: %v", username, err)
	}
	_ = resp.Body.Close()

	return client
}

// connectWebSocket creates a WebSocket connection using the authenticated HTTP client
func (ts *testServer) connectWebSocket(httpClient *http.Client, username string) *testClient {
	ts.t.Helper()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(ts.server.URL, "http") + "/ws"

	// Get cookies from the HTTP client
	serverURL, _ := url.Parse(ts.server.URL)
	cookies := httpClient.Jar.Cookies(serverURL)

	// Create WebSocket connection with cookies
	header := http.Header{}
	for _, cookie := range cookies {
		header.Add("Cookie", cookie.String())
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		ts.t.Fatalf("Failed to connect WebSocket for %s: %v", username, err)
	}

	tc := &testClient{
		conn:       conn,
		httpClient: httpClient,
		username:   username,
		messages:   make(chan []byte, 100),
		done:       make(chan struct{}),
		t:          ts.t,
	}

	// Start reading messages in background
	go tc.readMessages()

	return tc
}

func (tc *testClient) readMessages() {
	defer close(tc.done)
	for {
		_, message, err := tc.conn.ReadMessage()
		if err != nil {
			return
		}
		tc.messages <- message
	}
}

func (tc *testClient) close() {
	_ = tc.conn.Close()
	<-tc.done // Wait for reader to finish
}

// sendInit sends an init message and returns the response.
// Skips any presence updates that may arrive before the init response.
func (tc *testClient) sendInit() (*api.Envelope, error) {
	msg := `{"type":"init","data":{}}`
	if err := tc.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return nil, err
	}

	// Loop to skip any presence updates that arrive before the init response
	for {
		select {
		case response := <-tc.messages:
			var env api.Envelope
			if err := json.Unmarshal(response, &env); err != nil {
				return nil, err
			}
			if env.Type == "presence" {
				// Skip presence updates, keep waiting for init
				continue
			}
			return &env, nil
		case <-time.After(2 * time.Second):
			return nil, fmt.Errorf("timeout waiting for init response")
		}
	}
}

// sendMessage sends a chat message
func (tc *testClient) sendMessage(body, roomID string) error {
	msg := fmt.Sprintf(`{"type":"message","data":{"body":%q,"room_id":%q}}`, body, roomID)
	return tc.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

// sendHistoryRequest sends a history request
func (tc *testClient) sendHistoryRequest(roomID string, cursor string, limit int) error {
	msg := fmt.Sprintf(`{"type":"history","data":{"room_id":%q,"cursor":%q,"limit":%d}}`, roomID, cursor, limit)
	return tc.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

// waitForMessage waits for a message with timeout
func (tc *testClient) waitForMessage(timeout time.Duration) ([]byte, error) {
	select {
	case msg := <-tc.messages:
		return msg, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for message")
	}
}

// expectNoMessage verifies no message is received within timeout
func (tc *testClient) expectNoMessage(timeout time.Duration) error {
	select {
	case msg := <-tc.messages:
		return fmt.Errorf("unexpected message received: %s", msg)
	case <-time.After(timeout):
		return nil
	}
}

// TestIntegration_BasicMessageFlow tests basic message sending and receiving
func TestIntegration_BasicMessageFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create two users
	httpClient1 := ts.createUser("alice", "password123")
	httpClient2 := ts.createUser("bob", "password456")

	// Connect both to WebSocket
	client1 := ts.connectWebSocket(httpClient1, "alice")
	defer client1.close()
	client2 := ts.connectWebSocket(httpClient2, "bob")
	defer client2.close()

	// Initialize both clients
	initResp1, err := client1.sendInit()
	if err != nil {
		t.Fatalf("Client1 init failed: %v", err)
	}
	if initResp1.Type != "init" {
		t.Errorf("Expected init response type, got %s", initResp1.Type)
	}

	initResp2, err := client2.sendInit()
	if err != nil {
		t.Fatalf("Client2 init failed: %v", err)
	}

	// Extract room ID from init response
	initData1, ok := initResp1.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Failed to parse init data")
	}
	roomID := initData1["current_room"].(string)

	// Also init client2 to the same room
	initData2, ok := initResp2.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Failed to parse init data for client2")
	}
	if initData2["current_room"].(string) != roomID {
		t.Error("Both clients should be in the same default room")
	}

	// Alice sends a message
	err = client1.sendMessage("Hello from Alice!", roomID)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Both clients should receive the message
	msg1, err := client1.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Client1 didn't receive message: %v", err)
	}

	msg2, err := client2.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Client2 didn't receive message: %v", err)
	}

	// Verify message content
	if !strings.Contains(string(msg1), "Hello from Alice!") {
		t.Errorf("Client1 received wrong message: %s", msg1)
	}
	if !strings.Contains(string(msg2), "Hello from Alice!") {
		t.Errorf("Client2 received wrong message: %s", msg2)
	}
}

// TestIntegration_RoomIsolation tests that messages don't leak between rooms
func TestIntegration_RoomIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create a second room
	room2 := models.Room{
		ID:        models.GenerateRoomID(),
		Name:      "private",
		IsPrivate: models.TRUE,
		IsDefault: models.FALSE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := room2.Insert(context.Background(), ts.chatServer.db); err != nil {
		t.Fatalf("Failed to create second room: %v", err)
	}

	// Create users
	httpClient1 := ts.createUser("alice", "password123")
	httpClient2 := ts.createUser("bob", "password456")

	// Add bob to room2
	user2, err := models.UserByUsername(context.Background(), ts.chatServer.db, "bob")
	if err != nil {
		t.Fatalf("Failed to get bob's user: %v", err)
	}
	membership := &models.RoomsMember{
		UserID: user2.ID,
		RoomID: room2.ID,
	}
	if err := membership.Insert(context.Background(), ts.chatServer.db); err != nil {
		t.Fatalf("Failed to add bob to room2: %v", err)
	}

	// Connect both users
	client1 := ts.connectWebSocket(httpClient1, "alice")
	defer client1.close()
	client2 := ts.connectWebSocket(httpClient2, "bob")
	defer client2.close()

	// Initialize both clients
	initResp1, err := client1.sendInit()
	if err != nil {
		t.Fatalf("Client1 init failed: %v", err)
	}
	initData1 := initResp1.Data.(map[string]interface{})
	room1ID := initData1["current_room"].(string)

	_, err = client2.sendInit()
	if err != nil {
		t.Fatalf("Client2 init failed: %v", err)
	}

	// Send a message to switch client2 to room2
	// This implicitly switches the client's current room
	err = client2.sendMessage("Hello in room2!", room2.ID)
	if err != nil {
		t.Fatalf("Failed to send message to room2: %v", err)
	}

	// Client2 should receive their own message
	_, err = client2.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Client2 didn't receive their own message: %v", err)
	}

	// Client1 should NOT receive the room2 message
	err = client1.expectNoMessage(500 * time.Millisecond)
	if err != nil {
		t.Errorf("SECURITY BREACH: Client1 received message from room2: %v", err)
	}

	// Now Alice sends a message in room1
	err = client1.sendMessage("Hello in room1!", room1ID)
	if err != nil {
		t.Fatalf("Failed to send message to room1: %v", err)
	}

	// Client1 should receive it
	_, err = client1.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Client1 didn't receive their own message: %v", err)
	}

	// Client2 should NOT receive the room1 message (they're in room2 now)
	err = client2.expectNoMessage(500 * time.Millisecond)
	if err != nil {
		t.Errorf("SECURITY BREACH: Client2 received message from room1 while in room2: %v", err)
	}
}

// TestIntegration_MultipleClients tests message broadcast to many clients
func TestIntegration_MultipleClients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	const numClients = 10
	clients := make([]*testClient, numClients)

	// Create and connect all clients
	for i := 0; i < numClients; i++ {
		username := fmt.Sprintf("user%d", i)
		httpClient := ts.createUser(username, "password")
		clients[i] = ts.connectWebSocket(httpClient, username)
		defer clients[i].close()
	}

	// Initialize all clients
	var roomID string
	for i, client := range clients {
		initResp, err := client.sendInit()
		if err != nil {
			t.Fatalf("Client %d init failed: %v", i, err)
		}
		initData := initResp.Data.(map[string]interface{})
		if roomID == "" {
			roomID = initData["current_room"].(string)
		}
	}

	// First client sends a message
	err := clients[0].sendMessage("Broadcast to all!", roomID)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// All clients should receive the message
	var wg sync.WaitGroup
	errors := make(chan error, numClients)

	for i, client := range clients {
		wg.Add(1)
		go func(idx int, c *testClient) {
			defer wg.Done()
			msg, err := c.waitForMessage(2 * time.Second)
			if err != nil {
				errors <- fmt.Errorf("client %d: %v", idx, err)
				return
			}
			if !strings.Contains(string(msg), "Broadcast to all!") {
				errors <- fmt.Errorf("client %d received wrong message: %s", idx, msg)
			}
		}(i, client)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestIntegration_RapidMessages tests handling of rapid message sending
func TestIntegration_RapidMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	httpClient1 := ts.createUser("alice", "password123")
	httpClient2 := ts.createUser("bob", "password456")

	client1 := ts.connectWebSocket(httpClient1, "alice")
	defer client1.close()
	client2 := ts.connectWebSocket(httpClient2, "bob")
	defer client2.close()

	// Initialize clients
	initResp, err := client1.sendInit()
	if err != nil {
		t.Fatalf("Client1 init failed: %v", err)
	}
	initData := initResp.Data.(map[string]interface{})
	roomID := initData["current_room"].(string)

	_, err = client2.sendInit()
	if err != nil {
		t.Fatalf("Client2 init failed: %v", err)
	}

	// Send many messages rapidly with small delay to avoid overwhelming buffers
	// This tests realistic rapid message sending, not DOS-level flooding
	const numMessages = 25
	for i := 0; i < numMessages; i++ {
		err := client1.sendMessage(fmt.Sprintf("Message %d", i), roomID)
		if err != nil {
			t.Fatalf("Failed to send message %d: %v", i, err)
		}
		// Small delay to allow message processing
		time.Sleep(5 * time.Millisecond)
	}

	// Collect messages from both clients with timeout
	// Both clients receive all messages (sender gets their own messages back via broadcast)
	client1Received := 0
	client2Received := 0
	timeout := time.After(5 * time.Second)

	for client1Received < numMessages || client2Received < numMessages {
		select {
		case <-client1.messages:
			client1Received++
		case <-client2.messages:
			client2Received++
		case <-timeout:
			t.Fatalf("Timeout: client1=%d/%d, client2=%d/%d messages",
				client1Received, numMessages, client2Received, numMessages)
		}
	}

	t.Logf("Client1 received %d, Client2 received %d messages", client1Received, client2Received)
}

// TestIntegration_UnauthorizedWebSocket tests that unauthenticated connections are rejected
func TestIntegration_UnauthorizedWebSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Try to connect without authentication
	wsURL := "ws" + strings.TrimPrefix(ts.server.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)

	if err == nil {
		t.Error("Expected WebSocket connection to fail without auth")
	}

	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", resp.StatusCode)
	}
}

// TestIntegration_InvalidRoomMessage tests that sending to non-member room fails
func TestIntegration_InvalidRoomMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create a room that the user won't be a member of
	privateRoom := models.Room{
		ID:        models.GenerateRoomID(),
		Name:      "secret",
		IsPrivate: models.TRUE,
		IsDefault: models.FALSE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := privateRoom.Insert(context.Background(), ts.chatServer.db); err != nil {
		t.Fatalf("Failed to create private room: %v", err)
	}

	httpClient := ts.createUser("alice", "password123")
	client := ts.connectWebSocket(httpClient, "alice")
	defer client.close()

	_, err := client.sendInit()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Try to send a message to a room the user isn't a member of
	err = client.sendMessage("Sneaky message!", privateRoom.ID)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Should receive an error response, not a success
	msg, err := client.waitForMessage(2 * time.Second)
	if err != nil {
		// Connection might be closed, which is also acceptable
		return
	}

	// If we got a message, it should be an error, not a broadcast
	var env api.Envelope
	if err := json.Unmarshal(msg, &env); err == nil {
		if env.Type != "error" {
			t.Errorf("Expected error response for non-member room message, got type: %s", env.Type)
		}
	}
}

// TestIntegration_SessionPersistence tests that sessions persist across connections
func TestIntegration_SessionPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create and login user
	httpClient := ts.createUser("alice", "password123")

	// Connect first time
	client1 := ts.connectWebSocket(httpClient, "alice")
	initResp1, err := client1.sendInit()
	if err != nil {
		t.Fatalf("First init failed: %v", err)
	}
	initData1 := initResp1.Data.(map[string]interface{})
	userInfo1 := initData1["user"].(map[string]interface{})
	userID1 := userInfo1["id"].(string)
	client1.close()

	// Connect second time with same session
	client2 := ts.connectWebSocket(httpClient, "alice")
	defer client2.close()

	initResp2, err := client2.sendInit()
	if err != nil {
		t.Fatalf("Second init failed: %v", err)
	}
	initData2 := initResp2.Data.(map[string]interface{})
	userInfo2 := initData2["user"].(map[string]interface{})
	userID2 := userInfo2["id"].(string)

	// Should be the same user
	if userID1 != userID2 {
		t.Errorf("User ID changed between connections: %s vs %s", userID1, userID2)
	}
}

// TestIntegration_MessageHistory tests fetching message history
func TestIntegration_MessageHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	httpClient := ts.createUser("alice", "password123")
	client := ts.connectWebSocket(httpClient, "alice")
	defer client.close()

	// Initialize client
	initResp, err := client.sendInit()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	initData := initResp.Data.(map[string]interface{})
	roomID := initData["current_room"].(string)

	// Send a few messages
	for i := 0; i < 5; i++ {
		err := client.sendMessage(fmt.Sprintf("Message %d", i), roomID)
		if err != nil {
			t.Fatalf("Failed to send message %d: %v", i, err)
		}
		// Wait for the broadcast of our own message
		_, err = client.waitForMessage(2 * time.Second)
		if err != nil {
			t.Fatalf("Didn't receive message %d back: %v", i, err)
		}
		// Small delay to ensure messages have distinct timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Request message history
	err = client.sendHistoryRequest(roomID, "", 50)
	if err != nil {
		t.Fatalf("Failed to send history request: %v", err)
	}

	// Wait for history response
	historyMsg, err := client.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Didn't receive history response: %v", err)
	}

	// Parse and verify response
	var env api.Envelope
	if err := json.Unmarshal(historyMsg, &env); err != nil {
		t.Fatalf("Failed to parse history response: %v", err)
	}

	if env.Type != "history" {
		t.Errorf("Expected history response, got %s", env.Type)
	}

	historyData, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Failed to parse history data")
	}

	messages, ok := historyData["messages"].([]interface{})
	if !ok {
		t.Fatalf("Failed to parse messages array")
	}

	if len(messages) != 5 {
		t.Errorf("Expected 5 messages in history, got %d", len(messages))
	}

	// Verify messages are in newest-first order (Message 4 should be first)
	if len(messages) > 0 {
		firstMsg := messages[0].(map[string]interface{})
		if !strings.Contains(firstMsg["body"].(string), "Message 4") {
			t.Errorf("Expected newest message first, got: %s", firstMsg["body"])
		}
	}
}

// TestIntegration_MessageHistoryPagination tests cursor-based pagination of message history
func TestIntegration_MessageHistoryPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	httpClient := ts.createUser("alice", "password123")
	client := ts.connectWebSocket(httpClient, "alice")
	defer client.close()

	// Initialize client
	initResp, err := client.sendInit()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	initData := initResp.Data.(map[string]interface{})
	roomID := initData["current_room"].(string)

	// Send 10 messages
	for i := 0; i < 10; i++ {
		err := client.sendMessage(fmt.Sprintf("Message %d", i), roomID)
		if err != nil {
			t.Fatalf("Failed to send message %d: %v", i, err)
		}
		_, err = client.waitForMessage(2 * time.Second)
		if err != nil {
			t.Fatalf("Didn't receive message %d back: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Request first page (3 messages)
	err = client.sendHistoryRequest(roomID, "", 3)
	if err != nil {
		t.Fatalf("Failed to send first page request: %v", err)
	}

	historyMsg, err := client.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Didn't receive first page response: %v", err)
	}

	var env api.Envelope
	if err := json.Unmarshal(historyMsg, &env); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	historyData := env.Data.(map[string]interface{})
	messages := historyData["messages"].([]interface{})
	hasMore := historyData["has_more"].(bool)
	nextCursor := historyData["next_cursor"].(string)

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages on first page, got %d", len(messages))
	}
	if !hasMore {
		t.Error("Expected has_more to be true")
	}
	if nextCursor == "" {
		t.Error("Expected non-empty next_cursor")
	}

	// Request second page using cursor
	err = client.sendHistoryRequest(roomID, nextCursor, 3)
	if err != nil {
		t.Fatalf("Failed to send second page request: %v", err)
	}

	historyMsg2, err := client.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Didn't receive second page response: %v", err)
	}

	var env2 api.Envelope
	if err := json.Unmarshal(historyMsg2, &env2); err != nil {
		t.Fatalf("Failed to parse second response: %v", err)
	}

	historyData2 := env2.Data.(map[string]interface{})
	messages2 := historyData2["messages"].([]interface{})

	if len(messages2) != 3 {
		t.Errorf("Expected 3 messages on second page, got %d", len(messages2))
	}

	// Verify no overlap between pages
	page1IDs := make(map[string]bool)
	for _, m := range messages {
		page1IDs[m.(map[string]interface{})["id"].(string)] = true
	}
	for _, m := range messages2 {
		id := m.(map[string]interface{})["id"].(string)
		if page1IDs[id] {
			t.Errorf("Message %s appeared on both pages", id)
		}
	}
}

// TestIntegration_MessageHistorySecurityNonMember tests that non-members cannot fetch history
func TestIntegration_MessageHistorySecurityNonMember(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create a private room
	privateRoom := models.Room{
		ID:        models.GenerateRoomID(),
		Name:      "secret",
		IsPrivate: models.TRUE,
		IsDefault: models.FALSE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := privateRoom.Insert(context.Background(), ts.chatServer.db); err != nil {
		t.Fatalf("Failed to create private room: %v", err)
	}

	// Create another user and add them to the private room
	httpClient2 := ts.createUser("bob", "password456")
	user2, _ := models.UserByUsername(context.Background(), ts.chatServer.db, "bob")
	membership := &models.RoomsMember{UserID: user2.ID, RoomID: privateRoom.ID}
	if err := membership.Insert(context.Background(), ts.chatServer.db); err != nil {
		t.Fatalf("Failed to add bob to room: %v", err)
	}

	// Bob sends some messages to the private room
	client2 := ts.connectWebSocket(httpClient2, "bob")
	_, _ = client2.sendInit()
	_ = client2.sendMessage("Secret message 1", privateRoom.ID)
	_, _ = client2.waitForMessage(2 * time.Second)
	_ = client2.sendMessage("Secret message 2", privateRoom.ID)
	_, _ = client2.waitForMessage(2 * time.Second)
	client2.close()

	// Alice (not a member of private room) tries to fetch history
	httpClient1 := ts.createUser("alice", "password123")
	client1 := ts.connectWebSocket(httpClient1, "alice")
	defer client1.close()

	_, err := client1.sendInit()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Try to fetch history from the private room
	err = client1.sendHistoryRequest(privateRoom.ID, "", 50)
	if err != nil {
		t.Fatalf("Failed to send history request: %v", err)
	}

	// Should receive an error response
	msg, err := client1.waitForMessage(2 * time.Second)
	if err != nil {
		// Timeout or connection close is acceptable
		return
	}

	var env api.Envelope
	if err := json.Unmarshal(msg, &env); err == nil {
		if env.Type != "error" {
			t.Errorf("SECURITY BREACH: Expected error response for non-member history request, got type: %s", env.Type)

			// If it's a history response, check that it's empty or rejected
			if env.Type == "history" {
				historyData := env.Data.(map[string]interface{})
				messages := historyData["messages"].([]interface{})
				if len(messages) > 0 {
					t.Errorf("SECURITY BREACH: Non-member received %d messages from private room", len(messages))
				}
			}
		}
	}
}

// TestIntegration_PresenceOnlineOffline tests that presence updates are broadcast
// when users connect and disconnect.
func TestIntegration_PresenceOnlineOffline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create two users - both members of default "main" room
	httpClient1 := ts.createUser("alice", "password123")
	httpClient2 := ts.createUser("bob", "password456")

	// Alice connects and inits first
	client1 := ts.connectWebSocket(httpClient1, "alice")
	defer client1.close()
	_, err := client1.sendInit()
	if err != nil {
		t.Fatalf("Alice init failed: %v", err)
	}

	// Bob connects - Alice should receive a presence update
	client2 := ts.connectWebSocket(httpClient2, "bob")
	_, err = client2.sendInit()
	if err != nil {
		t.Fatalf("Bob init failed: %v", err)
	}

	// Alice should receive a presence update for Bob coming online
	msg, err := client1.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Alice did not receive presence update for Bob connecting: %v", err)
	}

	var env api.Envelope
	if err := json.Unmarshal(msg, &env); err != nil {
		t.Fatalf("Failed to unmarshal presence update: %v", err)
	}

	if env.Type != "presence" {
		t.Fatalf("Expected presence message, got %s", env.Type)
	}

	presenceData := env.Data.(map[string]interface{})
	if online, ok := presenceData["online"].(bool); !ok || !online {
		t.Errorf("Expected online=true, got %v", presenceData["online"])
	}

	// Bob disconnects - Alice should receive an offline presence update
	client2.close()

	msg, err = client1.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Alice did not receive presence update for Bob disconnecting: %v", err)
	}

	if err := json.Unmarshal(msg, &env); err != nil {
		t.Fatalf("Failed to unmarshal offline presence update: %v", err)
	}

	if env.Type != "presence" {
		t.Fatalf("Expected presence message, got %s", env.Type)
	}

	presenceData = env.Data.(map[string]interface{})
	if online, ok := presenceData["online"].(bool); !ok || online {
		t.Errorf("Expected online=false, got %v", presenceData["online"])
	}
	if lastSeen, ok := presenceData["last_seen_at"].(string); !ok || lastSeen == "" {
		t.Errorf("Expected last_seen_at to be set, got %v", presenceData["last_seen_at"])
	}
}

// TestIntegration_PresenceInitResponse tests that the init response includes
// online user IDs.
func TestIntegration_PresenceInitResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create two users
	httpClient1 := ts.createUser("alice", "password123")
	httpClient2 := ts.createUser("bob", "password456")

	// Alice connects first
	client1 := ts.connectWebSocket(httpClient1, "alice")
	defer client1.close()
	_, err := client1.sendInit()
	if err != nil {
		t.Fatalf("Alice init failed: %v", err)
	}

	// Bob connects and inits - should see Alice in online_user_ids
	client2 := ts.connectWebSocket(httpClient2, "bob")
	defer client2.close()

	initEnv, err := client2.sendInit()
	if err != nil {
		t.Fatalf("Bob init failed: %v", err)
	}

	// Parse init response
	initData := initEnv.Data.(map[string]interface{})
	onlineIDs, ok := initData["online_user_ids"].([]interface{})
	if !ok {
		t.Fatalf("Expected online_user_ids in init response, got %v", initData["online_user_ids"])
	}

	// Should contain at least Alice's ID
	if len(onlineIDs) < 1 {
		t.Errorf("Expected at least 1 online user, got %d", len(onlineIDs))
	}

	// Drain presence update that Alice receives for Bob
	_, _ = client1.waitForMessage(2 * time.Second)
}

func TestIntegration_MarkRead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create a user
	httpClient := ts.createUser("alice", "password123")
	client := ts.connectWebSocket(httpClient, "alice")
	defer client.close()

	initEnv, err := client.sendInit()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Get the current room ID
	initData := initEnv.Data.(map[string]interface{})
	currentRoom := initData["current_room"].(string)

	// Verify init includes read_positions
	readPositions, ok := initData["read_positions"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected read_positions in init response, got %T", initData["read_positions"])
	}
	if len(readPositions) == 0 {
		t.Log("No read positions yet (expected for new user)")
	}

	// Send a message to create something to mark as read
	err = client.sendMessage("Hello world", currentRoom)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for message confirmation
	msgBytes, err := client.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	var msgEnv api.Envelope
	if err := json.Unmarshal(msgBytes, &msgEnv); err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	// Send mark_read request
	readAt := time.Now().UTC().Format(time.RFC3339Nano)
	markReadMsg := fmt.Sprintf(`{"type":"mark_read","data":{"room_id":%q,"read_at":%q}}`, currentRoom, readAt)
	if err := client.conn.WriteMessage(websocket.TextMessage, []byte(markReadMsg)); err != nil {
		t.Fatalf("Failed to send mark_read: %v", err)
	}

	// Wait for mark_read response
	respBytes, err := client.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to receive mark_read response: %v", err)
	}

	var respEnv api.Envelope
	if err := json.Unmarshal(respBytes, &respEnv); err != nil {
		t.Fatalf("Failed to parse mark_read response: %v", err)
	}

	if respEnv.Type != "mark_read" {
		t.Errorf("Expected type 'mark_read', got %q", respEnv.Type)
	}

	respData := respEnv.Data.(map[string]interface{})
	if respData["room_id"] != currentRoom {
		t.Errorf("Expected room_id %q, got %q", currentRoom, respData["room_id"])
	}
	if respData["unread_count"].(float64) != 0 {
		t.Errorf("Expected unread_count 0, got %v", respData["unread_count"])
	}
}

func TestIntegration_MarkAllRead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create a user
	httpClient := ts.createUser("alice", "password123")
	client := ts.connectWebSocket(httpClient, "alice")
	defer client.close()

	_, err := client.sendInit()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Send mark_all_read request
	markAllMsg := `{"type":"mark_all_read","data":{}}`
	if err := client.conn.WriteMessage(websocket.TextMessage, []byte(markAllMsg)); err != nil {
		t.Fatalf("Failed to send mark_all_read: %v", err)
	}

	// Wait for response
	respBytes, err := client.waitForMessage(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to receive mark_all_read response: %v", err)
	}

	var respEnv api.Envelope
	if err := json.Unmarshal(respBytes, &respEnv); err != nil {
		t.Fatalf("Failed to parse mark_all_read response: %v", err)
	}

	if respEnv.Type != "mark_all_read" {
		t.Errorf("Expected type 'mark_all_read', got %q", respEnv.Type)
	}

	respData := respEnv.Data.(map[string]interface{})
	readAt, ok := respData["read_at"].(string)
	if !ok || readAt == "" {
		t.Errorf("Expected non-empty read_at, got %v", respData["read_at"])
	}
}

// TestIntegration_Logout tests that POST /logout clears the session and redirects
func TestIntegration_Logout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create and login user
	httpClient := ts.createUser("alice", "password123")

	// Verify we can connect before logout
	client1 := ts.connectWebSocket(httpClient, "alice")
	_, err := client1.sendInit()
	if err != nil {
		t.Fatalf("Init failed before logout: %v", err)
	}
	client1.close()

	// POST to /logout
	resp, err := httpClient.Post(ts.server.URL+"/logout", "", nil)
	if err != nil {
		t.Fatalf("Logout request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should redirect to /?logged_out=1
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected 302 redirect, got %d", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if location != "/?logged_out=1" {
		t.Errorf("Expected redirect to /?logged_out=1, got %q", location)
	}

	// After logout, WebSocket connection should fail
	wsURL := "ws" + strings.TrimPrefix(ts.server.URL, "http") + "/ws"
	u, _ := url.Parse(ts.server.URL)
	jar := httpClient.Jar
	cookies := jar.Cookies(u)

	header := http.Header{}
	for _, c := range cookies {
		header.Add("Cookie", c.String())
	}

	_, resp2, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		t.Error("Expected WebSocket connection to fail after logout")
	}
	if resp2 != nil && resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 after logout, got %d", resp2.StatusCode)
	}
}

// TestIntegration_LogoutGETIgnored tests that GET /logout is ignored (POST only)
func TestIntegration_LogoutGETIgnored(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	httpClient := ts.createUser("alice", "password123")

	// GET should redirect to home without logging out
	resp, err := httpClient.Get(ts.server.URL + "/logout")
	if err != nil {
		t.Fatalf("GET logout failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected 302 redirect, got %d", resp.StatusCode)
	}

	// Session should still work — WebSocket connection should succeed
	client := ts.connectWebSocket(httpClient, "alice")
	defer client.close()

	_, err = client.sendInit()
	if err != nil {
		t.Fatalf("Init failed after GET /logout (session should still be valid): %v", err)
	}
}

// TestIntegration_SessionExpiration tests that expired sessions are rejected
func TestIntegration_SessionExpiration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ts := newTestServer(t)
	defer ts.close()

	// Create user and get a session
	httpClient := ts.createUser("alice", "password123")

	// Manually backdate the session in the database to make it expired
	ctx := context.Background()
	u, _ := url.Parse(ts.server.URL)
	cookies := httpClient.Jar.Cookies(u)
	var sessionID string
	for _, c := range cookies {
		if c.Name == ts.chatServer.sessionKey {
			sessionID = c.Value
			break
		}
	}
	if sessionID == "" {
		t.Fatal("No session cookie found")
	}

	// Set session created_at to 8 days ago (past the 7-day expiration)
	expired := time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	_, err := ts.chatServer.db.ExecContext(ctx, "UPDATE sessions SET created_at = $1 WHERE id = $2", expired, sessionID)
	if err != nil {
		t.Fatalf("Failed to backdate session: %v", err)
	}

	// WebSocket connection should now fail
	wsURL := "ws" + strings.TrimPrefix(ts.server.URL, "http") + "/ws"
	header := http.Header{}
	for _, c := range cookies {
		header.Add("Cookie", c.String())
	}

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		t.Error("Expected WebSocket connection to fail with expired session")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for expired session, got %d", resp.StatusCode)
	}
}
