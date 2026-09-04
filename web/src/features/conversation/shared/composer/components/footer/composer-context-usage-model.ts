/**
 * INPUT: runtime 上报的上下文占用快照。
 * OUTPUT: 环形指标可安全渲染的数值与视觉等级。
 * POS: Composer 上下文用量组件的纯投影模型。
 */

import type { ContextUsageData } from "@/types/generated/protocol";
import type {
  ComposerContextUsageItem,
} from "../../composer-model";

export interface ContextUsageProjection {
  maxTokens: number;
  percentage: number;
  tone: "danger" | "soft" | "warning";
  totalTokens: number;
}

export interface ContextUsageItemProjection {
  agentId: string;
  avatar?: string | null;
  name: string;
  usage: ContextUsageProjection | null;
}

export interface ComposerContextUsageProjection {
  grouped: boolean;
  items: ContextUsageItemProjection[];
  summary: ContextUsageProjection;
}

// projectContextUsage 把协议值收敛为环形指标可安全渲染的范围。
export function projectContextUsage(
  usage: ContextUsageData | null,
): ContextUsageProjection | null {
  if (!usage || usage.max_tokens <= 0 || usage.total_tokens < 0) {
    return null;
  }
  const rawPercentage = Number.isFinite(usage.percentage)
    ? usage.percentage
    : usage.total_tokens / usage.max_tokens * 100;
  const percentage = Math.round(
    Math.min(100, Math.max(0, rawPercentage)),
  );
  const tone = percentage >= 95
    ? "danger"
    : percentage >= 80
      ? "warning"
      : "soft";
  return {
    maxTokens: usage.max_tokens,
    percentage,
    tone,
    totalTokens: usage.total_tokens,
  };
}

/** Room 用最高占用作为紧凑入口，弹层仍逐 Agent 展示真实快照。 */
export function projectComposerContextUsage({
  items = [],
  usage,
}: {
  items?: readonly ComposerContextUsageItem[];
  usage: ContextUsageData | null;
}): ComposerContextUsageProjection | null {
  if (items.length === 0) {
    const summary = projectContextUsage(usage);
    return summary ? { grouped: false, items: [], summary } : null;
  }

  const projectedItems = items.map((item) => ({
    agentId: item.agentId,
    avatar: item.avatar,
    name: item.name,
    usage: projectContextUsage(item.usage),
  }));
  const summary = projectedItems.reduce<ContextUsageProjection | null>(
    (current, item) => (
      item.usage && (!current || item.usage.percentage > current.percentage)
        ? item.usage
        : current
    ),
    null,
  );
  return summary
    ? { grouped: true, items: projectedItems, summary }
    : null;
}
