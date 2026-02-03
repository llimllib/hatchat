// Re-export protocol types and schemas from generated file
export type {
  ClientEnvelope,
  CreateDMRequest,
  CreateDMResponse,
  CreateRoomRequest,
  CreateRoomResponse,
  Envelope,
  ErrorResponse,
  GetProfileRequest,
  GetProfileResponse,
  HistoryRequest,
  HistoryResponse,
  InitRequest,
  InitResponse,
  JoinRoomRequest,
  JoinRoomResponse,
  LeaveRoomRequest,
  LeaveRoomResponse,
  ListRoomsRequest,
  ListRoomsResponse,
  ListUsersRequest,
  ListUsersResponse,
  Message,
  MessageType,
  Room,
  RoomInfoRequest,
  RoomInfoResponse,
  RoomMember,
  SendMessageRequest,
  ServerEnvelope,
  UpdateProfileRequest,
  UpdateProfileResponse,
  User,
} from "./protocol.generated";

export {
  CreateDMResponseSchema,
  CreateRoomResponseSchema,
  ErrorResponseSchema,
  GetProfileResponseSchema,
  HistoryResponseSchema,
  InitResponseSchema,
  isMessageType,
  JoinRoomResponseSchema,
  LeaveRoomResponseSchema,
  ListRoomsResponseSchema,
  ListUsersResponseSchema,
  MessageSchema,
  parseServerEnvelope,
  RoomInfoResponseSchema,
  RoomMemberSchema,
  RoomSchema,
  // Zod schemas for runtime validation
  ServerEnvelopeSchema,
  safeParseServerEnvelope,
  UpdateProfileResponseSchema,
  UserSchema,
} from "./protocol.generated";

// =============================================================================
// Client-specific types (not part of the protocol)
// =============================================================================

/**
 * InitialData is an alias for InitResponse for backward compatibility.
 * Use InitResponse directly in new code.
 */
export type { InitResponse as InitialData } from "./protocol.generated";

/**
 * Pending message waiting for server confirmation.
 * This is a client-side concept for optimistic UI updates.
 */
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
