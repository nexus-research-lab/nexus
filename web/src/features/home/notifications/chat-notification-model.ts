import type { ChatNotificationTargetState } from "@/store/sidebar";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";
import type {
  AssistantMessage,
  ContentBlock,
  EventMessage,
  Message,
} from "@/types/conversation/message";

import { buildChatNotificationTargetKey } from "./chat-notification-target";

const NOTIFICATION_TEXT_LIMIT = 120;

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

export interface ChatNotificationTarget {
  agent_id?: string | null;
  conversation_id?: string | null;
  key: string;
  room_id?: string | null;
  session_key?: string | null;
}

export function buildChatNotificationDirectoryIndex(
  directory: ChatNotificationDirectory,
): ChatNotificationDirectoryIndex {
  const conversationsById = new Map<string, LauncherConversationSummary>();
  const conversationsBySessionKey = new Map<string, LauncherConversationSummary>();
  const sessionTargetKeysByRoomId = new Map<string, string[]>();
  for (const conversation of directory.conversations) {
    if (conversation.conversation_id) {
      conversationsById.set(conversation.conversation_id, conversation);
    }
    if (conversation.session_key) {
      conversationsBySessionKey.set(conversation.session_key, conversation);
    }
    const sessionTargetKey = buildChatNotificationTargetKey({
      session_key: conversation.session_key,
    });
    if (conversation.room_id && sessionTargetKey) {
      const keys = sessionTargetKeysByRoomId.get(conversation.room_id) ?? [];
      keys.push(sessionTargetKey);
      sessionTargetKeysByRoomId.set(conversation.room_id, keys);
    }
  }
  return {
    agentsById: new Map(directory.agents.map((agent) => [agent.id, agent])),
    conversationsById,
    conversationsBySessionKey,
    roomsById: new Map(directory.rooms.map((room) => [room.id, room])),
    sessionTargetKeysByRoomId,
  };
}

export function isCompletedAssistantMessage(
  message: Message | null | undefined,
): message is AssistantMessage {
  if (!message || message.role !== "assistant" || message.result_summary?.subtype === "interrupted") {
    return false;
  }
  return Boolean(
    message.result_summary
    || message.is_complete
    || message.stop_reason
    || message.stream_status === "done"
    || message.stream_status === "error",
  );
}

export function buildMessageNotificationTarget(
  event: EventMessage,
  message: Message,
  index: ChatNotificationDirectoryIndex,
): ChatNotificationTarget | null {
  const eventConversationId = event.conversation_id ?? message.conversation_id ?? null;
  const sessionKey = event.session_key ?? message.session_key ?? null;
  const directoryConversation = eventConversationId
    ? index.conversationsById.get(eventConversationId)
    : sessionKey ? index.conversationsBySessionKey.get(sessionKey) : undefined;
  const conversationId = eventConversationId ?? directoryConversation?.conversation_id ?? null;
  const roomId = event.room_id ?? message.room_id ?? directoryConversation?.room_id ?? null;
  const key = buildChatNotificationTargetKey({
    conversation_id: conversationId,
    room_id: roomId,
    session_key: sessionKey,
  });
  return key
    ? {
        agent_id: event.agent_id ?? message.agent_id ?? null,
        conversation_id: conversationId,
        key,
        room_id: roomId,
        session_key: sessionKey,
      }
    : null;
}

export function buildNotificationContent(
  target: ChatNotificationTarget,
  message: AssistantMessage,
  index: ChatNotificationDirectoryIndex,
): { body: string; title: string } {
  const room = target.room_id ? index.roomsById.get(target.room_id) : undefined;
  const conversation = target.conversation_id
    ? index.conversationsById.get(target.conversation_id)
    : undefined;
  const agent = message.agent_id ? index.agentsById.get(message.agent_id) : undefined;
  const title = room?.room_type === "dm"
    ? agent?.name ?? conversation?.title ?? room?.name ?? "Nexus"
    : room?.name?.trim() || conversation?.title?.trim() || "群聊";
  const body = getMessageNotificationBody(message);
  return room?.room_type === "room" && agent?.name
    ? { body: compactNotificationText(`${agent.name}: ${body}`), title }
    : { body, title };
}

export function getNotificationMessageId(
  event: EventMessage,
  message: AssistantMessage,
  targetKey: string,
): string {
  return message.message_id
    || event.message_id
    || message.result_summary?.message_id
    || `${targetKey}:${message.round_id}:${event.timestamp}`;
}

export function toChatNotificationTargetState(
  target: ChatNotificationTarget,
): ChatNotificationTargetState {
  return {
    conversation_id: target.conversation_id,
    key: target.key,
    room_id: target.room_id,
    session_key: target.session_key,
  };
}

function getMessageNotificationBody(message: AssistantMessage): string {
  const summaryResult = message.result_summary?.result?.trim();
  if (summaryResult) {
    return compactNotificationText(summaryResult);
  }
  if (message.result_summary?.subtype === "error" || message.result_summary?.is_error) {
    return "执行失败";
  }
  const text = extractTextFromContent(message.content);
  return text ? compactNotificationText(text) : "处理完成";
}

function extractTextFromContent(content?: ContentBlock[] | null): string {
  return (content ?? [])
    .filter((block): block is Extract<ContentBlock, { type: "text" }> =>
      block.type === "text" && Boolean(block.text.trim()))
    .map((block) => block.text.trim())
    .join("\n\n");
}

function compactNotificationText(value: string): string {
  const normalized = value.replace(/\s+/g, " ").trim();
  return normalized.length <= NOTIFICATION_TEXT_LIMIT
    ? normalized
    : `${normalized.slice(0, NOTIFICATION_TEXT_LIMIT - 1)}…`;
}
