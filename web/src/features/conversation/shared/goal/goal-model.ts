/**
 * INPUT: Goal state, server-derived Execution binding, locale, UI command phase and complete budget input.
 * OUTPUT: 本地化 Goal 生命周期、保守未知态、完整预算校验与目标正文绑定的确认投影。
 * POS: Goal 纯模型；metadata 不参与 WorkGraph binding，也不输出布局或视觉 class。
 */
import type {
  Goal,
  GoalExecutionBinding,
  GoalExecutionBindingState,
  GoalStatus,
} from "@/types/conversation/goal";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { Locale, TranslationKey } from "@/shared/i18n/messages";
import type { UiBadgeTone } from "@/shared/ui/display/badge-styles";
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

interface GoalControllerProjection {
  canResume: boolean;
  clearDisabledReason: string | null;
  dialog: GoalDialog;
  draft: GoalDraft | null;
  loadingLabel: string | null;
}

interface GoalDraftFormModel {
  budgetInvalid: boolean;
  canClose: boolean;
  fieldsDisabled: boolean;
  isLoading: boolean;
  submitDisabled: boolean;
  submitLabel: string;
  submitTone: "default" | "primary";
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
  tone: UiBadgeTone;
  usageLabel: string | null;
}

interface GoalStatusProjectionInput {
  canResume: boolean;
  clearDisabledReason?: string | null;
  continuationHold: GoalContinuationHold | null;
  executionBinding?: GoalExecutionBinding | null;
  goal: Goal;
  isGenerating: boolean;
  locale: Locale;
  t: I18nContextValue["t"];
}

type GoalBindingDisplayState =
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
  status: GoalStatus | "unknown";
}

interface GoalActionRule {
  action: GoalStatusAction;
  visible: (input: GoalStatusProjectionInput) => boolean;
}

const GOAL_STATUS_LABEL: Record<GoalStatus, TranslationKey> = {
  active: "goal.status_active",
  blocked: "goal.status_blocked",
  budget_limited: "goal.status_budget_limited",
  complete: "goal.status_complete",
  paused: "goal.status_paused",
  usage_limited: "goal.status_usage_limited",
};

