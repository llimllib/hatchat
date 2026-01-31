export interface Room {
  id: string;
  name: string;
  is_private: boolean;
}

export interface User {
  id: `usr_${string}`;
  username: string;
  avatar: string;
}

export interface InitialData {
  Rooms: Room[];
  User: User;
  current_room: string;
}

export interface Message {
  id: string;
  room_id: string;
  user_id: string;
  username: string;
  body: string;
  created_at: string;
  modified_at: string;
}

export interface HistoryResponse {
  messages: Message[];
  has_more: boolean;
  next_cursor: string;
}

// Pending message waiting for server confirmation
export interface PendingMessage {
  tempId: string;
  body: string;
  roomId: string;
  element: HTMLElement;
}

/**
 * Create a unique key for matching pending messages with server confirmations.
 * We match by user_id + room_id + body since we don't have the server ID yet.
 */
export function makePendingKey(
  body: string,
  roomId: string,
  userId: string,
): string {
  return `${userId}:${roomId}:${body}`;
}
