import type { Message } from "@/types/conversation/message";

const BOTTOM_THRESHOLD_PX = 80;

export function getScrollBottomTop(element: HTMLDivElement): number {
  return Math.max(0, element.scrollHeight - element.clientHeight);
}

export function isNearScrollBottom(element: HTMLDivElement): boolean {
  return getScrollBottomTop(element) - element.scrollTop <= BOTTOM_THRESHOLD_PX;
}

export function buildConversationScrollContentKey(
  sessionKey: string | null,
  messages: readonly Message[],
): string {
  const firstMessage = messages[0] ?? null;
  const latestMessage = messages[messages.length - 1] ?? null;
  const latestAssistantStatus =
    latestMessage?.role === "assistant"
      ? (latestMessage.stream_status ?? "")
      : "";

  return [
    sessionKey ?? "",
    messages.length,
    firstMessage?.message_id ?? "",
    latestMessage?.message_id ?? "",
    latestMessage?.timestamp ?? 0,
    latestMessage?.role ?? "",
    latestAssistantStatus,
  ].join("\u001f");
}
