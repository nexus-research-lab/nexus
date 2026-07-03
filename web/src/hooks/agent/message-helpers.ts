import {
  type AssistantMessage,
  type ContentBlock,
  type ImageContent,
  type Message,
  type StreamMessage,
  type ThinkingContent,
  type TextContent,
} from '@/types';

function isStreamRenderableBlock(
  block: StreamMessage['content_block'],
): block is TextContent | ThinkingContent | ImageContent {
  return block?.type === 'text' ||
    block?.type === 'thinking' ||
    block?.type === 'image';
}

function normalizeAssistantMessages(messages: Message[]): Message[] {
  let hasChanges = false;
  const nextMessages = messages.map((message) => {
    if (message.role !== 'assistant') {
      return message;
    }

    const normalizedMessage = normalizeAssistantMessage(message);
    if (
      normalizedMessage.stream_status === message.stream_status
      && normalizedMessage.is_complete === message.is_complete
    ) {
      return message;
    }

    hasChanges = true;
    return normalizedMessage;
  });

  return hasChanges ? nextMessages : messages;
}

/**
 * 按 messageId 压缩消息列表。
 *
 * 中文说明：
 * 前端消息会同时来自历史加载、WebSocket 完整消息、流式 patch、本地 optimistic。
 * 这些通道在重连和 reload 交错时，可能短暂把同一条业务消息重复带进来。
 * 这里建立消息状态层的硬约束：messageId 在内存里必须唯一。
 * assistant 快照会复用同一个 messageId 分批补充 content block，去重时必须按块身份合并。
 */
export function dedupeMessagesById(messages: Message[]): Message[] {
  if (messages.length <= 1) {
    return messages;
  }

  const lastIndexById = new Map<string, number>();
  const messageById = new Map<string, Message>();
  let hasDuplicates = false;

  messages.forEach((message, index) => {
    if (lastIndexById.has(message.message_id)) {
      hasDuplicates = true;
    }
    lastIndexById.set(message.message_id, index);
    const existingMessage = messageById.get(message.message_id);
    messageById.set(
      message.message_id,
      existingMessage
        ? mergeMessageById(existingMessage, message)
        : message,
    );
  });

  if (!hasDuplicates) {
    return messages;
  }

  const nextMessages: Message[] = [];
  messages.forEach((message, index) => {
    if (lastIndexById.get(message.message_id) !== index) {
      return;
    }
    nextMessages.push(messageById.get(message.message_id) ?? message);
  });
  return nextMessages;
}

/**
 * 将后端 assistant 快照统一归一化为前端运行态语义。
 *
 * 中文说明：
 * 后端的 isComplete 主要服务于持久化与非 Web 渠道发送，不等价于“这一轮已经结束”。
 * assistant turn 自身是否收口可以看 stopReason / 显式 streamStatus，
 * 但整轮 round 的结束必须以后端推送的 roundStatus 为准。
 */
export function normalizeAssistantMessage(incoming: AssistantMessage): AssistantMessage {
  return {
    ...incoming,
    stream_status: incoming.stream_status ?? (
      incoming.stop_reason || incoming.is_complete ? 'done' : 'streaming'
    ),
  };
}

/**
 * 按 messageId 合并完整消息。
 */
export function upsertMessage(messages: Message[], incoming: Message): Message[] {
  const normalizedIncoming = (
    incoming.role === 'assistant'
      ? normalizeAssistantMessage(incoming)
      : incoming
  );
  const existingIndex = messages.findIndex(
    (message) => message.message_id === normalizedIncoming.message_id,
  );
  if (existingIndex === -1) {
    return normalizeAssistantMessages(
      dedupeMessagesById([...messages, normalizedIncoming]),
    );
  }

  const nextMessages = [...messages];
  nextMessages[existingIndex] = mergeMessageById(
    nextMessages[existingIndex],
    normalizedIncoming,
  );
  return normalizeAssistantMessages(dedupeMessagesById(nextMessages));
}

