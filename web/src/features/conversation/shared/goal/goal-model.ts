/**
 * INPUT: Goal state, server-derived Execution binding and UI command phase.
 * OUTPUT: Goal lifecycle plus meaningful server-derived WorkGraph binding badges, clear capability, form and controller projections.
 * POS: Goal panel pure model; metadata never participates in WorkGraph binding decisions.
 */
import type {
  Goal,
  GoalExecutionBinding,
  GoalExecutionBindingState,
  GoalStatus,
} from "@/types/conversation/goal";
import type { TranslationKey } from "@/shared/i18n/messages";
import { COMPOSER_COMPACT_LANE_CLASS_NAME } from "../composer/composer-styles";
import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "../conversation-panel-styles";
import type { GoalContinuationHold } from "./goal-continuation-hold";

export type GoalCommandPhase = "clearing" | "pausing" | "resuming" | "updating";

export interface GoalDraft {
  budget: string;
  goalId: string;
  objective: string;
}

export type GoalDialog =
  | { kind: "clear"; goal: Goal }
  | { kind: "none" };

export interface GoalControllerProjection {
  canResume: boolean;
  clearDisabledReason: string | null;
  dialog: GoalDialog;
  draft: GoalDraft | null;
  loadingLabel: string | null;
}

export interface GoalDraftFormModel {
  canClose: boolean;
  fieldsDisabled: boolean;
  isLoading: boolean;
  submitDisabled: boolean;
  submitLabel: string;
  submitTone: "default" | "primary";
}

interface GoalStatusTone {
  badge: string;
  icon: string;
  text: string;
}

export type GoalStatusAction =
  | "refresh"
  | "edit"
  | "pause"
  | "resume"
  | "clear";

export interface GoalStatusStripModel {
  actionDisabledReasons: Partial<Record<GoalStatusAction, string>>;
  actions: GoalStatusAction[];
  attentionMessage: string | null;
  attentionTone: "danger" | "warning" | null;
  bindingBadge: GoalBindingBadgeModel | null;
  statusLabel: string;
  statusTitle: string;
  tone: GoalStatusTone;
  usageLabel: string | null;
}

interface GoalStatusProjectionInput {
  canResume: boolean;
  clearDisabledReason?: string | null;
  continuationHold: GoalContinuationHold | null;
  error: string | null;
  executionBinding?: GoalExecutionBinding | null;
  goal: Goal;
  isGenerating: boolean;
}

export type GoalBindingDisplayState =
  | Exclude<GoalExecutionBindingState, "standalone" | "reserved">
  | "unavailable";

export interface GoalBindingBadgeModel {
  labelKey: TranslationKey;
  state: GoalBindingDisplayState;
  titleKey: TranslationKey;
  tone: "conflict" | "confirmed" | "pending" | "unavailable";
}

interface VisibleGoalStatus {
  label: string;
  status: GoalStatus;
}

interface GoalActionRule {
  action: GoalStatusAction;
  visible: (input: GoalStatusProjectionInput) => boolean;
}

export const GOAL_PANEL_STRIP_CLASS_NAME =
  `${CONVERSATION_CONTENT_LANE_CLASS_NAME} px-3 sm:px-5 xl:px-6`;

export const GOAL_PANEL_COMPACT_CLASS_NAME =
  `${COMPOSER_COMPACT_LANE_CLASS_NAME} px-4`;

export const GOAL_PANEL_SURFACE_CLASS_NAME =
  "rounded-[16px] border border-(--surface-control-border) bg-[color:color-mix(in_srgb,var(--surface-raised-background)_94%,transparent)] px-3 py-1.5 shadow-(--surface-control-shadow)";

export const GOAL_PANEL_ROW_CLASS_NAME =
  "group -mx-1 flex min-h-8 items-center gap-2 px-1 py-0.5 text-(--text-default)";