const GOAL_STATUS_TONE: Record<GoalStatus, UiBadgeTone> = {
  active: "active",
  blocked: "danger",
  budget_limited: "danger",
  complete: "info",
  paused: "warning",
  usage_limited: "danger",
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

function goalActualTokens(goal: Goal | null): number {
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

function goalStatusTone(status: GoalStatus | "unknown"): UiBadgeTone {
  return Object.hasOwn(GOAL_STATUS_TONE, status)
    ? GOAL_STATUS_TONE[status as GoalStatus]
    : "idle";
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
    actions: visibleStatus.status === "unknown"
      ? ["refresh"]
      : GOAL_ACTION_RULES.filter((rule) => rule.visible(activeInput)).map((rule) => rule.action),
    attentionMessage: resolveGoalAttentionMessage(activeInput),
    attentionTone: resolveGoalAttentionTone(activeInput),
    bindingBadge: resolveGoalBindingBadgeModel(input.executionBinding ?? null),
    statusLabel: visibleStatus.label,
    statusTitle: resolveGoalStatusTitle(activeInput, visibleStatus),
    tone: goalStatusTone(visibleStatus.status),
    usageLabel: buildGoalUsageLabel(input.goal, input.locale),
  };
}

function resolveGoalBindingBadgeModel(
  binding: GoalExecutionBinding | null,
): GoalBindingBadgeModel | null {
  if (binding?.state === "standalone" || binding?.state === "reserved") {
    return null;
  }
  const state = binding?.state ?? "unavailable";
  return Object.hasOwn(GOAL_BINDING_BADGE, state)
    ? GOAL_BINDING_BADGE[state]
    : GOAL_BINDING_BADGE.unavailable;
}

function resolveVisibleGoalStatus(
  input: GoalStatusProjectionInput,
): VisibleGoalStatus {
  if (!Object.hasOwn(GOAL_STATUS_LABEL, input.goal.status)) {
    return { label: input.t("goal.status_unknown"), status: "unknown" };
  }
  if (input.goal.status === "active" && input.isGenerating) {
    return { label: input.t("goal.status_executing"), status: "active" };
  }
  if (!isIdleActiveGoal(input)) {
    return {
      label: input.t(GOAL_STATUS_LABEL[input.goal.status]),
      status: input.goal.status,
    };
  }
  if (input.goal.last_error) {
    return { label: input.t("goal.status_attention"), status: "blocked" };
  }
  if (input.continuationHold) {
    return { label: input.continuationHold.label, status: "paused" };
  }
  if (goalContinuationSuppressed(input.goal)) {
    return { label: input.t("goal.continuation_stopped"), status: "paused" };
  }
  return {
    label: input.t(GOAL_STATUS_LABEL[input.goal.status]),
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
    return input.t("goal.continuation_stopped_title");
  }
  return visibleStatus.label;
}

function resolveGoalAttentionMessage(
  input: GoalStatusProjectionInput,
): string | null {
  if (input.goal.status === "active" &&
    goalContinuationSuppressed(input.goal) &&
    !input.isGenerating) {
    return input.t("goal.continuation_stopped_detail");
  }
  return null;
}

function resolveGoalAttentionTone(
  input: GoalStatusProjectionInput,
): GoalStatusStripModel["attentionTone"] {
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

function buildGoalUsageLabel(goal: Goal, locale: Locale): string | null {
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
  const actualLabel = `${goalActualTokensEstimated(goal) ? "≈" : ""}${actual.toLocaleString(locale)}`;
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
  budget,
  disabled,
  isLoading,
  loadingLabel,
  mutationBlocked,
  objective,
  t,
}: {
  budget: string;
  disabled: boolean;
  isLoading: boolean;
  loadingLabel: string | null;
  mutationBlocked?: boolean;
  objective: string;
  t: I18nContextValue["t"];
}): GoalDraftFormModel {
  const hasObjective = objective.trim().length > 0;
  const commandBusy = disabled || isLoading;
  const fieldsDisabled = commandBusy || Boolean(mutationBlocked);
  const budgetInvalid = !parseGoalBudgetInput(budget).valid;
  return {
    budgetInvalid,
    canClose: !commandBusy,
    fieldsDisabled,
    isLoading,
    submitDisabled: fieldsDisabled || !hasObjective || budgetInvalid,
    submitLabel: isLoading ? loadingLabel ?? t("common.saving") : t("common.save"),
    submitTone: hasObjective ? "primary" : "default",
  };
}

export function buildGoalControllerProjection({
  executionBinding,
  dialog,
  draft,
  goal,
  phase,
  t,
}: {
  dialog: GoalDialog;
  draft: GoalDraft | null;
  executionBinding: GoalExecutionBinding | null;
  goal: Goal | null;
  phase: GoalCommandPhase | null;
  t: I18nContextValue["t"];
}): GoalControllerProjection {
  const clearDisabledReason = goal
    ? resolveGoalClearDisabledReason(executionBinding, t)
    : null;
  return {
    canResume: goal ? canResumeGoal(goal) : false,
    clearDisabledReason,
    dialog: visibleGoalDialog(dialog, goal, clearDisabledReason === null),
    draft: draft?.goalId === goal?.id ? draft : null,
    loadingLabel: phase === "updating" ? t("goal.updating") : null,
  };
}

export function resolveGoalClearDisabledReason(
  binding: GoalExecutionBinding | null,
  t: I18nContextValue["t"],
): string | null {
  if (!binding) {
    return t("goal.clear_unavailable");
  }
  switch (binding.state) {
    case "standalone":
    case "reserved":
      return null;
    case "pending":
      return t("goal.clear_pending");
    case "confirmed":
      return t("goal.clear_confirmed");
    case "conflict":
      return t("goal.clear_conflict");
    default:
      return t("goal.clear_unavailable");
  }
}

export function createGoalDraft(goal: Goal): GoalDraft {
  return {
    budget: goal.token_budget ? String(goal.token_budget) : "",
    goalId: goal.id,
    objective: goal.objective,
  };
}

/** Blank removes a budget; invalid text must never become a different budget or removal. */
export function parseGoalBudgetInput(value: string):
  | { valid: true; value: number | null }
  | { valid: false } {
  const input = value.trim();
  if (!input) return { valid: true, value: null };
  if (!/^\d+$/.test(input)) return { valid: false };
  const parsed = Number(input);
  return Number.isSafeInteger(parsed) && parsed > 0
    ? { valid: true, value: parsed }
    : { valid: false };
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
    || dialog.goal.objective !== goal.objective
    || !clearAllowed
  ) {
    return EMPTY_GOAL_DIALOG;
  }
  return dialog;
}
