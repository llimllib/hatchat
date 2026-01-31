package server

import (
	"sync"
	"testing"
	"time"
)

// TestHub_RoomScopedBroadcast tests that messages are only sent to clients in the same room
// SECURITY: This is the critical test for room isolation
func TestHub_RoomScopedBroadcast(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	// Create clients in different rooms
	clientRoom1a := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 256),
	}
	clientRoom1b := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 256),
	}
	clientRoom2 := &Client{
		hub:         hub,
		currentRoom: "roo_room2345678",
		send:        make(chan []byte, 256),
	}

	// Register all clients
	hub.clients[clientRoom1a] = true
	hub.clients[clientRoom1b] = true
	hub.clients[clientRoom2] = true

	// Start the hub in a goroutine
	go hub.run()

	// Send a message to room1
	testMessage := []byte(`{"type":"message","data":{"body":"Hello room1"}}`)
	hub.broadcast <- RoomMessage{
		RoomID:  "roo_room1234567",
		Message: testMessage,
	}

	// Wait a bit for message processing
	time.Sleep(50 * time.Millisecond)

	// Check that room1 clients received the message
	select {
	case msg := <-clientRoom1a.send:
		if string(msg) != string(testMessage) {
			t.Errorf("Room1 client A received wrong message: got %s, want %s", msg, testMessage)
		}
	default:
		t.Error("Room1 client A did not receive the message")
	}

	select {
	case msg := <-clientRoom1b.send:
		if string(msg) != string(testMessage) {
			t.Errorf("Room1 client B received wrong message: got %s, want %s", msg, testMessage)
		}
	default:
		t.Error("Room1 client B did not receive the message")
	}

	// Check that room2 client did NOT receive the message
	select {
	case msg := <-clientRoom2.send:
		t.Errorf("SECURITY BREACH: Room2 client received message meant for room1: %s", msg)
	default:
		// This is expected - room2 client should not receive the message
	}
}

// TestHub_MessageIsolationBetweenRooms tests message isolation with multiple messages
// SECURITY: Ensures complete isolation even with interleaved messages
func TestHub_MessageIsolationBetweenRooms(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	// Create clients in different rooms
	room1Client := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 256),
	}
	room2Client := &Client{
		hub:         hub,
		currentRoom: "roo_room2345678",
		send:        make(chan []byte, 256),
	}
	room3Client := &Client{
		hub:         hub,
		currentRoom: "roo_room3456789",
		send:        make(chan []byte, 256),
	}

	// Register all clients
	hub.clients[room1Client] = true
	hub.clients[room2Client] = true
	hub.clients[room3Client] = true

	// Start the hub
	go hub.run()

	// Send messages to each room
	room1Msg := []byte(`{"type":"message","data":{"body":"Room 1 message"}}`)
	room2Msg := []byte(`{"type":"message","data":{"body":"Room 2 message"}}`)
	room3Msg := []byte(`{"type":"message","data":{"body":"Room 3 message"}}`)

	hub.broadcast <- RoomMessage{RoomID: "roo_room1234567", Message: room1Msg}
	hub.broadcast <- RoomMessage{RoomID: "roo_room2345678", Message: room2Msg}
	hub.broadcast <- RoomMessage{RoomID: "roo_room3456789", Message: room3Msg}

	time.Sleep(100 * time.Millisecond)

	// Verify each client only received their room's message
	// Room 1
	room1Count := len(room1Client.send)
	if room1Count != 1 {
		t.Errorf("Room1 client received %d messages, expected 1", room1Count)
	}
	if room1Count > 0 {
		msg := <-room1Client.send
		if string(msg) != string(room1Msg) {
			t.Errorf("Room1 received wrong message: got %s", msg)
		}
	}

	// Room 2
	room2Count := len(room2Client.send)
	if room2Count != 1 {
		t.Errorf("Room2 client received %d messages, expected 1", room2Count)
	}
	if room2Count > 0 {
		msg := <-room2Client.send
		if string(msg) != string(room2Msg) {
			t.Errorf("Room2 received wrong message: got %s", msg)
		}
	}

	// Room 3
	room3Count := len(room3Client.send)
	if room3Count != 1 {
		t.Errorf("Room3 client received %d messages, expected 1", room3Count)
	}
	if room3Count > 0 {
		msg := <-room3Client.send
		if string(msg) != string(room3Msg) {
			t.Errorf("Room3 received wrong message: got %s", msg)
		}
	}
}

