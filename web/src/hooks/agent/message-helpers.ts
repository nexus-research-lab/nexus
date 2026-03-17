import { AssistantMessage, Message, StreamMessage, ThinkingContent, TextContent } from '@/types';

function isStreamRenderableBlock(
  block: StreamMessage['content_block'],
): block is TextContent | ThinkingContent {
  return block?.type === 'text' || block?.type === 'thinking';
}

/**
 * 按 message_id 合并完整消息。
 */
export function upsertMessage(messages: Message[], incoming: Message): Message[] {
  const existingIndex = messages.findIndex(
    (message) => message.message_id === incoming.message_id,
  );
  if (existingIndex === -1) {
    return [...messages, incoming];
  }

  const nextMessages = [...messages];
  nextMessages[existingIndex] = incoming;
  return nextMessages;
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
        model: event.message?.model,
        timestamp: event.timestamp,
      },
    ];
  }

  if (existingIndex === -1) {
    return messages;
  }

  const assistantMessage = messages[existingIndex] as AssistantMessage;
  const nextMessage: AssistantMessage = {
    ...assistantMessage,
    model: event.message?.model || assistantMessage.model,
    stop_reason: event.message?.stop_reason || assistantMessage.stop_reason,
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
  return [...messages].sort((left, right) => left.timestamp - right.timestamp);
}
