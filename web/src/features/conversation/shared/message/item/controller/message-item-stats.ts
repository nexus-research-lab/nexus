import type { AssistantMessage } from "@/types/conversation/message";

import { stripRoomControlMarkers } from "../message-item-support";
import type { MessageStatsData } from "../message-item-types";

function formatCompactCount(value: number): string {
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

export function getResultSummaryDisplayText(
  resultSummary: AssistantMessage["result_summary"] | undefined,
): string | null {
  const resultText = stripRoomControlMarkers(resultSummary?.result ?? "");
  if (resultText) {
    return resultText;
  }
  if (!resultSummary) {
    return null;
  }
  if (resultSummary.subtype === "interrupted") {
    return null;
  }
  if (resultSummary.subtype === "error" || resultSummary.is_error) {
    return "执行失败";
  }
  return null;
}

export function buildMessageStats(
  resultSummary: AssistantMessage["result_summary"] | undefined,
): MessageStatsData | null {
  const usage = resultSummary?.usage;
  const duration = resultSummary
    ? resultSummary.duration_ms > 0
      ? `${(resultSummary.duration_ms / 1000).toFixed(1)}s`
      : "0s"
    : null;
  const cost =
    resultSummary?.total_cost_usd !== undefined
      ? `$${resultSummary.total_cost_usd.toFixed(4)}`
      : null;
  const cacheHit = usage?.cache_read_input_tokens;
  const tokens = usage
    ? `${formatCompactCount(usage.input_tokens)}↑ ${formatCompactCount(usage.output_tokens)}↓`
    : null;

  if (!duration && !tokens && !cost && !cacheHit) {
    return null;
  }

  return {
    duration,
    tokens,
    cost,
    cacheHit:
      cacheHit && cacheHit > 0
        ? `缓存 ${formatCompactCount(cacheHit)}`
        : null,
  };
}
