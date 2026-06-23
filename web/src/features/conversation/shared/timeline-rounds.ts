import type { AssistantMessage, Message } from "@/types/conversation/message";
import { strip_room_control_markers } from "./message/item/message-item-support";

// 终态轮次里 assistant 仅剩无回复标记（剥离后无文本、无工具/图片等块）时，
// 视为纯 no-reply，不在时间线显示。保守判定：任何工具/非文本块都算可见输出。
function is_blank_no_reply_round(messages: Message[]): boolean {
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
        if (strip_room_control_markers(block.text)) {
          return false;
        }
        continue;
      }
      return false;
    }
    const result_text = assistant.result_summary?.result;
    if (result_text && strip_room_control_markers(result_text)) {
      return false;
    }
  }
  return true;
}

/** 时间线除历史消息外，也要显示已启动但尚未产生消息的运行轮次。 */
export function build_timeline_round_ids(
  message_groups: Map<string, Message[]>,
  live_round_ids: string[] = [],
  extra_round_ids: Iterable<string> = [],
): string[] {
  const live = new Set(live_round_ids);
  const round_ids = Array.from(message_groups.keys()).filter(
    (round_id) =>
      live.has(round_id) ||
      !is_blank_no_reply_round(message_groups.get(round_id) ?? []),
  );
  const seen = new Set(round_ids);
  const append = (round_id: string | null | undefined) => {
    const normalized = round_id?.trim();
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    round_ids.push(normalized);
  };

  for (const round_id of extra_round_ids) {
    append(round_id);
  }
  for (const round_id of live_round_ids) {
    append(round_id);
  }
  return round_ids;
}
