/**
 * INPUT: 会话消息快照与滚动容器测量值。
 * OUTPUT: 溢出/贴底判定，以及覆盖并行流式回复正文增长的稳定内容版本。
 * POS: DM、Room 与 Thread 跟随滚动的纯模型真相源。
 */
import type { Message } from "@/types/conversation/message/entity";

const BOTTOM_THRESHOLD_PX = 80;
const SCROLL_OVERFLOW_TOLERANCE_PX = 1;

interface ScrollMetrics {
  clientHeight: number;
  scrollHeight: number;
  scrollTop: number;
}

interface ScrollMessageIdentity {
  messageId: string;
  role: Message["role"] | "";
  streamStatus: string;
  timestamp: number;
}

const EMPTY_SCROLL_MESSAGE_IDENTITY: ScrollMessageIdentity = {
  messageId: "",
  role: "",
  streamStatus: "",
  timestamp: 0,
};

export function getScrollBottomTop(element: ScrollMetrics): number {
  return Math.max(0, element.scrollHeight - element.clientHeight);
}

export function hasScrollableOverflow(element: ScrollMetrics): boolean {
  return getScrollBottomTop(element) > SCROLL_OVERFLOW_TOLERANCE_PX;
}

export function isNearScrollBottom(element: ScrollMetrics): boolean {
  return getScrollBottomTop(element) - element.scrollTop <= BOTTOM_THRESHOLD_PX;
}

function projectScrollMessageIdentity(
  message: Message | undefined,
): ScrollMessageIdentity {
  if (!message) {
    return EMPTY_SCROLL_MESSAGE_IDENTITY;
  }
  return {
    messageId: message.message_id,
    role: message.role,
    streamStatus:
      message.role === "assistant" ? message.stream_status ?? "" : "",
    timestamp: message.timestamp,
  };
}

function projectAssistantScrollRevision(message: Message): string | null {
  if (message.role !== "assistant") {
    return null;
  }
  let renderedLength = message.result_summary?.result?.length ?? 0;
  for (const block of message.content) {
    switch (block.type) {
      case "text":
        renderedLength += block.text.length;
        break;
      case "thinking":
        renderedLength += block.thinking.length;
        break;
      case "tool_use_error":
      case "system_event":
        renderedLength += block.content.length;
        break;
      case "task_progress":
        renderedLength += block.description.length;
        break;
      case "search_result":
        renderedLength +=
          (block.title?.length ?? 0)
          + (block.snippet?.length ?? 0);
        break;
      default:
        // 非文本块的增删仍会改变正文高度；动态大负载只计块身份，避免逐 token 序列化。
        renderedLength += 1;
        break;
    }
  }
  return [
    message.message_id,
    message.agent_round_id ?? "",
    message.stream_status ?? "",
    message.content.length,
    renderedLength,
  ].join(":");
}

export function buildConversationScrollContentKey(
  sessionKey: string | null,
  messages: readonly Message[],
): string {
  const firstMessage = projectScrollMessageIdentity(messages[0]);
  const latestMessage = projectScrollMessageIdentity(messages.at(-1));
  const assistantRevisions = messages.flatMap((message) => {
    const revision = projectAssistantScrollRevision(message);
    return revision ? [revision] : [];
  });

  return [
    sessionKey ?? "",
    messages.length,
    firstMessage.messageId,
    latestMessage.messageId,
    latestMessage.timestamp,
    latestMessage.role,
    latestMessage.streamStatus,
    ...assistantRevisions,
  ].join("\u001f");
}
