import type { Goal, GoalStatus } from "@/types/conversation/goal";

export const GOAL_STATUS_LABEL: Record<GoalStatus, string> = {
  active: "运行中",
  paused: "已暂停",
  complete: "已完成",
  blocked: "已阻塞",
  budget_limited: "预算耗尽",
  usage_limited: "续跑受限",
};

export function goalUsageTotal(goal: Goal | null): number {
  return goal?.usage?.total_tokens ?? 0;
}

export function goalBudgetPercent(goal: Goal | null): number | null {
  const budget = goal?.token_budget ?? null;
  if (!budget || budget <= 0) return null;
  return Math.min(100, Math.round((goalUsageTotal(goal) / budget) * 100));
}

export function goalStatusTone(status: GoalStatus): {
  badge: string;
  icon: string;
  meter: string;
  rail: string;
  text: string;
} {
  switch (status) {
    case "active":
      return {
        badge: "border-[color:color-mix(in_srgb,var(--success)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
        icon: "border-[color:color-mix(in_srgb,var(--success)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
        meter: "bg-(--success)",
        rail: "bg-(--success)",
        text: "text-(--success)",
      };
    case "paused":
      return {
        badge: "border-[color:color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning)",
        icon: "border-[color:color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning)",
        meter: "bg-(--warning)",
        rail: "bg-(--warning)",
        text: "text-(--warning)",
      };
    case "complete":
      return {
        badge: "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)",
        icon: "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)",
        meter: "bg-(--status-info-soft-text)",
        rail: "bg-(--status-info-soft-text)",
        text: "text-(--status-info-soft-text)",
      };
    case "blocked":
    case "budget_limited":
    case "usage_limited":
      return {
        badge: "border-destructive/25 bg-destructive/10 text-destructive",
        icon: "border-destructive/25 bg-destructive/10 text-destructive",
        meter: "bg-destructive",
        rail: "bg-destructive",
        text: "text-destructive",
      };
    default:
      return {
        badge: "border-transparent bg-transparent text-(--text-soft)",
        icon: "bg-[color:color-mix(in_srgb,var(--primary)_9%,transparent)] text-(--primary)",
        meter: "bg-(--text-soft)",
        rail: "bg-(--text-soft)",
        text: "text-(--text-soft)",
      };
  }
}
