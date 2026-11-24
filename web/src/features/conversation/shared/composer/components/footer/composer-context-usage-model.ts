/**
 * INPUT: runtime 上报的上下文占用快照。
 * OUTPUT: 环形指标可安全渲染的数值与视觉等级，以及详情浮层的分类占用条与明细行。
 * POS: Composer 上下文用量组件的纯投影模型。
 */

import type { ContextUsageData } from "@/types/generated/protocol";
import type {
  ComposerContextUsageItem,
} from "../../composer-model";

export type ContextUsageCategoryKind =
  | "agents"
  | "autocompact_buffer"
  | "free_space"
  | "mcp_tools"
  | "memory_files"
  | "messages"
  | "other"
  | "skills"
  | "system_prompt"
  | "system_tools";

export interface ContextUsageCategoryProjection {
  deferred: boolean;
  kind: ContextUsageCategoryKind;
  /** runtime 原始名称；未知分类直接展示它。 */
  name: string;
  /** 占窗口比例（0-100，一位小数）；deferred 分类不占窗口，为 null。 */
  percentage: number | null;
  tokens: number;
}

export interface ContextUsageProjection {
  /** 非 deferred 分类（含 buffer 与 free），按 runtime 顺序；没有明细时为空。 */
  categories: ContextUsageCategoryProjection[];
  /** 按需加载、尚未进入窗口的分类，只做提示。 */
  deferredCategories: ContextUsageCategoryProjection[];
  maxTokens: number;
  model: string | null;
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
  const categories = (usage.categories ?? [])
    .filter((category) => category.tokens > 0 && category.name.trim())
    .map((category) => projectCategory(category, usage.max_tokens));
  return {
    categories: categories.filter((category) => !category.deferred),
    deferredCategories: categories.filter((category) => category.deferred),
    maxTokens: usage.max_tokens,
    model: usage.model?.trim() || null,
    percentage,
    tone,
    totalTokens: usage.total_tokens,
  };
}

// Claude Code 与 nxs 的分类名拼写不同（"Autocompact buffer" / "Auto-compact buffer"、"Deferred MCP tools" / deferred 标记），
// 统一折叠成字母键再匹配；未知分类保留原名。
const CATEGORY_KIND_BY_KEY: Readonly<Record<string, ContextUsageCategoryKind>> = {
  agents: "agents",
  autocompactbuffer: "autocompact_buffer",
  customagents: "agents",
  freespace: "free_space",
  mcptools: "mcp_tools",
  memoryfiles: "memory_files",
  messages: "messages",
  skills: "skills",
  systemprompt: "system_prompt",
  systemtools: "system_tools",
};

function projectCategory(
  category: NonNullable<ContextUsageData["categories"]>[number],
  maxTokens: number,
): ContextUsageCategoryProjection {
  const name = category.name.trim();
  const key = name.toLowerCase().replace(/^deferred\s+/, "").replace(/\(deferred\)/, "").replace(/[^a-z]/g, "");
  const deferred = Boolean(category.deferred) || /deferred/i.test(name);
  return {
    deferred,
    kind: CATEGORY_KIND_BY_KEY[key] ?? "other",
    name,
    percentage: deferred ? null : Math.round(category.tokens / maxTokens * 1000) / 10,
    tokens: category.tokens,
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
