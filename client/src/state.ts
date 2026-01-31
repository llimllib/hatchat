import type { InitialData, Message, Room } from "./types";

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
  initialData?: InitialData;
  currentRoom?: string;

  // Per-room cached state
  private roomStates: Map<string, RoomState> = new Map();

  /**
   * Initialize state with data from server
   */
  setInitialData(data: InitialData) {
    this.initialData = data;
  }

  /**
   * Get the current user, throws if not initialized
   */
  get user() {
    if (!this.initialData) {
      throw new Error("Not yet initialized");
    }
    return this.initialData.User;
  }

  /**
   * Get all rooms the user is a member of
   */
  get rooms(): Room[] {
    return this.initialData?.Rooms || [];
  }

  /**
   * Get a room by ID
   */
  getRoom(roomId: string): Room | undefined {
    return this.rooms.find((r) => r.id === roomId);
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
}
