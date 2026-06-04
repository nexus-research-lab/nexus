import type { Goal, GoalStatus } from "@/types/conversation/goal";

export const GOAL_STATUS_LABEL: Record<GoalStatus, string> = {
  active: "运行中",
  paused: "已暂停",
  complete: "已完成",
  blocked: "已阻塞",
  budget_limited: "预算耗尽",
  usage_limited: "续跑受限",
};

export function goal_usage_total(goal: Goal | null): number {
  return goal?.usage?.total_tokens ?? 0;
}

export function goal_budget_percent(goal: Goal | null): number | null {
  const budget = goal?.token_budget ?? null;
  if (!budget || budget <= 0) return null;
  return Math.min(100, Math.round((goal_usage_total(goal) / budget) * 100));
}

export function goal_elapsed_label(seconds?: number | null): string {
  const normalized = Math.max(0, Math.floor(seconds ?? 0));
  if (normalized < 60) return `${normalized}s`;
  const minutes = Math.floor(normalized / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remaining_minutes = minutes % 60;
  if (hours < 24) {
    return remaining_minutes === 0 ? `${hours}h` : `${hours}h ${remaining_minutes}m`;
  }
  const days = Math.floor(hours / 24);
  const remaining_hours = hours % 24;
  return `${days}d ${remaining_hours}h ${remaining_minutes}m`;
}

export function goal_runtime_label(goal: Goal, is_generating: boolean): string {
  switch (goal.status) {
    case "active":
      if (!is_generating && goal.last_error) return "需处理";
      if (!is_generating && (goal.empty_progress_count ?? 0) > 0) return "续跑暂停";
      return is_generating ? "执行中" : "追踪中";
    case "paused":
      return "已暂停";
    case "blocked":
      return "等待输入";
    case "budget_limited":
      return "预算耗尽";
    case "usage_limited":
      return "续跑受限";
    case "complete":
      return "已完成";
    default:
      return "Goal";
  }
}

export function goal_status_tone(status: GoalStatus): {
  badge: string;
  icon: string;
  meter: string;
  rail: string;
  text: string;
} {
  switch (status) {
    case "active":
      return {
        badge: "border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
        icon: "border-emerald-500/25 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300",
        meter: "bg-emerald-500",
        rail: "bg-emerald-500",
        text: "text-emerald-700 dark:text-emerald-300",
      };
    case "paused":
      return {
        badge: "border-amber-500/25 bg-amber-500/10 text-amber-700 dark:text-amber-300",
        icon: "border-amber-500/25 bg-amber-500/10 text-amber-600 dark:text-amber-300",
        meter: "bg-amber-500",
        rail: "bg-amber-500",
        text: "text-amber-700 dark:text-amber-300",
      };
    case "complete":
      return {
        badge: "border-sky-500/25 bg-sky-500/10 text-sky-700 dark:text-sky-300",
        icon: "border-sky-500/25 bg-sky-500/10 text-sky-600 dark:text-sky-300",
        meter: "bg-sky-500",
        rail: "bg-sky-500",
        text: "text-sky-700 dark:text-sky-300",
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
        meter: "bg-muted-foreground",
        rail: "bg-muted-foreground",
        text: "text-(--text-soft)",
      };
  }
}
