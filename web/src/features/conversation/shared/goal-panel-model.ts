import type { Goal, GoalEvent, GoalStatus } from "@/types/conversation/goal";

export const GOAL_STATUS_LABEL: Record<GoalStatus, string> = {
  active: "运行中",
  paused: "已暂停",
  complete: "已完成",
  blocked: "已阻塞",
  budget_limited: "预算耗尽",
  usage_limited: "续跑受限",
  cleared: "已清除",
};

export const GOAL_EVENT_LABEL: Record<string, string> = {
  created: "创建",
  updated: "更新",
  resumed: "继续",
  paused: "暂停",
  completed: "完成",
  blocked: "阻塞",
  cleared: "清除",
  usage_recorded: "用量",
  budget_limited: "预算",
  usage_limited: "受限",
  continuation_scheduled: "续跑",
  checkpoint_created: "检查点",
};

const GOAL_SOURCE_LABEL: Record<GoalEvent["source"], string> = {
  user: "用户",
  model: "模型",
  system: "系统",
  external: "外部",
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
  return `${days}d ${remaining_hours}h`;
}

export function goal_runtime_label(goal: Goal, is_generating: boolean): string {
  switch (goal.status) {
    case "active":
      return is_generating ? "执行中" : "待续跑";
    case "paused":
      return "暂停";
    case "blocked":
      return "等待输入";
    case "budget_limited":
      return "预算耗尽";
    case "usage_limited":
      return "续跑受限";
    case "complete":
      return "完成";
    case "cleared":
      return "清除";
    default:
      return "Goal";
  }
}

export function goal_event_label(event: GoalEvent): string {
  const label = GOAL_EVENT_LABEL[event.event_type] ?? event.event_type;
  const source = GOAL_SOURCE_LABEL[event.source] ?? event.source;
  return `${source} · ${label}`;
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
        badge: "border-border/70 bg-muted/60 text-muted-foreground",
        icon: "border-border/70 bg-muted/60 text-muted-foreground",
        meter: "bg-muted-foreground",
        rail: "bg-muted-foreground",
        text: "text-muted-foreground",
      };
  }
}
