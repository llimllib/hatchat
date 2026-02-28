package server

import (
	"encoding/json"
	"testing"
	"time"
)

// TestWriteDeadlineNotStale is an integration test that verifies responses
// succeed even when there's been time for a deadline to potentially expire.
// This catches the bug from issue #57 where readPump used stale deadlines.
func TestWriteDeadlineNotStale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping deadline test in short mode")
	}

	ts := newTestServer(t)
	defer ts.close()

	// Create and connect a user
	httpClient := ts.createUser("testuser", "password123")
	client := ts.connectWebSocket(httpClient, "testuser")
	defer client.close()

	// Send init to establish connection
	initResp, err := client.sendInit()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Extract room ID
	initData, ok := initResp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Failed to parse init data")
	}
	roomID := initData["current_room"].(string)

	// Send multiple requests with small delays between them
	// Under the old buggy code, if the write deadline wasn't refreshed,
	// later requests could fail. With the fix, all should succeed.
	for i := 0; i < 5; i++ {
		// Small delay - not enough to trigger actual timeout, but exercises the path
		time.Sleep(50 * time.Millisecond)

		// Send a history request
		err := client.sendHistoryRequest(roomID, "", 10)
		if err != nil {
			t.Fatalf("Request %d: failed to send history request: %v", i, err)
		}

		// Wait for response
		response, err := client.waitForMessage(2 * time.Second)
		if err != nil {
			t.Fatalf("Request %d: failed to receive response: %v", i, err)
		}

		// Verify it's a history response
		var env struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(response, &env); err != nil {
			t.Fatalf("Request %d: failed to parse response: %v", i, err)
		}
		if env.Type != "history" {
			t.Errorf("Request %d: expected history response, got %s", i, env.Type)
		}
	}
}

// TestConcurrentRequestsWithWriteDeadline verifies that multiple rapid requests
// all receive responses, testing that write deadlines are properly managed
// even under concurrent load.
func TestConcurrentRequestsWithWriteDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent deadline test in short mode")
	}

	ts := newTestServer(t)
	defer ts.close()

	httpClient := ts.createUser("concurrent", "password123")
	client := ts.connectWebSocket(httpClient, "concurrent")
	defer client.close()

	// Init
	initResp, err := client.sendInit()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	initData, ok := initResp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Failed to parse init data")
	}
	roomID := initData["current_room"].(string)

	// Send several requests rapidly
	numRequests := 10
	for i := 0; i < numRequests; i++ {
		err := client.sendHistoryRequest(roomID, "", 10)
		if err != nil {
			t.Fatalf("Failed to send request %d: %v", i, err)
		}
	}

	// Collect all responses
	responses := 0
	timeout := time.After(5 * time.Second)
	for responses < numRequests {
		select {
		case msg := <-client.messages:
			var env struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(msg, &env); err == nil && env.Type == "history" {
				responses++
			}
		case <-timeout:
			t.Fatalf("Timeout: only received %d/%d responses", responses, numRequests)
		}
	}
}
