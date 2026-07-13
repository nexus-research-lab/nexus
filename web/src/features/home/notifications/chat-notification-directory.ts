import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

import { buildChatNotificationTargetKey } from "./chat-notification-target";

export interface ChatNotificationDirectory {
  agents: LauncherAgentSummary[];
  conversations: LauncherConversationSummary[];
  rooms: LauncherRoomSummary[];
}

export interface ChatNotificationDirectoryIndex {
  agentsById: Map<string, LauncherAgentSummary>;
  conversationsById: Map<string, LauncherConversationSummary>;
  conversationsBySessionKey: Map<string, LauncherConversationSummary>;
  roomsById: Map<string, LauncherRoomSummary>;
  sessionTargetKeysByRoomId: Map<string, string[]>;
}

function appendRoomSessionTarget(
  index: ChatNotificationDirectoryIndex,
  conversation: LauncherConversationSummary,
): void {
  if (!conversation.room_id) {
    return;
  }
  const targetKey = buildChatNotificationTargetKey({
    session_key: conversation.session_key,
  });
  if (!targetKey) {
    return;
  }
  const keys = index.sessionTargetKeysByRoomId.get(conversation.room_id) ?? [];
  keys.push(targetKey);
  index.sessionTargetKeysByRoomId.set(conversation.room_id, keys);
}

function indexConversation(
  index: ChatNotificationDirectoryIndex,
  conversation: LauncherConversationSummary,
): void {
  if (conversation.conversation_id) {
    index.conversationsById.set(conversation.conversation_id, conversation);
  }
  if (conversation.session_key) {
    index.conversationsBySessionKey.set(conversation.session_key, conversation);
  }
  appendRoomSessionTarget(index, conversation);
}

export function buildChatNotificationDirectoryIndex(
  directory: ChatNotificationDirectory,
): ChatNotificationDirectoryIndex {
  const index: ChatNotificationDirectoryIndex = {
    agentsById: new Map(directory.agents.map((agent) => [agent.id, agent])),
    conversationsById: new Map(),
    conversationsBySessionKey: new Map(),
    roomsById: new Map(directory.rooms.map((room) => [room.id, room])),
    sessionTargetKeysByRoomId: new Map(),
  };
  for (const conversation of directory.conversations) {
    indexConversation(index, conversation);
  }
  return index;
}

export function findNotificationConversation(
  index: ChatNotificationDirectoryIndex,
  conversationId: string | null,
  sessionKey: string | null,
): LauncherConversationSummary | undefined {
  if (conversationId) {
    return index.conversationsById.get(conversationId);
  }
  return sessionKey
    ? index.conversationsBySessionKey.get(sessionKey)
    : undefined;
}

export function getRoomSessionTargetKeys(
  index: ChatNotificationDirectoryIndex,
  roomId: string,
): string[] {
  return index.sessionTargetKeysByRoomId.get(roomId) ?? [];
}
