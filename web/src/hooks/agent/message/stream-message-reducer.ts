import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";
import type {
  ImageContent,
  TextContent,
  ThinkingContent,
} from "@/types/conversation/message/content";
import type { StreamMessage } from "@/types/conversation/message/event";

type StreamRenderableBlock = TextContent | ThinkingContent | ImageContent;

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
  const nextMessage = applyStreamEvent(currentMessage, event);
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
  return {
    agent_id: event.agent_id,
    content: [],
    is_complete: false,
    message_id: event.message_id,
    model: event.message?.model,
    role: "assistant",
    round_id: event.round_id,
    session_id: event.session_id,
    session_key: event.session_key,
    stream_status: "streaming",
    timestamp: event.timestamp,
  };
}

function applyStreamEvent(
  currentMessage: AssistantMessage,
  event: StreamMessage,
): AssistantMessage {
  const stopReason = event.message?.stop_reason || currentMessage.stop_reason;
  const isTerminal = event.type === "message_stop" || Boolean(stopReason);
  const nextMessage: AssistantMessage = {
    ...currentMessage,
    content: [...currentMessage.content],
    is_complete: isTerminal ? true : currentMessage.is_complete,
    model: event.message?.model || currentMessage.model,
    stop_reason: stopReason,
    stream_status: isTerminal ? "done" : "streaming",
    usage: event.usage || currentMessage.usage,
  };

  const contentChanged = applyStreamContentBlock(nextMessage, event);
  return contentChanged || hasMetadataChanged(currentMessage, nextMessage)
    ? nextMessage
    : currentMessage;
}

function applyStreamContentBlock(
  message: AssistantMessage,
  event: StreamMessage,
): boolean {
  if (!isIndexedContentEvent(event)) {
    return false;
  }

  let changed = false;
  while (message.content.length <= event.index) {
    message.content.push({ type: "text", text: "" });
    changed = true;
  }
  if (!jsonEqual(message.content[event.index], event.content_block)) {
    message.content[event.index] = event.content_block;
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
  return (
    block?.type === "text" ||
    block?.type === "thinking" ||
    block?.type === "image"
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