export const GOAL_PANEL_LEADING_ICON_CLASS_NAME =
  "inline-flex h-5 w-5 shrink-0 items-center justify-center radius-control-xs bg-[color:color-mix(in_srgb,var(--primary)_9%,transparent)] text-(--primary)";

export const GOAL_PANEL_BADGE_CLASS_NAME =
  "inline-flex shrink-0 items-center radius-control-xs border px-1.5 py-0.5 text-2xs font-semibold leading-none text-(--text-soft)";

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
  text: "text-(--success)",
};

const PAUSED_TONE: GoalStatusTone = {
  badge: "border-[color:color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning)",
  icon: "border-[color:color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning)",
  text: "text-(--warning)",
};

const COMPLETE_TONE: GoalStatusTone = {
  badge: "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)",
  icon: "border-(--status-info-soft-border) bg-(--status-info-soft-bg) text-(--status-info-soft-text)",
  text: "text-(--status-info-soft-text)",
};

const LIMITED_TONE: GoalStatusTone = {
  badge: "border-destructive/25 bg-destructive/10 text-destructive",
  icon: "border-destructive/25 bg-destructive/10 text-destructive",
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

const GOAL_BINDING_BADGE: Record<
  GoalBindingDisplayState,
  GoalBindingBadgeModel
> = {
  pending: {
    labelKey: "goal.binding_pending",
    state: "pending",
    titleKey: "goal.binding_pending_title",
    tone: "pending",
  },
  confirmed: {
    labelKey: "goal.binding_confirmed",
    state: "confirmed",
    titleKey: "goal.binding_confirmed_title",
    tone: "confirmed",
  },
  conflict: {
    labelKey: "goal.binding_conflict",
    state: "conflict",
    titleKey: "goal.binding_conflict_title",
    tone: "conflict",
  },
  unavailable: {
    labelKey: "goal.binding_unavailable",
    state: "unavailable",
    titleKey: "goal.binding_unavailable_title",
    tone: "unavailable",
  },
};

const EMPTY_PROGRESS_LABEL = "自动续跑已停止";
const GOAL_EXECUTING_LABEL = "执行中";
const EMPTY_PROGRESS_MESSAGE =
  "上一轮未产生可计入进展，系统已停止自动续跑；这不是 Agent 主动暂停。";

const GOAL_ACTION_RULES: GoalActionRule[] = [
  { action: "refresh", visible: () => true },
  { action: "edit", visible: () => true },
  {
    action: "pause",
    visible: ({ goal }) => goal.status === "active",
  },
  {
    action: "resume",
    visible: ({ canResume, isGenerating }) => canResume && !isGenerating,
  },
  { action: "clear", visible: () => true },
];

export const EMPTY_GOAL_DIALOG: GoalDialog = { kind: "none" };

const RESUMABLE_GOAL_STATUSES = new Set<GoalStatus>([
  "blocked",
  "paused",
  "usage_limited",
]);

function positiveTokenCount(value: number | null | undefined): number {
  return Number.isFinite(value) ? Math.max(0, value ?? 0) : 0;
}

export function goalActualTokens(goal: Goal | null): number {
  const usage = goal?.usage;
  if (!usage) {
    return 0;
  }
  const hasBreakdown = [
    usage.input_tokens,
    usage.output_tokens,
    usage.cache_creation_input_tokens,
    usage.cache_read_input_tokens,
    usage.reasoning_tokens,
  ].some((value) => Number.isFinite(value) && (value ?? 0) > 0);
  const explicitActual = positiveTokenCount(usage.actual_tokens);
  if (usage.actual_tokens !== undefined && (explicitActual > 0 || !hasBreakdown)) {
    return explicitActual;
  }
  if (!hasBreakdown) {
    return positiveTokenCount(usage.total_tokens);
  }
  return positiveTokenCount(usage.input_tokens)
    + positiveTokenCount(usage.cache_creation_input_tokens)
    + positiveTokenCount(usage.cache_read_input_tokens)
    + Math.max(
      positiveTokenCount(usage.output_tokens),
      positiveTokenCount(usage.reasoning_tokens),
    );
}

function goalActualTokensEstimated(goal: Goal): boolean {
  const usage = goal.usage;
  return goalActualTokens(goal) > 0
    && (usage?.actual_tokens_estimated === true
      || usage?.actual_tokens === undefined
      || positiveTokenCount(usage.actual_tokens) === 0);
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
    actionDisabledReasons: input.clearDisabledReason
      ? { clear: input.clearDisabledReason }
      : {},
    actions: GOAL_ACTION_RULES.filter((rule) => rule.visible(activeInput)).map(
      (rule) => rule.action,
    ),
    attentionMessage: resolveGoalAttentionMessage(activeInput),
    attentionTone: resolveGoalAttentionTone(activeInput),
    bindingBadge: resolveGoalBindingBadgeModel(input.executionBinding ?? null),
    statusLabel: visibleStatus.label,
    statusTitle: resolveGoalStatusTitle(activeInput, visibleStatus),
    tone: goalStatusTone(visibleStatus.status),
    usageLabel: buildGoalUsageLabel(input.goal),
  };
}

