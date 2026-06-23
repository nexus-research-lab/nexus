import {
  type AssistantMessage,
  type ContentBlock,
  type ImageContent,
  type Message,
  type StreamMessage,
  type ThinkingContent,
  type TextContent,
} from '@/types';

function is_stream_renderable_block(
  block: StreamMessage['content_block'],
): block is TextContent | ThinkingContent | ImageContent {
  return block?.type === 'text' ||
    block?.type === 'thinking' ||
    block?.type === 'image';
}

function normalize_assistant_messages(messages: Message[]): Message[] {
  let has_changes = false;
  const next_messages = messages.map((message) => {
    if (message.role !== 'assistant') {
      return message;
    }

    const normalized_message = normalize_assistant_message(message);
    if (
      normalized_message.stream_status === message.stream_status
      && normalized_message.is_complete === message.is_complete
    ) {
      return message;
    }

    has_changes = true;
    return normalized_message;
  });

  return has_changes ? next_messages : messages;
}

/**
 * 按 message_id 压缩消息列表。
 *
 * 中文说明：
 * 前端消息会同时来自历史加载、WebSocket 完整消息、流式 patch、本地 optimistic。
 * 这些通道在重连和 reload 交错时，可能短暂把同一条业务消息重复带进来。
 * 这里建立消息状态层的硬约束：message_id 在内存里必须唯一。
 * assistant 快照会复用同一个 message_id 分批补充 content block，去重时必须按块身份合并。
 */
export function dedupe_messages_by_id(messages: Message[]): Message[] {
  if (messages.length <= 1) {
    return messages;
  }

  const last_index_by_id = new Map<string, number>();
  const message_by_id = new Map<string, Message>();
  let has_duplicates = false;

  messages.forEach((message, index) => {
    if (last_index_by_id.has(message.message_id)) {
      has_duplicates = true;
    }
    last_index_by_id.set(message.message_id, index);
    const existing_message = message_by_id.get(message.message_id);
    message_by_id.set(
      message.message_id,
      existing_message
        ? merge_message_by_id(existing_message, message)
        : message,
    );
  });

  if (!has_duplicates) {
    return messages;
  }

  const next_messages: Message[] = [];
  messages.forEach((message, index) => {
    if (last_index_by_id.get(message.message_id) !== index) {
      return;
    }
    next_messages.push(message_by_id.get(message.message_id) ?? message);
  });
  return next_messages;
}

/**
 * 将后端 assistant 快照统一归一化为前端运行态语义。
 *
 * 中文说明：
 * 后端的 is_complete 主要服务于持久化与非 Web 渠道发送，不等价于“这一轮已经结束”。
 * assistant turn 自身是否收口可以看 stop_reason / 显式 stream_status，
 * 但整轮 round 的结束必须以后端推送的 round_status 为准。
 */
export function normalize_assistant_message(incoming: AssistantMessage): AssistantMessage {
  return {
    ...incoming,
    stream_status: incoming.stream_status ?? (
      incoming.stop_reason || incoming.is_complete ? 'done' : 'streaming'
    ),
  };
}

/**
 * 按 message_id 合并完整消息。
 */
export function upsert_message(messages: Message[], incoming: Message): Message[] {
  const normalized_incoming = (
    incoming.role === 'assistant'
      ? normalize_assistant_message(incoming)
      : incoming
  );
  const existingIndex = messages.findIndex(
    (message) => message.message_id === normalized_incoming.message_id,
  );
  if (existingIndex === -1) {
    return normalize_assistant_messages(
      dedupe_messages_by_id([...messages, normalized_incoming]),
    );
  }

  const nextMessages = [...messages];
  nextMessages[existingIndex] = merge_message_by_id(
    nextMessages[existingIndex],
    normalized_incoming,
  );
  return normalize_assistant_messages(dedupe_messages_by_id(nextMessages));
}

function merge_message_by_id(existing: Message, incoming: Message): Message {
  if (existing.role === 'assistant' && incoming.role === 'assistant') {
    return merge_assistant_message(existing, incoming);
  }
  return incoming;
}

function merge_assistant_message(
  existing: AssistantMessage,
  incoming: AssistantMessage,
): AssistantMessage {
  const normalized_existing = normalize_assistant_message(existing);
  const normalized_incoming = normalize_assistant_message(incoming);
  return normalize_assistant_message({
    ...normalized_existing,
    ...normalized_incoming,
    content: merge_assistant_content_blocks(
      normalized_existing.content,
      normalized_incoming.content,
    ),
    result_summary: normalized_incoming.result_summary ?? normalized_existing.result_summary,
    usage: normalized_incoming.usage ?? normalized_existing.usage,
    stop_reason: normalized_incoming.stop_reason ?? normalized_existing.stop_reason,
    is_complete: normalized_incoming.is_complete ?? normalized_existing.is_complete,
    stream_status: normalized_incoming.stream_status ?? normalized_existing.stream_status,
  });
}