function mergeMessageById(existing: Message, incoming: Message): Message {
  if (existing.role === 'assistant' && incoming.role === 'assistant') {
    return mergeAssistantMessage(existing, incoming);
  }
  return incoming;
}

function mergeAssistantMessage(
  existing: AssistantMessage,
  incoming: AssistantMessage,
): AssistantMessage {
  const normalizedExisting = normalizeAssistantMessage(existing);
  const normalizedIncoming = normalizeAssistantMessage(incoming);
  return normalizeAssistantMessage({
    ...normalizedExisting,
    ...normalizedIncoming,
    content: mergeAssistantContentBlocks(
      normalizedExisting.content,
      normalizedIncoming.content,
    ),
    result_summary: normalizedIncoming.result_summary ?? normalizedExisting.result_summary,
    usage: normalizedIncoming.usage ?? normalizedExisting.usage,
    stop_reason: normalizedIncoming.stop_reason ?? normalizedExisting.stop_reason,
    is_complete: normalizedIncoming.is_complete ?? normalizedExisting.is_complete,
    stream_status: normalizedIncoming.stream_status ?? normalizedExisting.stream_status,
  });
}

function mergeAssistantContentBlocks(
  existingBlocks: ContentBlock[],
  incomingBlocks: ContentBlock[],
): ContentBlock[] {
  if (existingBlocks.length === 0) {
    return [...incomingBlocks];
  }
  if (incomingBlocks.length === 0) {
    return [...existingBlocks];
  }

  const mergedBlocks = [...existingBlocks];
  const indexByKey = new Map<string, number>();
  mergedBlocks.forEach((block, index) => {
    const key = assistantContentBlockKey(block);
    if (key && !indexByKey.has(key)) {
      indexByKey.set(key, index);
    }
  });

  for (const incomingBlock of incomingBlocks) {
    const textBlockIndex = findMergeableTextBlockIndex(mergedBlocks, incomingBlock);
    if (textBlockIndex !== -1) {
      mergedBlocks[textBlockIndex] = incomingBlock;
      continue;
    }

    const key = assistantContentBlockKey(incomingBlock);
    const existingIndex = key ? indexByKey.get(key) : undefined;
    if (existingIndex !== undefined) {
      mergedBlocks[existingIndex] = incomingBlock;
      continue;
    }
    if (key) {
      indexByKey.set(key, mergedBlocks.length);
    }
    mergedBlocks.push(incomingBlock);
  }

  return mergedBlocks;
}

function findMergeableTextBlockIndex(
  blocks: ContentBlock[],
  incomingBlock: ContentBlock,
): number {
  if (incomingBlock.type !== 'text') {
    return -1;
  }
  for (let index = blocks.length - 1; index >= 0; index -= 1) {
    const currentBlock = blocks[index];
    if (currentBlock.type !== 'text') {
      continue;
    }
    if (
      currentBlock.text === incomingBlock.text ||
      currentBlock.text.startsWith(incomingBlock.text) ||
      incomingBlock.text.startsWith(currentBlock.text)
    ) {
      return index;
    }
  }
  return -1;
}

function assistantContentBlockKey(block: ContentBlock): string | null {
  switch (block.type) {
    case 'thinking':
      return 'thinking';
    case 'text':
      return `text:${block.text}`;
    case 'tool_use':
      return block.id ? `tool_use:${block.id}` : null;
    case 'tool_result':
      return block.tool_use_id ? `tool_result:${block.tool_use_id}` : null;
    case 'task_progress':
      return block.task_id ? `task_progress:${block.task_id}` : null;
    case 'workspace_file_artifact':
      if (block.id) {
        return `workspace_file_artifact:${block.id}`;
      }
      return `workspace_file_artifact:${block.path}:${block.operation ?? ''}`;
    case 'system_event':
      return [
        'system_event',
        block.source_message_id,
        block.subtype ?? '',
        block.tool_use_id ?? '',
        block.content,
      ].join(':');
    case 'tool_use_error':
      return `tool_use_error:${block.content}`;
    case 'image':
      return imageContentBlockKey(block);
    default:
      return null;
  }
}