export function resolveGoalBindingBadgeModel(
  binding: GoalExecutionBinding | null,
): GoalBindingBadgeModel | null {
  if (binding?.state === "standalone" || binding?.state === "reserved") {
    return null;
  }
  return GOAL_BINDING_BADGE[binding?.state ?? "unavailable"];
}

function resolveVisibleGoalStatus(
  input: GoalStatusProjectionInput,
): VisibleGoalStatus {
  if (input.goal.status === "active" && input.isGenerating) {
    return { label: GOAL_EXECUTING_LABEL, status: "active" };
  }
  if (!isIdleActiveGoal(input)) {
    return {
      label: GOAL_STATUS_LABEL[input.goal.status],
      status: input.goal.status,
    };
  }
  if (input.goal.last_error) {
    return { label: "需处理", status: "blocked" };
  }
  if (input.continuationHold) {
    return { label: input.continuationHold.label, status: "paused" };
  }
  if (goalContinuationSuppressed(input.goal)) {
    return { label: EMPTY_PROGRESS_LABEL, status: "paused" };
  }
  return {
    label: GOAL_STATUS_LABEL[input.goal.status],
    status: input.goal.status,
  };
}

function resolveGoalStatusTitle(
  input: GoalStatusProjectionInput,
  visibleStatus: VisibleGoalStatus,
): string {
  if (input.continuationHold) {
    return input.continuationHold.detail;
  }
  if (input.goal.status === "active" &&
    goalContinuationSuppressed(input.goal) &&
    !input.isGenerating) {
    return `${EMPTY_PROGRESS_MESSAGE} 点击“继续”可重试。`;
  }
  return visibleStatus.label;
}

function resolveGoalAttentionMessage(
  input: GoalStatusProjectionInput,
): string | null {
  if (input.error || input.goal.last_error) {
    return input.error ?? input.goal.last_error ?? null;
  }
  if (input.goal.status === "active" &&
    goalContinuationSuppressed(input.goal) &&
    !input.isGenerating) {
    return EMPTY_PROGRESS_MESSAGE;
  }
  return null;
}

function resolveGoalAttentionTone(
  input: GoalStatusProjectionInput,
): GoalStatusStripModel["attentionTone"] {
  if (input.error || input.goal.last_error) {
    return "danger";
  }
  if (input.goal.status === "blocked" && input.goal.blocker) {
    return "warning";
  }
  if (input.goal.status === "active" &&
    goalContinuationSuppressed(input.goal) &&
    !input.isGenerating) {
    return "warning";
  }
  return null;
}

function isIdleActiveGoal(input: GoalStatusProjectionInput): boolean {
  return input.goal.status === "active" && !input.isGenerating;
}

