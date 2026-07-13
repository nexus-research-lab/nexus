import type { ChatNotificationTargetState } from "@/store/sidebar";
import type {
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";
import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";
import type { ContentBlock } from "@/types/conversation/message/content";
import type { EventMessage } from "@/types/generated/protocol";

import {
  findNotificationConversation,
  type ChatNotificationDirectoryIndex,
} from "./chat-notification-directory";
import { buildChatNotificationTargetKey } from "./chat-notification-target";

const NOTIFICATION_TEXT_LIMIT = 120;

export interface ChatNotificationTarget {
  agent_id?: string | null;
  conversation_id?: string | null;
  key: string;
  room_id?: string | null;
  session_key?: string | null;
}

interface NotificationDirectoryContext {
  agentName: string | undefined;
  conversation: LauncherConversationSummary | undefined;
  room: LauncherRoomSummary | undefined;
}

function firstDefined<T>(
  values: Array<T | null | undefined>,
): T | null {
  for (const value of values) {
    if (value !== null && value !== undefined) {
      return value;
    }
  }
  return null;
}

function firstNonEmpty(values: Array<string | null | undefined>): string | null {
  for (const value of values) {
    const normalized = value?.trim();
    if (normalized) {
      return normalized;
    }
  }
  return null;
}

export function isCompletedAssistantMessage(
  message: Message | null | undefined,
): message is AssistantMessage {
  if (!message || message.role !== "assistant" || message.result_summary?.subtype === "interrupted") {
    return false;
  }
  const completionSignals = [
    message.result_summary,
    message.is_complete,
    message.stop_reason,
    message.stream_status === "done",
    message.stream_status === "error",
  ];
  return completionSignals.some(Boolean);
}

function resolveNotificationTargetLocation(
  event: EventMessage,
  message: Message,
  index: ChatNotificationDirectoryIndex,
): Omit<ChatNotificationTarget, "key"> {
  const eventConversationId = firstDefined([
    event.conversation_id,
    message.conversation_id,
  ]);
  const sessionKey = firstDefined([event.session_key, message.session_key]);
  const directoryConversation = findNotificationConversation(
    index,
    eventConversationId,
    sessionKey,
  );
  return {
    agent_id: firstDefined([event.agent_id, message.agent_id]),
    conversation_id: firstDefined([
      eventConversationId,
      directoryConversation?.conversation_id,
    ]),
    room_id: firstDefined([
      event.room_id,
      message.room_id,
      directoryConversation?.room_id,
    ]),
    session_key: sessionKey,
  };
}

export function buildMessageNotificationTarget(
  event: EventMessage,
  message: Message,
  index: ChatNotificationDirectoryIndex,
): ChatNotificationTarget | null {
  const location = resolveNotificationTargetLocation(event, message, index);
  const key = buildChatNotificationTargetKey({
    conversation_id: location.conversation_id,
    room_id: location.room_id,
    session_key: location.session_key,
  });
  return key ? { ...location, key } : null;
}

function resolveNotificationDirectoryContext(
  target: ChatNotificationTarget,
  message: AssistantMessage,
  index: ChatNotificationDirectoryIndex,
): NotificationDirectoryContext {
  const room = target.room_id
    ? index.roomsById.get(target.room_id)
    : undefined;
  const conversation = target.conversation_id
    ? index.conversationsById.get(target.conversation_id)
    : undefined;
  const agent = message.agent_id
    ? index.agentsById.get(message.agent_id)
    : undefined;
  return { agentName: agent?.name, conversation, room };
}

function resolveDmNotificationTitle(
  context: NotificationDirectoryContext,
): string {
  return firstDefined([
    context.agentName,
    context.conversation?.title,
    context.room?.name,
  ]) ?? "Nexus";
}

function resolveGroupNotificationTitle(
  context: NotificationDirectoryContext,
): string {
  return firstNonEmpty([
    context.room?.name,
    context.conversation?.title,
  ]) ?? "群聊";
}

function resolveNotificationTitle(
  context: NotificationDirectoryContext,
): string {
  const resolver = context.room?.room_type === "dm"
    ? resolveDmNotificationTitle
    : resolveGroupNotificationTitle;
  return resolver(context);
}

function prefixGroupNotificationBody(
  body: string,
  context: NotificationDirectoryContext,
): string {
  return context.room?.room_type === "room" && context.agentName
    ? compactNotificationText(`${context.agentName}: ${body}`)
    : body;
}

export function buildNotificationContent(
  target: ChatNotificationTarget,
  message: AssistantMessage,
  index: ChatNotificationDirectoryIndex,
): { body: string; title: string } {
  const context = resolveNotificationDirectoryContext(target, message, index);
  const body = getMessageNotificationBody(message);
  return {
    body: prefixGroupNotificationBody(body, context),
    title: resolveNotificationTitle(context),
  };
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

function isErrorResult(message: AssistantMessage): boolean {
  const summary = message.result_summary;
  return summary?.subtype === "error" || summary?.is_error === true;
}

function getResultSummaryBody(message: AssistantMessage): string | null {
  const summaryResult = message.result_summary?.result?.trim();
  if (summaryResult) {
    return compactNotificationText(summaryResult);
  }
  return isErrorResult(message) ? "执行失败" : null;
}

function getMessageNotificationBody(message: AssistantMessage): string {
  const resultBody = getResultSummaryBody(message);
  if (resultBody) {
    return resultBody;
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
