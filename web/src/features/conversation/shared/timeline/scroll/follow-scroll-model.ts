import type { Message } from "@/types/conversation/message/entity";

const BOTTOM_THRESHOLD_PX = 80;

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

export function getScrollBottomTop(element: HTMLDivElement): number {
  return Math.max(0, element.scrollHeight - element.clientHeight);
}

export function isNearScrollBottom(element: HTMLDivElement): boolean {
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

export function buildConversationScrollContentKey(
  sessionKey: string | null,
  messages: readonly Message[],
): string {
  const firstMessage = projectScrollMessageIdentity(messages[0]);
  const latestMessage = projectScrollMessageIdentity(messages.at(-1));

  return [
    sessionKey ?? "",
    messages.length,
    firstMessage.messageId,
    latestMessage.messageId,
    latestMessage.timestamp,
    latestMessage.role,
    latestMessage.streamStatus,
  ].join("\u001f");
}