function buildGoalUsageLabel(goal: Goal): string | null {
  if (
    !goal.usage
    || (goal.status === "complete" && goal.usage_finalized !== true)
  ) {
    return null;
  }
  const actual = goalActualTokens(goal);
  if (actual <= 0) {
    return null;
  }
  const actualLabel = `${goalActualTokensEstimated(goal) ? "≈" : ""}${actual.toLocaleString()}`;
  return `${actualLabel} tokens`;
}

export function buildGoalActivityKey(
  messageCount: number,
  isLoading: boolean,
  refreshSequence: number,
): string {
  return `${messageCount}:${isLoading ? "loading" : "idle"}:${refreshSequence}`;
}

export function buildGoalDraftFormModel({
  disabled,
  isLoading,
  loadingLabel,
  objective,
}: {
  disabled: boolean;
  isLoading: boolean;
  loadingLabel: string | null;
  objective: string;
}): GoalDraftFormModel {
  const hasObjective = objective.trim().length > 0;
  const fieldsDisabled = disabled || isLoading;
  return {
    canClose: !fieldsDisabled,
    fieldsDisabled,
    isLoading,
    submitDisabled: fieldsDisabled || !hasObjective,
    submitLabel: isLoading ? loadingLabel ?? "保存中" : "保存",
    submitTone: hasObjective ? "primary" : "default",
  };
}

export function buildGoalControllerProjection({
  executionBinding,
  dialog,
  draft,
  goal,
  phase,
}: {
  dialog: GoalDialog;
  draft: GoalDraft | null;
  executionBinding: GoalExecutionBinding | null;
  goal: Goal | null;
  phase: GoalCommandPhase | null;
}): GoalControllerProjection {
  const clearDisabledReason = goal
    ? resolveGoalClearDisabledReason(executionBinding)
    : null;
  return {
    canResume: goal ? canResumeGoal(goal) : false,
    clearDisabledReason,
    dialog: visibleGoalDialog(dialog, goal, clearDisabledReason === null),
    draft: draft?.goalId === goal?.id ? draft : null,
    loadingLabel: phase === "updating" ? "正在更新目标" : null,
  };
}

export function resolveGoalClearDisabledReason(
  binding: GoalExecutionBinding | null,
): string | null {
  if (!binding) {
    return "正在确认 Goal 与工作图的绑定状态，暂时不能清除。";
  }
  switch (binding.state) {
    case "standalone":
    case "reserved":
      return null;
    case "pending":
      return "Goal 与工作图的绑定正在确认，暂时不能清除。";
    case "confirmed":
      return "Goal 已绑定工作图，请先完成或终止工作图。";
    case "conflict":
      return "Goal 与工作图的绑定存在冲突，请刷新后检查工作图。";
  }
}

export function createGoalDraft(goal: Goal): GoalDraft {
  return {
    budget: goal.token_budget ? String(goal.token_budget) : "",
    goalId: goal.id,
    objective: goal.objective,
  };
}

export function nextGoalBudgetInput(
  goal: Goal,
  value: string,
): number | null | undefined {
  if (value.trim()) {
    return normalizeGoalBudget(value);
  }
  return goal.token_budget ? null : undefined;
}

function normalizeGoalBudget(value: string): number | null {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function canResumeGoal(goal: Goal): boolean {
  return RESUMABLE_GOAL_STATUSES.has(goal.status)
    || (goal.status === "active" && goalContinuationSuppressed(goal));
}

function goalContinuationSuppressed(goal: Goal): boolean {
  return goal.continuation_state === "suspended";
}

function visibleGoalDialog(
  dialog: GoalDialog,
  goal: Goal | null,
  clearAllowed: boolean,
): GoalDialog {
  if (
    dialog.kind !== "clear"
    || !goal
    || dialog.goal.id !== goal.id
    || !clearAllowed
  ) {
    return EMPTY_GOAL_DIALOG;
  }
  return dialog;
}