// TestHub_ClientWithNoRoom tests that clients without a room don't receive any messages
func TestHub_ClientWithNoRoom(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	// Create a client with no room assigned
	clientNoRoom := &Client{
		hub:         hub,
		currentRoom: "", // No room assigned
		send:        make(chan []byte, 256),
	}

	// Create a client in a room
	clientWithRoom := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 256),
	}

	hub.clients[clientNoRoom] = true
	hub.clients[clientWithRoom] = true

	go hub.run()

	// Send a message to room1
	testMsg := []byte(`{"type":"message","data":{"body":"Test"}}`)
	hub.broadcast <- RoomMessage{RoomID: "roo_room1234567", Message: testMsg}

	time.Sleep(50 * time.Millisecond)

	// Client with no room should not receive message
	select {
	case msg := <-clientNoRoom.send:
		t.Errorf("Client with no room received message: %s", msg)
	default:
		// Expected
	}

	// Client with room should receive message
	select {
	case msg := <-clientWithRoom.send:
		if string(msg) != string(testMsg) {
			t.Errorf("Client with room received wrong message: got %s, want %s", msg, testMsg)
		}
	default:
		t.Error("Client with room did not receive message")
	}
}

// TestHub_MessageToNonExistentRoom tests that messages to non-existent rooms don't cause issues
func TestHub_MessageToNonExistentRoom(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	client := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 256),
	}

	hub.clients[client] = true

	go hub.run()

	// Send a message to a room that has no clients
	testMsg := []byte(`{"type":"message","data":{"body":"Ghost message"}}`)
	hub.broadcast <- RoomMessage{RoomID: "roo_nonexistent1", Message: testMsg}

	time.Sleep(50 * time.Millisecond)

	// No client should receive this message
	select {
	case msg := <-client.send:
		t.Errorf("Client received message for non-existent room: %s", msg)
	default:
		// Expected - no message should be received
	}
}

// TestHub_ClientRegistrationAndUnregistration tests proper client lifecycle management
func TestHub_ClientRegistrationAndUnregistration(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	go hub.run()

	client := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 256),
	}

	// Register client
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Send a message
	testMsg := []byte(`{"type":"message","data":{"body":"Test"}}`)
	hub.broadcast <- RoomMessage{RoomID: "roo_room1234567", Message: testMsg}
	time.Sleep(50 * time.Millisecond)

	// Should receive message
	select {
	case msg := <-client.send:
		if string(msg) != string(testMsg) {
			t.Errorf("Got wrong message: %s", msg)
		}
	default:
		t.Error("Did not receive message after registration")
	}

	// Unregister client
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	// Client's send channel should be closed
	_, ok := <-client.send
	if ok {
		t.Error("Client's send channel was not closed after unregistration")
	}
}

// TestHub_MultipleClientsInSameRoom tests that all clients in a room receive messages
func TestHub_MultipleClientsInSameRoom(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	// Create multiple clients in the same room
	const numClients = 10
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			hub:         hub,
			currentRoom: "roo_room1234567",
			send:        make(chan []byte, 256),
		}
		hub.clients[clients[i]] = true
	}

	go hub.run()

	// Send a message
	testMsg := []byte(`{"type":"message","data":{"body":"Broadcast test"}}`)
	hub.broadcast <- RoomMessage{RoomID: "roo_room1234567", Message: testMsg}

	time.Sleep(100 * time.Millisecond)

	// All clients should receive the message
	for i, client := range clients {
		select {
		case msg := <-client.send:
			if string(msg) != string(testMsg) {
				t.Errorf("Client %d received wrong message: got %s", i, msg)
			}
		default:
			t.Errorf("Client %d did not receive the message", i)
		}
	}
}

