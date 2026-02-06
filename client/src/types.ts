// Re-export protocol types and schemas from generated file
export type {
  AddReactionRequest,
  ClientEnvelope,
  CreateDMRequest,
  CreateDMResponse,
  CreateRoomRequest,
  CreateRoomResponse,
  DeleteMessageRequest,
  EditMessageRequest,
  Envelope,
  ErrorResponse,
  GetMessageContextRequest,
  GetMessageContextResponse,
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
  MessageDeleted,
  MessageEdited,
  MessageType,
  Reaction,
  ReactionUpdated,
  RemoveReactionRequest,
  Room,
  RoomInfoRequest,
  RoomInfoResponse,
  RoomMember,
  SearchRequest,
  SearchResponse,
  SearchResult,
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
  GetMessageContextResponseSchema,
  GetProfileResponseSchema,
  HistoryResponseSchema,
  InitResponseSchema,
  isMessageType,
  JoinRoomResponseSchema,
  LeaveRoomResponseSchema,
  ListRoomsResponseSchema,
  ListUsersResponseSchema,
  MessageDeletedSchema,
  MessageEditedSchema,
  MessageSchema,
  parseServerEnvelope,
  ReactionSchema,
  ReactionUpdatedSchema,
  RoomInfoResponseSchema,
  RoomMemberSchema,
  RoomSchema,
  SearchResponseSchema,
  SearchResultSchema,
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