function imageContentBlockKey(block: ImageContent): string | null {
  const rawKey = (
    block.path
    || block.url
    || block.uri
    || block.source?.path
    || block.source?.url
    || block.source?.uri
    || block.data
    || block.source?.data
    || null
  );
  return rawKey ? `image:${rawKey}` : null;
}

function jsonEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

/**
 * 将流式增量应用到当前消息列表。
 */
export function applyStreamMessage(messages: Message[], event: StreamMessage): Message[] {
  const existingIndex = messages.findIndex(
    (message) => message.role === 'assistant' && message.message_id === event.message_id,
  );

  if (event.type === 'message_start') {
    if (existingIndex !== -1) {
      return messages;
    }
    return [
      ...messages,
      {
        message_id: event.message_id,
        session_key: event.session_key,
        agent_id: event.agent_id,
        round_id: event.round_id,
        session_id: event.session_id,
        role: 'assistant',
        content: [],
        is_complete: false,
        stream_status: 'streaming',
        model: event.message?.model,
        timestamp: event.timestamp,
      },
    ];
  }

  if (existingIndex === -1) {
    return messages;
  }

  const assistantMessage = messages[existingIndex] as AssistantMessage;
  const stopReason = event.message?.stop_reason || assistantMessage.stop_reason;
  const isTerminalStreamEvent = event.type === 'message_stop';
  const nextModel = event.message?.model || assistantMessage.model;
  const nextIsComplete = stopReason || isTerminalStreamEvent ? true : assistantMessage.is_complete;
  const nextStreamStatus = stopReason || isTerminalStreamEvent ? 'done' : 'streaming';
  const nextUsage = event.usage || assistantMessage.usage;
  const nextMessage: AssistantMessage = {
    ...assistantMessage,
    model: nextModel,
    stop_reason: stopReason,
    is_complete: nextIsComplete,
    stream_status: nextStreamStatus,
    usage: nextUsage,
    content: [...assistantMessage.content],
  };
  let changed =
    nextModel !== assistantMessage.model ||
    stopReason !== assistantMessage.stop_reason ||
    nextIsComplete !== assistantMessage.is_complete ||
    nextStreamStatus !== assistantMessage.stream_status ||
    !jsonEqual(nextUsage, assistantMessage.usage);

  if (
    (event.type === 'content_block_start' || event.type === 'content_block_delta') &&
    typeof event.index === 'number' &&
    isStreamRenderableBlock(event.content_block)
  ) {
    const streamBlock = event.content_block;
    while (nextMessage.content.length <= event.index) {
      nextMessage.content.push({ type: 'text', text: '' });
      changed = true;
    }
    if (!jsonEqual(nextMessage.content[event.index], streamBlock)) {
      nextMessage.content[event.index] = streamBlock;
      changed = true;
    }
  }

  // 重放或重复到达的 stream patch 不应触发 React 状态更新。
  if (!changed) {
    return messages;
  }

  const nextMessages = [...messages];
  nextMessages[existingIndex] = nextMessage;
  return nextMessages;
}

/**
 * 按时间戳排序消息，保证历史与实时消息顺序稳定。
 */
export function sortMessages(messages: Message[]): Message[] {
  const uniqueMessages = dedupeMessagesById(messages);
  return normalizeAssistantMessages(
    [...uniqueMessages].sort((left, right) => left.timestamp - right.timestamp),
  );
}

/**
 * 合并服务端快照与本地消息，保留尚未落库的本地 optimistic 消息。
 *
 * 规则：
 * 1. 同 message_id 的消息始终以服务端快照为准；
 * 2. 仅把服务端中不存在的本地消息补回去；
 * 3. 最终统一排序，避免 session 首屏加载把用户刚发出的消息冲掉。
 */
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
  const mergedMessages = [...uniqueLoadedMessages];

  for (const localMessage of localMessages) {
    if (!loadedMessageIds.has(localMessage.message_id)) {
      mergedMessages.push(localMessage);
    }
  }

  return sortMessages(dedupeMessagesById(mergedMessages));
}
