/**
 * INPUT: 历史快照、WebSocket 增量消息与 optimistic 本地消息。
 * OUTPUT: message_id 唯一、同一条用户输入（input_seq）只占一行且不会被旧快照回滚的会话消息集合。
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

/**
 * 用户输入行的身份是后端受理时铸造的 input_seq：chat_ack、实时 durable 行与
 * 历史读取对同一条输入给出同一个值。本合同生效前落盘的行没有游标，仍以
 * message_id 为身份。
 */
function userInputSeq(message: Message): number | undefined {
  return message.role === "user" ? message.input_seq : undefined;
}

/** 已发出但尚未被任何受理证据绑定到游标的本地 optimistic 用户行。 */
function isPendingUserMessage(message: Message): boolean {
  return message.role === "user"
    && message.input_seq === undefined
    && message.client_message_id !== undefined
    && message.client_message_id === message.message_id;
}

/**
 * 找出与 durable 用户行是同一条输入的全部本地行：游标相同的行，以及被它的
 * client_message_id 点名、尚无游标的 optimistic 行。ACK 丢失时两者会并存
 * （历史或实时行先带来游标，optimistic 行还在等受理证据），必须一起收口。
 */
function findSameUserInputIndexes(
  messages: readonly Message[],
  durable: Message,
): number[] {
  if (durable.role !== "user") {
    return [];
  }
  const { client_message_id: clientMessageId, input_seq: inputSeq } = durable;
  return messages.flatMap((message, index) => {
    const sharesCursor = inputSeq !== undefined
      && userInputSeq(message) === inputSeq;
    const isNamedPendingRow = Boolean(clientMessageId)
      && isPendingUserMessage(message)
      && message.message_id === clientMessageId;
    return sharesCursor || isNamedPendingRow ? [index] : [];
  });
}

/** durable 字段以服务端为准；本页发出的行保留 client_message_id 作为 Feed 节点 key。 */
function adoptDurableUserMessage(
  locals: readonly Message[],
  durable: Message,
): Message {
  if (durable.role !== "user") {
    return durable;
  }
  const clientMessageId = locals.flatMap((local) => (
    local.role === "user" && local.client_message_id
      ? [local.client_message_id]
      : []
  ))[0];
  return clientMessageId
    ? { ...durable, client_message_id: clientMessageId }
    : durable;
}

/**
 * 实时 durable 用户行在同一次集合更新中原位收口同一条输入的本地行：Room 的
 * 广播先于 ACK 到达，重连重放会再送一遍，都不能短暂追加第二个 feed node。
 */
export function upsertRealtimeMessage(
  messages: Message[],
  incoming: Message,
): Message[] {
  const sameInputIndexes = findSameUserInputIndexes(messages, incoming);
  if (sameInputIndexes.length === 0) {
    return upsertMessage(messages, incoming);
  }
  const adopted = adoptDurableUserMessage(
    sameInputIndexes.map((index) => messages[index]),
    incoming,
  );
  return dedupeMessagesById(messages.flatMap((message, index) => {
    if (index === sameInputIndexes[0]) {
      return [adopted];
    }
    return sameInputIndexes.includes(index)
      || message.message_id === incoming.message_id
      ? []
      : [message];
  }));
}

export function sortMessages(messages: Message[]): Message[] {
  const uniqueMessages = dedupeMessagesById(messages);
  return normalizeAssistantMessages(
    [...uniqueMessages].sort((left, right) => left.timestamp - right.timestamp),
  );
}

/**
 * 历史快照与本地集合归并：用户行按 input_seq 配对（旧行按 message_id），其余按
 * message_id 做单调进度归并；只补回快照里没有对应行的本地消息。
 */
export function mergeLoadedMessages(
  loadedMessages: Message[],
  localMessages: Message[],
): Message[] {
  const uniqueLoadedMessages = dedupeMessagesById(loadedMessages);
  if (localMessages.length === 0) {
    return sortMessages(uniqueLoadedMessages);
  }

  const uniqueLocalMessages = dedupeMessagesById(localMessages);
  const localIndexesById = new Map(
    uniqueLocalMessages.map((message, index) => [message.message_id, index]),
  );
  const pairedLocalIndexes = new Set<number>();
  const reconciledLoadedMessages = uniqueLoadedMessages.map((message) => {
    const sameInputIndexes = findSameUserInputIndexes(
      uniqueLocalMessages,
      message,
    );
    const sameIdIndex = localIndexesById.get(message.message_id);
    const localIndexes = sameInputIndexes.length > 0 || sameIdIndex === undefined
      ? sameInputIndexes
      : [sameIdIndex];
    if (localIndexes.length === 0) {
      return message;
    }
    localIndexes.forEach((index) => pairedLocalIndexes.add(index));
    const locals = localIndexes.map((index) => uniqueLocalMessages[index]);
    const localMessage = locals[0];
    if (
      localMessage.role === "assistant"
      && message.role === "assistant"
    ) {
      return mergeLoadedAssistantMessage(message, localMessage);
    }
    // 历史请求没有版本戳，可能晚于 durable reparent 增量返回。reparent
    // 对同一条用户输入是单向状态；仅保护身份字段，其他服务端字段仍可刷新。
    const reparented = locals.find(isReparentedUserMessage);
    if (reparented && !isReparentedUserMessage(message)) {
      return reparented;
    }
    return adoptDurableUserMessage(locals, message);
  });
  const localOnlyMessages = uniqueLocalMessages.filter(
    (_, index) => !pairedLocalIndexes.has(index),
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
    serializedSize(normalized.recalled_memories),
    serializedSize(normalized.usage),
    serializedSize(normalized.goal_completion_receipt),
    serializedSize(normalized.handoff_reply),
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
