import { AssistantMessage, Message, StreamMessage, ThinkingContent, TextContent } from '@/types';

function isStreamRenderableBlock(
  block: StreamMessage['content_block'],
): block is TextContent | ThinkingContent {
  return block?.type === 'text' || block?.type === 'thinking';
}

function normalize_completed_assistant_states(messages: Message[]): Message[] {
  const completed_round_ids = new Set(
    messages
      .filter((message) => message.role === 'result')
      .map((message) => message.round_id),
  );

  let has_changes = false;
  const next_messages = messages.map((message) => {
    if (message.role !== 'assistant') {
      return message;
    }

    if (
      !completed_round_ids.has(message.round_id) ||
      (message.is_complete && message.stream_status === 'done')
    ) {
      return message;
    }

    has_changes = true;
    return {
      ...message,
      is_complete: true,
      stream_status: 'done' as const,
    };
  });

  return has_changes ? next_messages : messages;
}

/**
 * 将后端 assistant 快照统一归一化为前端运行态语义。
 *
 * 中文说明：
 * 后端的 is_complete 主要服务于持久化与非 Web 渠道发送，不等价于“这一轮已经结束”。
 * assistant turn 自身是否收口可以看 stop_reason / 显式 stream_status，
 * 但整轮 round 的结束必须以后端推送的 round_status 为准。
 */
export function normalizeAssistantMessage(incoming: AssistantMessage): AssistantMessage {
  return {
    ...incoming,
    stream_status: incoming.stream_status ?? (
      incoming.stop_reason ? 'done' : 'streaming'
    ),
  };
}

/**
 * 按 message_id 合并完整消息。
 */
export function upsertMessage(messages: Message[], incoming: Message): Message[] {
  const normalized_incoming = (
    incoming.role === 'assistant'
      ? normalizeAssistantMessage(incoming)
      : incoming
  );
  const existingIndex = messages.findIndex(
    (message) => message.message_id === normalized_incoming.message_id,
  );
  if (existingIndex === -1) {
    return normalize_completed_assistant_states([...messages, normalized_incoming]);
  }

  const nextMessages = [...messages];
  nextMessages[existingIndex] = normalized_incoming;
  return normalize_completed_assistant_states(nextMessages);
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
    isStreamRenderableBlock(event.content_block)
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
export function sortMessages(messages: Message[]): Message[] {
  return normalize_completed_assistant_states(
    [...messages].sort((left, right) => left.timestamp - right.timestamp),
  );
}
