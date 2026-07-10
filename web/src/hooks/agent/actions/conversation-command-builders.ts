import type { WebSocketMessage } from "@/types/system/websocket";

import type { ResolvedConversationActionContext } from "./conversation-action-context";

export function buildConversationAddress(
  context: ResolvedConversationActionContext,
): Record<string, unknown> {
  const roomScope = context.chatType === "group"
    ? {
        ...(context.roomId ? { room_id: context.roomId } : {}),
        ...(context.conversationId
          ? { conversation_id: context.conversationId }
          : {}),
      }
    : {};
  return {
    agent_id: context.agentId,
    session_key: context.sessionKey,
    ...roomScope,
  };
}

export function buildConversationScope(
  context: ResolvedConversationActionContext,
): Record<string, unknown> {
  return {
    ...buildConversationAddress(context),
    ...(context.chatType === "group" ? { chat_type: "group" } : {}),
  };
}

export function buildSessionBindMessage({
  session_key: sessionKey,
  last_seen_session_seq: lastSeenSessionSeq,
  agent_id: agentId,
  room_id: roomId,
  conversation_id: conversationId,
}: {
  session_key: string;
  last_seen_session_seq?: number;
  agent_id?: string | null;
  room_id?: string | null;
  conversation_id?: string | null;
}): WebSocketMessage {
  return {
    type: "bind_session",
    session_key: sessionKey,
    ...(lastSeenSessionSeq && lastSeenSessionSeq > 0
      ? { last_seen_session_seq: lastSeenSessionSeq }
      : {}),
    ...(agentId ? { agent_id: agentId } : {}),
    ...(roomId ? { room_id: roomId } : {}),
    ...(conversationId ? { conversation_id: conversationId } : {}),
  };
}

export function buildRoomSubscriptionMessage({
  type,
  room_id: roomId,
  conversation_id: conversationId,
  last_seen_room_seq: lastSeenRoomSeq,
}: {
  type: "subscribe_room" | "unsubscribe_room";
  room_id: string;
  conversation_id?: string | null;
  last_seen_room_seq?: number;
}): WebSocketMessage {
  return {
    type,
    room_id: roomId,
    ...(conversationId ? { conversation_id: conversationId } : {}),
    ...(type === "subscribe_room" && lastSeenRoomSeq && lastSeenRoomSeq > 0
      ? { last_seen_room_seq: lastSeenRoomSeq }
      : {}),
  };
}
