import type { Message } from "@/types";

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

/** 服务端快照覆盖同 ID 消息，仅补回尚未落库的本地 optimistic 消息。 */
export function mergeLoadedMessages(
  loadedMessages: Message[],
  localMessages: Message[],
): Message[] {
  const uniqueLoadedMessages = dedupeMessagesById(loadedMessages);
  if (localMessages.length === 0) {
    return sortMessages(uniqueLoadedMessages);
  }

  const loadedMessageIds = new Set(
    uniqueLoadedMessages.map((message) => message.message_id),
  );
  const localOnlyMessages = localMessages.filter(
    (message) => !loadedMessageIds.has(message.message_id),
  );
  return sortMessages([...uniqueLoadedMessages, ...localOnlyMessages]);
}

function mergeMessageById(existing: Message, incoming: Message): Message {
  if (existing.role === "assistant" && incoming.role === "assistant") {
    return mergeAssistantMessage(existing, incoming);
  }
  return incoming;
}
