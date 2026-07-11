import type { AssistantMessage } from "@/types/conversation/message/entity";

import { stripRoomControlMarkers } from "../../message-content-model";

type ResultSummary = NonNullable<AssistantMessage["result_summary"]>;

interface CompactCountRule {
  format: (value: number) => string;
  matches: (value: number) => boolean;
}

interface ResultDisplayContext {
  resultText: string;
  summary?: ResultSummary;
}

interface ResultDisplayRule {
  matches: (context: ResultDisplayContext) => boolean;
  resolve: (context: ResultDisplayContext) => string | null;
}

const DEFAULT_COMPACT_COUNT_RULE: CompactCountRule = {
  format: (value) => `${value}`,
  matches: () => true,
};

const COMPACT_COUNT_RULES: CompactCountRule[] = [
  { matches: (value) => !Number.isFinite(value), format: () => "0" },
  { matches: (value) => value >= 10_000_000, format: (value) => `${(value / 1_000_000).toFixed(0)}m` },
  { matches: (value) => value >= 1_000_000, format: (value) => `${(value / 1_000_000).toFixed(1)}m` },
  { matches: (value) => value >= 10_000, format: (value) => `${(value / 1_000).toFixed(0)}k` },
  { matches: (value) => value >= 1_000, format: (value) => `${(value / 1_000).toFixed(1)}k` },
];

const DEFAULT_RESULT_DISPLAY_RULE: ResultDisplayRule = {
  matches: () => true,
  resolve: () => null,
};

const RESULT_DISPLAY_RULES: ResultDisplayRule[] = [
  {
    matches: ({ resultText }) => Boolean(resultText),
    resolve: ({ resultText }) => resultText,
  },
  {
    matches: ({ summary }) => summary?.subtype === "interrupted",
    resolve: () => null,
  },
  {
    matches: ({ summary }) => summary?.subtype === "error"
      || Boolean(summary?.is_error),
    resolve: () => "执行失败",
  },
];

function formatCompactCount(value: number): string {
  const rule = COMPACT_COUNT_RULES.find((candidate) => candidate.matches(value))
    ?? DEFAULT_COMPACT_COUNT_RULE;
  return rule.format(value);
}

export function getResultSummaryDisplayText(
  resultSummary: AssistantMessage["result_summary"] | undefined,
): string | null {
  const context: ResultDisplayContext = {
    resultText: stripRoomControlMarkers(resultSummary?.result ?? ""),
    summary: resultSummary,
  };
  const rule = RESULT_DISPLAY_RULES.find(
    (candidate) => candidate.matches(context),
  ) ?? DEFAULT_RESULT_DISPLAY_RULE;
  return rule.resolve(context);
}

export function buildMessageStats(
  resultSummary: AssistantMessage["result_summary"] | undefined,
) {
  const stats = {
    duration: resolveDuration(resultSummary),
    tokens: resolveTokens(resultSummary),
    cost: resolveCost(resultSummary),
    cacheHit: resolveCacheHit(resultSummary),
  };
  return Object.values(stats).some(Boolean) ? stats : null;
}

function resolveDuration(summary?: ResultSummary): string | null {
  if (!summary) {
    return null;
  }
  return summary.duration_ms > 0
    ? `${(summary.duration_ms / 1000).toFixed(1)}s`
    : "0s";
}

function resolveCost(summary?: ResultSummary): string | null {
  return summary?.total_cost_usd === undefined
    ? null
    : `$${summary.total_cost_usd.toFixed(4)}`;
}

function resolveTokens(summary?: ResultSummary): string | null {
  const usage = summary?.usage;
  return usage
    ? `${formatCompactCount(usage.input_tokens)}↑ ${formatCompactCount(usage.output_tokens)}↓`
    : null;
}

function resolveCacheHit(summary?: ResultSummary): string | null {
  const cacheHit = summary?.usage?.cache_read_input_tokens ?? 0;
  return cacheHit > 0 ? `缓存 ${formatCompactCount(cacheHit)}` : null;
}
