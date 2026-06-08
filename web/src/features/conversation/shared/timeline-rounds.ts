import type { Message } from "@/types/conversation/message";

/** 时间线除历史消息外，也要显示已启动但尚未产生消息的运行轮次。 */
export function build_timeline_round_ids(
  message_groups: Map<string, Message[]>,
  live_round_ids: string[] = [],
  extra_round_ids: Iterable<string> = [],
): string[] {
  const round_ids = Array.from(message_groups.keys());
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
