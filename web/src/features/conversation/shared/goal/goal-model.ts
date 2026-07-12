import { formatTokens } from "@/lib/format/token-count";
import type { Goal, GoalStatus } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";

interface GoalStatusTone {
  badge: string;
  icon: string;
  meter: string;
  text: string;
}

export type GoalStatusAction =
  | "refresh"
  | "edit"
  | "pause"
  | "resume"
  | "clear";

export interface GoalStatusStripModel {
  actions: GoalStatusAction[];
  attentionMessage: string | null;
  budgetLabel: string | null;
  isExecuting: boolean;
  statusLabel: string;
  statusTitle: string;
  tone: GoalStatusTone;
  usagePercent: number | null;
}

interface GoalStatusProjectionInput {
  canResume: boolean;
  continuationHold: GoalContinuationHold | null;
  error: string | null;
  goal: Goal;
  isGenerating: boolean;
}

interface VisibleGoalStatus {
  label: string;
  status: GoalStatus;
}

interface GoalStatusOverride {
  label: string;
  matches: (input: GoalStatusProjectionInput) => boolean;
  status: GoalStatus;
}

interface GoalActionRule {
  action: GoalStatusAction;
  visible: (input: GoalStatusProjectionInput) => boolean;
}

export const GOAL_PANEL_STRIP_CLASS_NAME =
  "mx-auto w-full max-w-[960px] px-3 sm:px-5 xl:px-6";

export const GOAL_PANEL_COMPACT_CLASS_NAME = "max-w-none px-2";

export const GOAL_PANEL_SURFACE_CLASS_NAME =
  "border-b border-(--surface-canvas-border) py-1";

export const GOAL_PANEL_ROW_CLASS_NAME =
  "group -mx-1 flex min-h-8 items-center gap-2 px-1 py-0.5 text-(--text-default)";

export const GOAL_PANEL_LEADING_ICON_CLASS_NAME =
  "inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-[7px] bg-[color:color-mix(in_srgb,var(--primary)_9%,transparent)] text-(--primary)";

export const GOAL_PANEL_BADGE_CLASS_NAME =
  "inline-flex shrink-0 items-center rounded-[7px] border px-1.5 py-0.5 text-[10px] font-semibold leading-none text-(--text-soft)";

const GOAL_STATUS_LABEL: Record<GoalStatus, string> = {
  active: "运行中",
  blocked: "已阻塞",
  budget_limited: "预算耗尽",
  complete: "已完成",
  paused: "已暂停",
  usage_limited: "续跑受限",
};

const ACTIVE_TONE: GoalStatusTone = {
  badge: "border-[color:color-mix(in_srgb,var(--success)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
  icon: "border-[color:color-mix(in_srgb,var(--success)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
  meter: "bg-(--success)",
  text: "text-(--success)",
};

const PAUSED_TONE: GoalStatusTone = {
  badge: "border-[color:color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning)",
  icon: "border-[color:color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning)",
  meter: "bg-(--warning)",
  text: "text-(--warning)",
};

const COMPLETE_TONE: GoalStatusTone = {
  badge: "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)",
  icon: "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)",
  meter: "bg-(--status-info-soft-text)",
  text: "text-(--status-info-soft-text)",
};

const LIMITED_TONE: GoalStatusTone = {
  badge: "border-destructive/25 bg-destructive/10 text-destructive",
  icon: "border-destructive/25 bg-destructive/10 text-destructive",
  meter: "bg-destructive",
  text: "text-destructive",
};

const GOAL_STATUS_TONE: Record<GoalStatus, GoalStatusTone> = {
  active: ACTIVE_TONE,
  blocked: LIMITED_TONE,
  budget_limited: LIMITED_TONE,
  complete: COMPLETE_TONE,
  paused: PAUSED_TONE,
  usage_limited: LIMITED_TONE,
};

const ACTIVE_STATUS_OVERRIDES: GoalStatusOverride[] = [
  {
    label: "需处理",
    matches: ({ goal }) => Boolean(goal.last_error),
    status: "blocked",
  },
  {
    label: "待继续",
    matches: ({ continuationHold, goal }) =>
      continuationHold !== null || (goal.empty_progress_count ?? 0) > 0,
    status: "paused",
  },
];

const GOAL_ACTION_RULES: GoalActionRule[] = [
  { action: "refresh", visible: () => true },
  { action: "edit", visible: () => true },
  {
    action: "pause",
    visible: ({ goal }) => goal.status === "active",
  },
  { action: "resume", visible: ({ canResume }) => canResume },
  { action: "clear", visible: () => true },
];

function goalUsageTotal(goal: Goal | null): number {
  return goal?.usage?.total_tokens ?? 0;
}

function goalBudgetPercent(goal: Goal | null): number | null {
  const budget = goal?.token_budget ?? null;
  if (!budget || budget <= 0) {
    return null;
  }
  return Math.min(100, Math.round((goalUsageTotal(goal) / budget) * 100));
}

function goalStatusTone(status: GoalStatus): GoalStatusTone {
  return GOAL_STATUS_TONE[status];
}

export function buildGoalStatusStripModel(
  input: GoalStatusProjectionInput,
): GoalStatusStripModel {
  const activeContinuationHold =
    input.goal.status === "active" ? input.continuationHold : null;
  const activeInput = { ...input, continuationHold: activeContinuationHold };
  const visibleStatus = resolveVisibleGoalStatus(activeInput);

  return {
    actions: GOAL_ACTION_RULES.filter((rule) => rule.visible(activeInput)).map(
      (rule) => rule.action,
    ),
    attentionMessage: input.error ?? input.goal.last_error ?? null,
    budgetLabel: buildGoalBudgetLabel(input.goal),
    isExecuting: input.isGenerating && input.goal.status === "active",
    statusLabel: visibleStatus.label,
    statusTitle: activeContinuationHold?.detail ?? visibleStatus.label,
    tone: goalStatusTone(visibleStatus.status),
    usagePercent: goalBudgetPercent(input.goal),
  };
}

function resolveVisibleGoalStatus(
  input: GoalStatusProjectionInput,
): VisibleGoalStatus {
  const override = isIdleActiveGoal(input)
    ? ACTIVE_STATUS_OVERRIDES.find((candidate) => candidate.matches(input))
    : null;
  return {
    label: override?.label ?? GOAL_STATUS_LABEL[input.goal.status],
    status: override?.status ?? input.goal.status,
  };
}

function isIdleActiveGoal(input: GoalStatusProjectionInput): boolean {
  return input.goal.status === "active" && !input.isGenerating;
}

function buildGoalBudgetLabel(goal: Goal): string | null {
  const usageTotal = goalUsageTotal(goal);
  const budget = goal.token_budget ?? null;
  if (budget && budget > 0) {
    return `${formatTokens(usageTotal)} / ${formatTokens(budget)}`;
  }
  return usageTotal > 0 ? formatTokens(usageTotal) : null;
}

export function buildGoalActivityKey(
  messageCount: number,
  isLoading: boolean,
  refreshSequence: number,
): string {
  return `${messageCount}:${isLoading ? "loading" : "idle"}:${refreshSequence}`;
}
