import type { Message } from "@/types/conversation/message";

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
