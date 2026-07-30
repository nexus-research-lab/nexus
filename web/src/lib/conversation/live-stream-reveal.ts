/**
 * INPUT: 仅由当前 WebSocket stream 创建的 Assistant 消息与 ContentBlock。
 * OUTPUT: 不可由历史协议伪造的本地 Symbol 标记，以及首帧提交后的精确清理。
 * POS: transport/reducer 与 Markdown 视图之间的 live-first-frame 语义边界；不持久化、不重播历史。
 */
import type { ContentBlock } from "@/types/conversation/message/content";
import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";

const LIVE_STREAM_ASSISTANT = Symbol("nexus.live-stream-assistant");
const LIVE_STREAM_REVEAL = Symbol("nexus.live-stream-reveal");

type LiveStreamAssistant = AssistantMessage & {
  [LIVE_STREAM_ASSISTANT]?: true;
};

type LiveStreamRevealBlock = ContentBlock & {
  [LIVE_STREAM_REVEAL]?: true;
};

export function markLiveStreamAssistant(
  message: AssistantMessage,
): AssistantMessage {
  return ({
    ...message,
    [LIVE_STREAM_ASSISTANT]: true,
  } as LiveStreamAssistant);
}

export function isLiveStreamAssistant(
  message: AssistantMessage,
): boolean {
  return (message as LiveStreamAssistant)[LIVE_STREAM_ASSISTANT] === true;
}

export function markLiveStreamRevealBlock(
  block: ContentBlock,
): ContentBlock {
  if (!isSmoothRevealBlock(block)) {
    return block;
  }
  return ({
    ...block,
    [LIVE_STREAM_REVEAL]: true,
  } as LiveStreamRevealBlock);
}

export function preserveLiveStreamRevealMarker(
  current: ContentBlock,
  incoming: ContentBlock,
): ContentBlock {
  return hasLiveStreamRevealMarker(current)
    ? markLiveStreamRevealBlock(incoming)
    : incoming;
}

export function hasLiveStreamRevealMarker(
  block: ContentBlock,
): boolean {
  return (
    isSmoothRevealBlock(block)
    && (block as LiveStreamRevealBlock)[LIVE_STREAM_REVEAL] === true
  );
}

export function clearLiveStreamRevealMarkers(
  messages: Message[],
  messageIds: ReadonlySet<string>,
): Message[] {
  if (messageIds.size === 0) {
    return messages;
  }

  let messagesChanged = false;
  const nextMessages = messages.map((message) => {
    if (
      message.role !== "assistant"
      || !messageIds.has(message.message_id)
    ) {
      return message;
    }

    let contentChanged = false;
    const content = message.content.map((block) => {
      if (!hasLiveStreamRevealMarker(block)) {
        return block;
      }
      const nextBlock = { ...block } as LiveStreamRevealBlock;
      delete nextBlock[LIVE_STREAM_REVEAL];
      contentChanged = true;
      return nextBlock;
    });
    if (!contentChanged) {
      return message;
    }
    messagesChanged = true;
    return { ...message, content };
  });

  return messagesChanged ? nextMessages : messages;
}

export function hasVisibleSmoothRevealContent(
  block: ContentBlock | undefined,
): boolean {
  if (block?.type === "text") {
    return Boolean(block.text.trim());
  }
  if (block?.type === "thinking") {
    return Boolean(block.thinking.trim());
  }
  return false;
}

function isSmoothRevealBlock(
  block: ContentBlock,
): block is Extract<ContentBlock, { type: "text" | "thinking" }> {
  return block.type === "text" || block.type === "thinking";
}
