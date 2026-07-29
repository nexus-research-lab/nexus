/**
 * INPUT: 已归并的 Assistant 快照与按帧排队的 runtime stream 事件。
 * OUTPUT: 保持活动与终态内容单调、只向活动消息应用非回退索引内容块的消息集合。
 * POS: 实时完整快照与延迟 stream patch 汇合前的单消息竞态保护层。
 */
import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";
import type {
  ContentBlock,
} from "@/types/conversation/message/content";
import type { StreamMessage } from "@/types/conversation/message/event";
import {
  isLiveStreamAssistant,
  markLiveStreamAssistant,
  markLiveStreamRevealBlock,
  preserveLiveStreamRevealMarker,
} from "@/lib/conversation/live-stream-reveal";

type StreamRenderableBlock = ContentBlock;

interface StreamMetadataProjection {
  is_complete?: boolean;
  model?: string;
  stop_reason?: AssistantMessage["stop_reason"];
  stream_status: AssistantMessage["stream_status"];
  usage?: AssistantMessage["usage"];
}

export function applyStreamMessage(
  messages: Message[],
  event: StreamMessage,
): Message[] {
  const existingIndex = messages.findIndex(
    (message) =>
      message.role === "assistant" && message.message_id === event.message_id,
  );
  if (event.type === "message_start") {
    return existingIndex === -1
      ? [...messages, createStreamingAssistantMessage(event)]
      : messages;
  }
  if (existingIndex === -1) {
    return messages;
  }

  const currentMessage = messages[existingIndex] as AssistantMessage;
  const currentWasTerminal = isTerminalAssistantMessage(currentMessage);
  const nextMessage = applyStreamEvent(currentMessage, event);
  if (!currentWasTerminal && isTerminalEmptyAssistant(nextMessage)) {
    return messages.filter((_, index) => index !== existingIndex);
  }
  if (nextMessage === currentMessage) {
    return messages;
  }
  const nextMessages = [...messages];
  nextMessages[existingIndex] = nextMessage;
  return nextMessages;
}

function createStreamingAssistantMessage(
  event: StreamMessage,
): AssistantMessage {
  return markLiveStreamAssistant({
    agent_id: event.agent_id,
    ...(event.agent_round_id
      ? { agent_round_id: event.agent_round_id }
      : {}),
    content: [],
    ...(event.conversation_id
      ? { conversation_id: event.conversation_id }
      : {}),
    is_complete: false,
    message_id: event.message_id,
    model: event.message?.model,
    role: "assistant",
    round_id: event.round_id,
    ...(event.room_id ? { room_id: event.room_id } : {}),
    session_id: event.session_id,
    session_key: event.session_key,
    ...(event.parent_tool_use_id
      ? {
        parent_id: event.parent_tool_use_id,
        parent_tool_use_id: event.parent_tool_use_id,
      }
      : {}),
    stream_status: "streaming",
    timestamp: event.timestamp,
  });
}

function applyStreamEvent(
  currentMessage: AssistantMessage,
  event: StreamMessage,
): AssistantMessage {
  if (isTerminalAssistantMessage(currentMessage)) {
    // 排队中的 stream 事件整体早于已提交的终态 message 快照；内容与
    // metadata 都必须冻结，否则 cancelled/error 会被旧 delta 改回 streaming。
    return currentMessage;
  }

  const nextMessage: AssistantMessage = {
    ...currentMessage,
    content: [...currentMessage.content],
    ...projectStreamMetadata(currentMessage, event),
  };

  const contentChanged = applyStreamContentBlock(nextMessage, event);
  return contentChanged || hasMetadataChanged(currentMessage, nextMessage)
    ? nextMessage
    : currentMessage;
}

function isTerminalAssistantMessage(message: AssistantMessage): boolean {
  return (
    message.stream_status === "done"
    || message.stream_status === "cancelled"
    || message.stream_status === "error"
    || message.is_complete === true
    || Boolean(message.stop_reason)
    || Boolean(message.result_summary)
  );
}

