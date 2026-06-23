import type { AssistantMessage } from "@/types/conversation/message";

import type { MessageStatsData } from "./message-item-types";

function format_compact_count(value: number): string {
  if (!Number.isFinite(value)) {
    return "0";
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(value >= 10_000_000 ? 0 : 1)}m`;
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(value >= 10_000 ? 0 : 1)}k`;
  }
  return `${value}`;
}

export function get_result_summary_display_text(
  result_summary: AssistantMessage["result_summary"] | undefined,
): string | null {
  const result_text = result_summary?.result?.trim();
  if (result_text) {
    return result_text;
  }
  if (!result_summary) {
    return null;
  }
  if (result_summary.subtype === "interrupted") {
    return null;
  }
  if (result_summary.subtype === "error" || result_summary.is_error) {
    return "执行失败";
  }
  return null;
}

export function build_message_stats(
  result_summary: AssistantMessage["result_summary"] | undefined,
): MessageStatsData | null {
  const usage = result_summary?.usage;
  const duration = result_summary
    ? result_summary.duration_ms > 0
      ? `${(result_summary.duration_ms / 1000).toFixed(1)}s`
      : "0s"
    : null;
  const cost =
    result_summary?.total_cost_usd !== undefined
      ? `$${result_summary.total_cost_usd.toFixed(4)}`
      : null;
  const cache_hit = usage?.cache_read_input_tokens;
  const tokens = usage
    ? `${format_compact_count(usage.input_tokens)}↑ ${format_compact_count(usage.output_tokens)}↓`
    : null;

  if (!duration && !tokens && !cost && !cache_hit) {
    return null;
  }

  return {
    duration,
    tokens,
    cost,
    cache_hit:
      cache_hit && cache_hit > 0
        ? `缓存 ${format_compact_count(cache_hit)}`
        : null,
  };
}
