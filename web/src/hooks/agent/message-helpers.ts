import { AssistantMessage, Message, StreamMessage, ThinkingContent, TextContent } from '@/types';

function is_stream_renderable_block(
  block: StreamMessage['content_block'],
): block is TextContent | ThinkingContent {
  return block?.type === 'text' || block?.type === 'thinking';
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
 * 按 message_id 压缩消息列表，统一保留最后一次写入。
 *
 * 中文说明：
 * 前端消息会同时来自历史加载、WebSocket 完整消息、流式 patch、本地 optimistic。
 * 这些通道在重连和 reload 交错时，可能短暂把同一条业务消息重复带进来。
 * 这里建立消息状态层的硬约束：message_id 在内存里必须唯一。
 */
export function dedupe_messages_by_id(messages: Message[]): Message[] {
  if (messages.length <= 1) {
    return messages;
  }

  const last_index_by_id = new Map<string, number>();
  let has_duplicates = false;

  messages.forEach((message, index) => {
    if (last_index_by_id.has(message.message_id)) {
      has_duplicates = true;
    }
    last_index_by_id.set(message.message_id, index);
  });

  if (!has_duplicates) {
    return messages;
  }

  const next_messages: Message[] = [];
  messages.forEach((message, index) => {
    if (last_index_by_id.get(message.message_id) !== index) {
      return;
    }
    next_messages.push(message);
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
  nextMessages[existingIndex] = normalized_incoming;
  return normalize_assistant_messages(dedupe_messages_by_id(nextMessages));
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
  const nextMessage: AssistantMessage = {
    ...assistantMessage,
    model: event.message?.model || assistantMessage.model,
    stop_reason,
    is_complete: stop_reason || is_terminal_stream_event ? true : assistantMessage.is_complete,
    stream_status: stop_reason || is_terminal_stream_event ? 'done' : 'streaming',
    usage: event.usage || assistantMessage.usage,
    content: [...assistantMessage.content],
  };

  if (
    (event.type === 'content_block_start' || event.type === 'content_block_delta') &&
    typeof event.index === 'number' &&
    is_stream_renderable_block(event.content_block)
  ) {
    const streamBlock = event.content_block;
    while (nextMessage.content.length <= event.index) {
      nextMessage.content.push({ type: 'text', text: '' });
    }
    nextMessage.content[event.index] = streamBlock;
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