function projectStreamMetadata(
  currentMessage: AssistantMessage,
  event: StreamMessage,
): StreamMetadataProjection {
  const stopReason = event.message?.stop_reason || currentMessage.stop_reason;
  const isTerminal = event.type === "message_stop" || Boolean(stopReason);
  return {
    is_complete: isTerminal ? true : currentMessage.is_complete,
    model: event.message?.model || currentMessage.model,
    stop_reason: stopReason,
    stream_status: isTerminal ? "done" : "streaming",
    usage: event.usage || currentMessage.usage,
  };
}

function applyStreamContentBlock(
  message: AssistantMessage,
  event: StreamMessage,
): boolean {
  if (!isIndexedContentEvent(event)) {
    return false;
  }

  const hadCurrentBlock = message.content.length > event.index;
  let changed = false;
  while (message.content.length <= event.index) {
    message.content.push({ type: "text", text: "" });
    changed = true;
  }
  const currentBlock = message.content[event.index];
  if (isRegressiveStreamBlock(currentBlock, event.content_block)) {
    return changed;
  }
  const isNewLiveBlock = (
    isLiveStreamAssistant(message)
    && (!hadCurrentBlock || currentBlock.type !== event.content_block.type)
  );
  const incomingBlock = isNewLiveBlock
    ? markLiveStreamRevealBlock(event.content_block)
    : preserveLiveStreamRevealMarker(currentBlock, event.content_block);
  if (isNewLiveBlock || !jsonEqual(currentBlock, incomingBlock)) {
    message.content[event.index] = incomingBlock;
    changed = true;
  }
  return changed;
}

function isIndexedContentEvent(
  event: StreamMessage,
): event is StreamMessage & {
  content_block: StreamRenderableBlock;
  index: number;
} {
  return (
    (event.type === "content_block_start" ||
      event.type === "content_block_delta") &&
    typeof event.index === "number" &&
    event.index >= 0 &&
    isStreamRenderableBlock(event.content_block)
  );
}

function isStreamRenderableBlock(
  block: StreamMessage["content_block"],
): block is StreamRenderableBlock {
  return Boolean(block && typeof block.type === "string");
}

function isRegressiveStreamBlock(
  current: StreamRenderableBlock,
  incoming: StreamRenderableBlock,
): boolean {
  if (current.type !== incoming.type) {
    return false;
  }
  if (current.type === "text" && incoming.type === "text") {
    return (
      current.text.length > incoming.text.length
      && current.text.startsWith(incoming.text)
    );
  }
  if (current.type === "thinking" && incoming.type === "thinking") {
    return (
      current.thinking.length > incoming.thinking.length
      && current.thinking.startsWith(incoming.thinking)
    );
  }
  // 其他累计块（尤其 tool input）没有统一文本字段；旧 patch 的序列化负载
  // 更小时不得覆盖较新的完整 snapshot。等长修正仍允许进入。
  return JSON.stringify(current).length > JSON.stringify(incoming).length;
}

function isTerminalEmptyAssistant(message: AssistantMessage): boolean {
  if (message.stream_status !== "done" || message.content.length > 1) {
    return false;
  }
  if (message.content.length === 0) {
    return true;
  }
  const block = message.content[0];
  return block.type === "text" && (
    !block.text.trim()
    || block.text.trim() === "(no content)"
    || block.text.trim() === "[Request interrupted by user for tool use]"
  );
}

function hasMetadataChanged(
  current: AssistantMessage,
  next: AssistantMessage,
): boolean {
  return (
    next.model !== current.model ||
    next.stop_reason !== current.stop_reason ||
    next.is_complete !== current.is_complete ||
    next.stream_status !== current.stream_status ||
    !jsonEqual(next.usage, current.usage)
  );
}

function jsonEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}