// TestHub_RapidMessages tests handling of rapid successive messages
func TestHub_RapidMessages(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	client := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 256),
	}
	hub.clients[client] = true

	go hub.run()

	// Send many messages rapidly
	const numMessages = 100
	for i := 0; i < numMessages; i++ {
		msg := []byte(`{"type":"message","data":{"body":"Message"}}`)
		hub.broadcast <- RoomMessage{RoomID: "roo_room1234567", Message: msg}
	}

	time.Sleep(200 * time.Millisecond)

	// Count received messages
	received := 0
	for {
		select {
		case <-client.send:
			received++
		default:
			goto done
		}
	}
done:

	if received != numMessages {
		t.Errorf("Expected %d messages, received %d", numMessages, received)
	}
}

// TestHub_RoomSwitching tests that a client switching rooms receives the correct messages
func TestHub_RoomSwitching(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	client := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 256),
	}
	hub.clients[client] = true

	go hub.run()

	// Send message to room1 - client should receive it
	room1Msg := []byte(`{"type":"message","data":{"body":"Room 1"}}`)
	hub.broadcast <- RoomMessage{RoomID: "roo_room1234567", Message: room1Msg}
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-client.send:
		if string(msg) != string(room1Msg) {
			t.Errorf("Expected room1 message, got: %s", msg)
		}
	default:
		t.Error("Did not receive room1 message")
	}

	// Switch client to room2
	client.currentRoom = "roo_room2345678"

	// Send message to room1 - client should NOT receive it
	hub.broadcast <- RoomMessage{RoomID: "roo_room1234567", Message: room1Msg}
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-client.send:
		t.Errorf("SECURITY: Client received room1 message after switching to room2: %s", msg)
	default:
		// Expected
	}

	// Send message to room2 - client should receive it
	room2Msg := []byte(`{"type":"message","data":{"body":"Room 2"}}`)
	hub.broadcast <- RoomMessage{RoomID: "roo_room2345678", Message: room2Msg}
	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-client.send:
		if string(msg) != string(room2Msg) {
			t.Errorf("Expected room2 message, got: %s", msg)
		}
	default:
		t.Error("Did not receive room2 message after switching rooms")
	}
}

// TestHub_ConcurrentBroadcasts tests thread safety of concurrent broadcasts
func TestHub_ConcurrentBroadcasts(t *testing.T) {
	hub := &Hub{
		broadcast:  make(chan RoomMessage, 100), // Buffered for concurrent sends
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}

	// Create clients in two different rooms
	room1Client := &Client{
		hub:         hub,
		currentRoom: "roo_room1234567",
		send:        make(chan []byte, 1000),
	}
	room2Client := &Client{
		hub:         hub,
		currentRoom: "roo_room2345678",
		send:        make(chan []byte, 1000),
	}
	hub.clients[room1Client] = true
	hub.clients[room2Client] = true

	go hub.run()

	// Send messages concurrently from multiple goroutines
	var wg sync.WaitGroup
	const messagesPerRoom = 50

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < messagesPerRoom; i++ {
			hub.broadcast <- RoomMessage{
				RoomID:  "roo_room1234567",
				Message: []byte(`{"room":"1"}`),
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < messagesPerRoom; i++ {
			hub.broadcast <- RoomMessage{
				RoomID:  "roo_room2345678",
				Message: []byte(`{"room":"2"}`),
			}
		}
	}()

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// Count messages for each client
	room1Count := len(room1Client.send)
	room2Count := len(room2Client.send)

	if room1Count != messagesPerRoom {
		t.Errorf("Room1 client received %d messages, expected %d", room1Count, messagesPerRoom)
	}
	if room2Count != messagesPerRoom {
		t.Errorf("Room2 client received %d messages, expected %d", room2Count, messagesPerRoom)
	}

	// Verify no cross-contamination
	for i := 0; i < room1Count; i++ {
		msg := <-room1Client.send
		if string(msg) != `{"room":"1"}` {
			t.Errorf("Room1 client received wrong message: %s", msg)
		}
	}
	for i := 0; i < room2Count; i++ {
		msg := <-room2Client.send
		if string(msg) != `{"room":"2"}` {
			t.Errorf("Room2 client received wrong message: %s", msg)
		}
	}
}