function merge_assistant_content_blocks(
  existing_blocks: ContentBlock[],
  incoming_blocks: ContentBlock[],
): ContentBlock[] {
  if (existing_blocks.length === 0) {
    return [...incoming_blocks];
  }
  if (incoming_blocks.length === 0) {
    return [...existing_blocks];
  }

  const merged_blocks = [...existing_blocks];
  const index_by_key = new Map<string, number>();
  merged_blocks.forEach((block, index) => {
    const key = assistant_content_block_key(block);
    if (key && !index_by_key.has(key)) {
      index_by_key.set(key, index);
    }
  });

  for (const incoming_block of incoming_blocks) {
    const text_block_index = find_mergeable_text_block_index(merged_blocks, incoming_block);
    if (text_block_index !== -1) {
      merged_blocks[text_block_index] = incoming_block;
      continue;
    }

    const key = assistant_content_block_key(incoming_block);
    const existing_index = key ? index_by_key.get(key) : undefined;
    if (existing_index !== undefined) {
      merged_blocks[existing_index] = incoming_block;
      continue;
    }
    if (key) {
      index_by_key.set(key, merged_blocks.length);
    }
    merged_blocks.push(incoming_block);
  }

  return merged_blocks;
}

function find_mergeable_text_block_index(
  blocks: ContentBlock[],
  incoming_block: ContentBlock,
): number {
  if (incoming_block.type !== 'text') {
    return -1;
  }
  for (let index = blocks.length - 1; index >= 0; index -= 1) {
    const current_block = blocks[index];
    if (current_block.type !== 'text') {
      continue;
    }
    if (
      current_block.text === incoming_block.text ||
      current_block.text.startsWith(incoming_block.text) ||
      incoming_block.text.startsWith(current_block.text)
    ) {
      return index;
    }
  }
  return -1;
}

function assistant_content_block_key(block: ContentBlock): string | null {
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
      return image_content_block_key(block);
    default:
      return null;
  }
}

function image_content_block_key(block: ImageContent): string | null {
  const raw_key = (
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
  return raw_key ? `image:${raw_key}` : null;
}

function json_equal(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

/**
 * 将流式增量应用到当前消息列表。
 */
export function apply_stream_message(messages: Message[], event: StreamMessage): Message[] {
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
  const stop_reason = event.message?.stop_reason || assistantMessage.stop_reason;
  const is_terminal_stream_event = event.type === 'message_stop';
  const next_model = event.message?.model || assistantMessage.model;
  const next_is_complete = stop_reason || is_terminal_stream_event ? true : assistantMessage.is_complete;
  const next_stream_status = stop_reason || is_terminal_stream_event ? 'done' : 'streaming';
  const next_usage = event.usage || assistantMessage.usage;
  const nextMessage: AssistantMessage = {
    ...assistantMessage,
    model: next_model,
    stop_reason,
    is_complete: next_is_complete,
    stream_status: next_stream_status,
    usage: next_usage,
    content: [...assistantMessage.content],
  };
  let changed =
    next_model !== assistantMessage.model ||
    stop_reason !== assistantMessage.stop_reason ||
    next_is_complete !== assistantMessage.is_complete ||
    next_stream_status !== assistantMessage.stream_status ||
    !json_equal(next_usage, assistantMessage.usage);

  if (
    (event.type === 'content_block_start' || event.type === 'content_block_delta') &&
    typeof event.index === 'number' &&
    is_stream_renderable_block(event.content_block)
  ) {
    const streamBlock = event.content_block;
    while (nextMessage.content.length <= event.index) {
      nextMessage.content.push({ type: 'text', text: '' });
      changed = true;
    }
    if (!json_equal(nextMessage.content[event.index], streamBlock)) {
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
export function sort_messages(messages: Message[]): Message[] {
  const unique_messages = dedupe_messages_by_id(messages);
  return normalize_assistant_messages(
    [...unique_messages].sort((left, right) => left.timestamp - right.timestamp),
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
export function merge_loaded_messages(
  loaded_messages: Message[],
  local_messages: Message[],
): Message[] {
  const unique_loaded_messages = dedupe_messages_by_id(loaded_messages);
  if (local_messages.length === 0) {
    return sort_messages(unique_loaded_messages);
  }

  const loaded_message_ids = new Set(
    unique_loaded_messages.map((message) => message.message_id),
  );
  const merged_messages = [...unique_loaded_messages];

  for (const local_message of local_messages) {
    if (!loaded_message_ids.has(local_message.message_id)) {
      merged_messages.push(local_message);
    }
  }

  return sort_messages(dedupe_messages_by_id(merged_messages));
}
