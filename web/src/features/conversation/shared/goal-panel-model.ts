import type { Goal, GoalStatus } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";

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

export type GoalRunTone = "active" | "waiting" | "stopped" | "done";

export interface GoalRunState {
  detail: string;
  label: string;
  tone: GoalRunTone;
}

export function goal_run_state(
  goal: Goal,
  is_generating: boolean,
  continuation_hold: GoalContinuationHold | null = null,
): GoalRunState {
  switch (goal.status) {
    case "active":
      if (continuation_hold) {
        return {
          detail: continuation_hold.detail,
          label: continuation_hold.label,
          tone: "waiting",
        };
      }
      if ((goal.empty_progress_count ?? 0) > 0) {
        return {
          detail: "上一轮没有可计入进展，等待新的用户或外部活动",
          label: "续跑暂停",
          tone: "waiting",
        };
      }
      return is_generating
        ? {
            detail: "正在执行 Goal，完成后会显示回复",
            label: "当前轮次",
            tone: "active",
          }
        : {
            detail: "下一轮会携带 Goal 上下文继续推进",
            label: "下轮继续",
            tone: "active",
          };
    case "paused":
      return {
        detail: "用户暂停后不会自动续跑",
        label: "已暂停",
        tone: "waiting",
      };
    case "blocked":
      return {
        detail: "解除阻塞或补充输入后继续",
        label: "等待输入",
        tone: "stopped",
      };
    case "budget_limited":
      return {
        detail: "调整预算后才能继续",
        label: "预算耗尽",
        tone: "stopped",
      };
    case "usage_limited":
      return {
        detail: "续跑上限触发，需要用户确认继续",
        label: "续跑受限",
        tone: "stopped",
      };
    case "complete":
      return {
        detail: "目标已完成，不再自动续跑",
        label: "已完成",
        tone: "done",
      };
    default:
      return {
        detail: "Goal 状态已更新",
        label: "Goal",
        tone: "waiting",
      };
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
        badge: "border-border/70 bg-muted/60 text-muted-foreground",
        icon: "border-border/70 bg-muted/60 text-muted-foreground",
        meter: "bg-muted-foreground",
        rail: "bg-muted-foreground",
        text: "text-muted-foreground",
      };
  }
}
