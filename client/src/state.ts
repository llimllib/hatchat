import type { InitResponse, Message, Room } from "./types";

/**
 * Per-room state that persists when switching rooms
 */
export interface RoomState {
  messages: Message[];
  historyCursor?: string;
  hasMoreHistory: boolean;
  scrollPosition: number;
}

/**
 * Application state management with room message caching and scroll position tracking.
 */
export class AppState {
  initialData?: InitResponse;
  currentRoom?: string;

  // Per-room cached state
  private roomStates: Map<string, RoomState> = new Map();

  /**
   * Initialize state with data from server
   */
  setInitialData(data: InitResponse) {
    this.initialData = data;
  }

  /**
   * Get the current user, throws if not initialized
   */
  get user() {
    if (!this.initialData) {
      throw new Error("Not yet initialized");
    }
    return this.initialData.user;
  }

  /**
   * Get all channel rooms the user is a member of
   */
  get rooms(): Room[] {
    return this.initialData?.rooms || [];
  }

  /**
   * Get all DM rooms the user is a member of (sorted by most recent activity)
   */
  get dms(): Room[] {
    return this.initialData?.dms || [];
  }

  /**
   * Get a room by ID (searches both channels and DMs)
   */
  getRoom(roomId: string): Room | undefined {
    return (
      this.rooms.find((r) => r.id === roomId) ||
      this.dms.find((r) => r.id === roomId)
    );
  }

  /**
   * Add a new room to the user's room list
   */
  addRoom(room: Room) {
    if (!this.initialData) {
      throw new Error("Not yet initialized");
    }
    if (room.room_type === "dm") {
      // Add DM to beginning (most recent)
      const exists = this.initialData.dms.some((r) => r.id === room.id);
      if (!exists) {
        this.initialData.dms.unshift(room);
      }
    } else {
      // Add channel
      const exists = this.initialData.rooms.some((r) => r.id === room.id);
      if (!exists) {
        this.initialData.rooms.push(room);
        // Sort channels by name
        this.initialData.rooms.sort((a: Room, b: Room) =>
          a.name.localeCompare(b.name),
        );
      }
    }
  }

  /**
   * Remove a room from the user's room list
   */
  removeRoom(roomId: string) {
    if (!this.initialData) {
      throw new Error("Not yet initialized");
    }
    this.initialData.rooms = this.initialData.rooms.filter(
      (r) => r.id !== roomId,
    );
    this.initialData.dms = this.initialData.dms.filter((r) => r.id !== roomId);
    // Also clear the room state cache
    this.roomStates.delete(roomId);
  }

  /**
   * Get state for a specific room, creating it if needed
   */
  getRoomState(roomId: string): RoomState {
    let state = this.roomStates.get(roomId);
    if (!state) {
      state = {
        messages: [],
        historyCursor: undefined,
        hasMoreHistory: false,
        scrollPosition: 0,
      };
      this.roomStates.set(roomId, state);
    }
    return state;
  }

  /**
   * Get state for the current room
   */
  getCurrentRoomState(): RoomState | undefined {
    if (!this.currentRoom) {
      return undefined;
    }
    return this.getRoomState(this.currentRoom);
  }

  /**
   * Check if we have cached messages for a room
   */
  hasMessagesForRoom(roomId: string): boolean {
    const state = this.roomStates.get(roomId);
    return state !== undefined && state.messages.length > 0;
  }

  /**
   * Add messages to a room's cache. Handles both:
   * - History loading (prepending older messages)
   * - New incoming messages (appending)
   */
  addMessages(roomId: string, messages: Message[], prepend: boolean = false) {
    const state = this.getRoomState(roomId);

    if (prepend) {
      // Prepending older history messages - avoid duplicates
      const existingIds = new Set(state.messages.map((m) => m.id));
      const newMessages = messages.filter((m) => !existingIds.has(m.id));
      state.messages = [...newMessages, ...state.messages];
    } else {
      // Appending new messages
      const existingIds = new Set(state.messages.map((m) => m.id));
      const newMessages = messages.filter((m) => !existingIds.has(m.id));
      state.messages = [...state.messages, ...newMessages];
    }
  }

  /**
   * Add a single new message to a room
   */
  addMessage(roomId: string, message: Message) {
    this.addMessages(roomId, [message], false);
  }

  /**
   * Update pagination state for a room
   */
  updatePagination(
    roomId: string,
    cursor: string | undefined,
    hasMore: boolean,
  ) {
    const state = this.getRoomState(roomId);
    state.historyCursor = cursor;
    state.hasMoreHistory = hasMore;
  }

  /**
   * Save scroll position for a room
   */
  saveScrollPosition(roomId: string, position: number) {
    const state = this.getRoomState(roomId);
    state.scrollPosition = position;
  }

  /**
   * Switch to a different room
   */
  setCurrentRoom(roomId: string) {
    this.currentRoom = roomId;
  }

  /**
   * Move a DM to the top of the list (called when a new message is received)
   */
  bumpDM(roomId: string) {
    if (!this.initialData) return;
    const idx = this.initialData.dms.findIndex((r) => r.id === roomId);
    if (idx > 0) {
      const [dm] = this.initialData.dms.splice(idx, 1);
      this.initialData.dms.unshift(dm);
    }
  }
}
