import { beforeEach, describe, expect, it } from "vitest";
import { AppState } from "./state";
import type { InitialData, Message } from "./types";

describe("AppState", () => {
  let state: AppState;

  // Factory function to create fresh mock data for each test
  const createMockInitialData = (): InitialData => ({
    Rooms: [
      { id: "roo_123", name: "general", is_private: false },
      { id: "roo_456", name: "random", is_private: false },
    ],
    User: { id: "usr_abc123", username: "testuser", avatar: "" },
    current_room: "roo_123",
  });

  // For backwards compatibility with existing tests
  const mockInitialData = createMockInitialData();

  const createMessage = (overrides: Partial<Message> = {}): Message => ({
    id: `msg_${Math.random().toString(36).slice(2)}`,
    room_id: "roo_123",
    user_id: "usr_abc123",
    username: "testuser",
    body: "Hello world",
    created_at: new Date().toISOString(),
    modified_at: new Date().toISOString(),
    ...overrides,
  });

  beforeEach(() => {
    state = new AppState();
  });

  describe("initialization", () => {
    it("starts without initial data", () => {
      expect(state.initialData).toBeUndefined();
      expect(state.currentRoom).toBeUndefined();
    });

    it("stores initial data", () => {
      state.setInitialData(mockInitialData);
      expect(state.initialData).toBe(mockInitialData);
    });

    it("provides access to user after initialization", () => {
      state.setInitialData(mockInitialData);
      expect(state.user.username).toBe("testuser");
    });

    it("throws when accessing user before initialization", () => {
      expect(() => state.user).toThrow("Not yet initialized");
    });
  });

  describe("rooms", () => {
    it("returns empty array when not initialized", () => {
      expect(state.rooms).toEqual([]);
    });

    it("returns rooms when initialized", () => {
      state.setInitialData(mockInitialData);
      expect(state.rooms).toHaveLength(2);
      expect(state.rooms[0].name).toBe("general");
    });

    it("finds room by ID", () => {
      state.setInitialData(mockInitialData);
      const room = state.getRoom("roo_456");
      expect(room?.name).toBe("random");
    });

    it("returns undefined for unknown room ID", () => {
      state.setInitialData(mockInitialData);
      expect(state.getRoom("roo_unknown")).toBeUndefined();
    });

    it("adds a new room", () => {
      state.setInitialData(createMockInitialData());
      state.addRoom({ id: "roo_789", name: "new-channel", is_private: false });
      expect(state.rooms).toHaveLength(3);
      expect(state.getRoom("roo_789")?.name).toBe("new-channel");
    });

    it("sorts rooms by name after adding", () => {
      state.setInitialData(createMockInitialData());
      state.addRoom({ id: "roo_aaa", name: "aardvark", is_private: false });
      expect(state.rooms[0].name).toBe("aardvark");
    });

    it("does not add duplicate rooms", () => {
      state.setInitialData(createMockInitialData());
      state.addRoom({ id: "roo_123", name: "general-dupe", is_private: false });
      expect(state.rooms).toHaveLength(2); // Still 2, not 3
      expect(state.getRoom("roo_123")?.name).toBe("general"); // Original name preserved
    });

    it("throws when adding room before initialization", () => {
      expect(() =>
        state.addRoom({ id: "roo_new", name: "test", is_private: false }),
      ).toThrow("Not yet initialized");
    });
  });

  describe("room state", () => {
    it("creates new room state on first access", () => {
      const roomState = state.getRoomState("roo_123");
      expect(roomState.messages).toEqual([]);
      expect(roomState.hasMoreHistory).toBe(false);
      expect(roomState.scrollPosition).toBe(0);
    });

    it("returns same state on subsequent access", () => {
      const roomState1 = state.getRoomState("roo_123");
      roomState1.scrollPosition = 100;
      const roomState2 = state.getRoomState("roo_123");
      expect(roomState2.scrollPosition).toBe(100);
    });

    it("tracks current room state", () => {
      state.setCurrentRoom("roo_123");
      const currentState = state.getCurrentRoomState();
      expect(currentState).toBeDefined();
    });
  });

  describe("message caching", () => {
    beforeEach(() => {
      state.setInitialData(mockInitialData);
      state.setCurrentRoom("roo_123");
    });

    it("adds messages to room", () => {
      const msg = createMessage();
      state.addMessage("roo_123", msg);
      expect(state.hasMessagesForRoom("roo_123")).toBe(true);
      expect(state.getRoomState("roo_123").messages).toHaveLength(1);
    });

    it("appends new messages at end", () => {
      const msg1 = createMessage({ id: "msg_1", body: "First" });
      const msg2 = createMessage({ id: "msg_2", body: "Second" });
      state.addMessage("roo_123", msg1);
      state.addMessage("roo_123", msg2);

      const messages = state.getRoomState("roo_123").messages;
      expect(messages[0].body).toBe("First");
      expect(messages[1].body).toBe("Second");
    });

    it("prepends history messages at beginning", () => {
      const msg1 = createMessage({ id: "msg_1", body: "Recent" });
      state.addMessage("roo_123", msg1);

      const olderMessages = [
        createMessage({ id: "msg_old1", body: "Old 1" }),
        createMessage({ id: "msg_old2", body: "Old 2" }),
      ];
      state.addMessages("roo_123", olderMessages, true);

      const messages = state.getRoomState("roo_123").messages;
      expect(messages[0].body).toBe("Old 1");
      expect(messages[1].body).toBe("Old 2");
      expect(messages[2].body).toBe("Recent");
    });

    it("avoids duplicate messages", () => {
      const msg = createMessage({ id: "msg_123" });
      state.addMessage("roo_123", msg);
      state.addMessage("roo_123", msg); // Add same message again

      expect(state.getRoomState("roo_123").messages).toHaveLength(1);
    });

    it("returns false for empty room", () => {
      expect(state.hasMessagesForRoom("roo_456")).toBe(false);
    });
  });

  describe("pagination", () => {
    it("updates pagination state", () => {
      state.updatePagination("roo_123", "2024-01-01T00:00:00Z", true);
      const roomState = state.getRoomState("roo_123");
      expect(roomState.historyCursor).toBe("2024-01-01T00:00:00Z");
      expect(roomState.hasMoreHistory).toBe(true);
    });

    it("clears cursor when no more history", () => {
      state.updatePagination("roo_123", undefined, false);
      const roomState = state.getRoomState("roo_123");
      expect(roomState.historyCursor).toBeUndefined();
      expect(roomState.hasMoreHistory).toBe(false);
    });
  });

  describe("scroll position", () => {
    it("saves scroll position", () => {
      state.saveScrollPosition("roo_123", 500);
      expect(state.getRoomState("roo_123").scrollPosition).toBe(500);
    });

    it("preserves scroll position per room", () => {
      state.saveScrollPosition("roo_123", 100);
      state.saveScrollPosition("roo_456", 200);

      expect(state.getRoomState("roo_123").scrollPosition).toBe(100);
      expect(state.getRoomState("roo_456").scrollPosition).toBe(200);
    });
  });
});
