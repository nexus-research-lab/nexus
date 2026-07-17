/**
 * INPUT: 历史快照、WebSocket 增量消息与 optimistic 本地消息。
 * OUTPUT: message_id 唯一且不会被旧快照回滚的会话消息集合。
 * POS: 所有消息传输通道汇合后的内存一致性边界。
 */
import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
} from "@/types/conversation/message/entity";

import {
  mergeAssistantMessage,
  normalizeAssistantMessage,
  normalizeAssistantMessages,
} from "./assistant-message-model";

interface IndexedMessage {
  lastIndex: number;
  message: Message;
}

/** 不同传输通道汇入内存后，message_id 必须保持唯一。 */
export function dedupeMessagesById(messages: Message[]): Message[] {
  if (messages.length <= 1) {
    return messages;
  }

  const indexedMessages = new Map<string, IndexedMessage>();
  messages.forEach((message, index) => {
    const existing = indexedMessages.get(message.message_id)?.message;
    indexedMessages.set(message.message_id, {
      lastIndex: index,
      message: existing ? mergeMessageById(existing, message) : message,
    });
  });
  if (indexedMessages.size === messages.length) {
    return messages;
  }

  return messages.flatMap((message, index) => {
    const indexed = indexedMessages.get(message.message_id);
    return indexed?.lastIndex === index ? [indexed.message] : [];
  });
}

export function upsertMessage(
  messages: Message[],
  incoming: Message,
): Message[] {
  const uniqueMessages = dedupeMessagesById(messages);
  const normalizedIncoming =
    incoming.role === "assistant"
      ? normalizeAssistantMessage(incoming)
      : incoming;
  const existingIndex = uniqueMessages.findIndex(
    (message) => message.message_id === normalizedIncoming.message_id,
  );
  if (existingIndex === -1) {
    return normalizeAssistantMessages(
      [...uniqueMessages, normalizedIncoming],
    );
  }

  const nextMessages = [...uniqueMessages];
  nextMessages[existingIndex] = mergeMessageById(
    nextMessages[existingIndex],
    normalizedIncoming,
  );
  return normalizeAssistantMessages(nextMessages);
}

export function sortMessages(messages: Message[]): Message[] {
  const uniqueMessages = dedupeMessagesById(messages);
  return normalizeAssistantMessages(
    [...uniqueMessages].sort((left, right) => left.timestamp - right.timestamp),
  );
}

/** 同 ID 按单调进度归并，仅补回尚未落库的本地 optimistic 消息。 */
export function mergeLoadedMessages(
  loadedMessages: Message[],
  localMessages: Message[],
): Message[] {
  const uniqueLoadedMessages = dedupeMessagesById(loadedMessages);
  if (localMessages.length === 0) {
    return sortMessages(uniqueLoadedMessages);
  }

  const uniqueLocalMessages = dedupeMessagesById(localMessages);
  const localMessagesById = new Map(
    uniqueLocalMessages.map((message) => [message.message_id, message]),
  );
  const reconciledLoadedMessages = uniqueLoadedMessages.map((message) => {
    const localMessage = localMessagesById.get(message.message_id);
    if (
      localMessage?.role === "assistant"
      && message.role === "assistant"
    ) {
      return mergeLoadedAssistantMessage(message, localMessage);
    }
    // 历史请求没有版本戳，可能晚于 durable reparent 增量返回。reparent
    // 对同一 user message_id 是单向状态；仅保护身份字段，其他服务端字段仍可刷新。
    if (
      localMessage
      && isReparentedUserMessage(localMessage)
      && !isReparentedUserMessage(message)
    ) {
      return localMessage;
    }
    return message;
  });
  const loadedMessageIds = new Set(
    reconciledLoadedMessages.map((message) => message.message_id),
  );
  const localOnlyMessages = uniqueLocalMessages.filter(
    (message) => !loadedMessageIds.has(message.message_id),
  );
  return sortMessages([...reconciledLoadedMessages, ...localOnlyMessages]);
}

const ASSISTANT_STATUS_PROGRESS: Record<AssistantMessageStatus, number> = {
  pending: 0,
  streaming: 1,
  cancelled: 2,
  done: 2,
  error: 2,
};

function mergeLoadedAssistantMessage(
  loaded: AssistantMessage,
  local: AssistantMessage,
): AssistantMessage {
  if (!isAssistantSnapshotAhead(loaded, local)) {
    return local;
  }
  return mergeAssistantMessage(local, loaded);
}

function isAssistantSnapshotAhead(
  candidate: AssistantMessage,
  current: AssistantMessage,
): boolean {
  const candidateProgress = assistantSnapshotProgress(candidate);
  const currentProgress = assistantSnapshotProgress(current);
  for (let index = 0; index < candidateProgress.length; index += 1) {
    if (candidateProgress[index] !== currentProgress[index]) {
      return candidateProgress[index] > currentProgress[index];
    }
  }
  return false;
}

function assistantSnapshotProgress(
  message: AssistantMessage,
): readonly number[] {
  const normalized = normalizeAssistantMessage(message);
  return [
    ASSISTANT_STATUS_PROGRESS[normalized.stream_status ?? "streaming"],
    normalized.result_summary?.timestamp ?? 0,
    normalized.result_summary ? 1 : 0,
    normalized.stop_reason ? 1 : 0,
    serializedSize(normalized.content),
    serializedSize(normalized.usage),
    normalized.timestamp,
  ];
}

function serializedSize(value: unknown): number {
  return value === undefined ? 0 : JSON.stringify(value).length;
}

function isReparentedUserMessage(message: Message): boolean {
  const sourceRoundId = message.source_round_id?.trim();
  return message.role === "user"
    && Boolean(sourceRoundId && sourceRoundId !== message.round_id.trim());
}

function mergeMessageById(existing: Message, incoming: Message): Message {
  if (existing.role === "assistant" && incoming.role === "assistant") {
    return mergeAssistantMessage(existing, incoming);
  }
  return incoming;
}
