import type { AssistantMessage, Message } from "@/types/conversation/message";
import { stripRoomControlMarkers } from "./message/item/message-item-support";

// 终态轮次里 assistant 仅剩无回复标记（剥离后无文本、无工具/图片等块）时，
// 视为纯 no-reply，不在时间线显示。保守判定：任何工具/非文本块都算可见输出。
function isBlankNoReplyRound(messages: Message[]): boolean {
  const assistants = messages.filter(
    (message): message is AssistantMessage => message.role === "assistant",
  );
  if (assistants.length === 0) {
    return false;
  }
  for (const assistant of assistants) {
    for (const block of assistant.content) {
      if (block.type === "thinking") {
        continue;
      }
      if (block.type === "text") {
        if (stripRoomControlMarkers(block.text)) {
          return false;
        }
        continue;
      }
      return false;
    }
    const resultText = assistant.result_summary?.result;
    if (resultText && stripRoomControlMarkers(resultText)) {
      return false;
    }
  }
  return true;
}

/** 时间线除历史消息外，也要显示已启动但尚未产生消息的运行轮次。 */
export function buildTimelineRoundIds(
  messageGroups: Map<string, Message[]>,
  liveRoundIds: string[] = [],
  extraRoundIds: Iterable<string> = [],
): string[] {
  const live = new Set(liveRoundIds);
  const roundIds = Array.from(messageGroups.keys()).filter(
    (roundId) =>
      live.has(roundId) ||
      !isBlankNoReplyRound(messageGroups.get(roundId) ?? []),
  );
  const seen = new Set(roundIds);
  const append = (roundId: string | null | undefined) => {
    const normalized = roundId?.trim();
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    roundIds.push(normalized);
  };

  for (const roundId of extraRoundIds) {
    append(roundId);
  }
  for (const roundId of liveRoundIds) {
    append(roundId);
  }
  return roundIds;
}
