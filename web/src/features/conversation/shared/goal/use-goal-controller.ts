"use client";

/**
 * INPUT: Goal/binding/reliability resource, scope-bound UI draft/dialog state, whole-budget validation and user lifecycle actions.
 * OUTPUT: Goal panel controller with exact mutation intents, stale-read gates and safe read-only recovery.
 * POS: Goal interaction orchestrator; the backend remains the final mutation authority.
 */

import {
  type FormEvent,
  useCallback,
  useEffect,
} from "react";

import {
  clearGoalApi,
  pauseGoalApi,
  resumeGoalApi,
  updateGoalApi,
} from "@/lib/api/conversation/goal-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { Goal } from "@/types/conversation/goal";

import {
  buildGoalControllerProjection,
  createGoalDraft,
  EMPTY_GOAL_DIALOG,
  parseGoalBudgetInput,
  resolveGoalClearDisabledReason,
  type GoalDialog,
  type GoalDraft,
} from "./goal-model";
import { useGoalResource } from "./use-goal-resource";

interface GoalControllerOptions {
  activityKey?: number | string | null;
  disabled: boolean;
  onGoalChange?: (goal: Goal | null) => void;
  sessionKey: string | null;
}

function updateGoalDraft(
  current: GoalDraft | null,
  values: Partial<Pick<GoalDraft, "budget" | "objective">>,
): GoalDraft | null {
  return current ? { ...current, ...values } : null;
}

export function useGoalController({
  activityKey = null,
  disabled,
  onGoalChange,
  sessionKey,
}: GoalControllerOptions) {
  const { t } = useI18n();

  const resource = useGoalResource({
    sessionKey,
  });
  const {
    executionBinding,
    goal,
    isLoading,
    mutationBlockReason,
    mutationsBlocked,
    ownerScopeGeneration,
    phase,
    refresh,
    reliability,
    runCommand,
  } = resource;
  const scopeKey = JSON.stringify([ownerScopeGeneration, sessionKey, goal?.id]);
  const [draft, setDraft] = useResettableState<GoalDraft | null>(null, scopeKey);
  // A confirmation belongs to the displayed objective and current clear permission.
  const [dialog, setDialog] = useResettableState<GoalDialog>(EMPTY_GOAL_DIALOG, JSON.stringify([
    scopeKey, goal?.objective, resolveGoalClearDisabledReason(executionBinding, t) === null,
    disabled || isLoading || mutationsBlocked,
  ]));
  const projection = buildGoalControllerProjection({
    t,
    dialog,
    draft,
    executionBinding,
    goal,
    phase,
  });

  const clearGoal = useCallback(async () => {
    if (
      !goal
      || disabled
      || isLoading
      || mutationsBlocked
      || projection.clearDisabledReason
    ) {
      return;
    }
    await runCommand(
      { operation: "clear" },
      async (goalId) => {
        await clearGoalApi(goalId);
        // A successful response means the exact target is no longer available:
        // `cleared=false` is the service's concurrent-already-absent outcome.
        return null;
      },
    );
  }, [
    disabled,
    goal,
    isLoading,
    mutationsBlocked,
    projection.clearDisabledReason,
    runCommand,
  ]);

  const submit = useCallback(async (event: FormEvent) => {
    event.preventDefault();
    const currentDraft = projection.draft;
    if (
      !goal
      || !currentDraft?.objective.trim()
      || isLoading
      || disabled
      || mutationsBlocked
    ) {
      return;
    }
    const objective = currentDraft.objective.trim();
    const budget = parseGoalBudgetInput(currentDraft.budget);
    if (!budget.valid) return;
    const tokenBudget = budget.value ?? (goal.token_budget ? null : undefined);
    const outcome = await runCommand(
      {
        objective,
        operation: "update",
        tokenBudget,
      },
      (goalId) => updateGoalApi(goalId, {
        objective,
        token_budget: tokenBudget,
      }),
    );
    if (outcome.ok) {
      setDraft((current) => current === currentDraft ? null : current);
    }
  }, [disabled, goal, isLoading, mutationsBlocked, projection.draft, runCommand, setDraft]);

  const confirmDialog = useCallback(() => {
    const currentDialog = projection.dialog;
    setDialog(EMPTY_GOAL_DIALOG);
    if (currentDialog.kind === "clear") {
      void clearGoal();
    }
  }, [clearGoal, projection.dialog, setDialog]);

  useEffect(() => {
    void refresh();
  }, [activityKey, refresh]);

  useEffect(() => {
    if (
      reliability?.operation === "update"
      && (
        reliability.kind === "mutation_applied"
        || reliability.kind === "mutation_committed"
        || reliability.kind === "mutation_committed_refresh_failed"
        || reliability.kind === "mutation_target_not_current"
      )
    ) {
      setDraft(null);
    }
  }, [reliability?.kind, reliability?.operation, setDraft]);

  useEffect(() => {
    if (!isLoading) {
      onGoalChange?.(goal);
    }
  }, [goal, isLoading, onGoalChange]);

  return {
    actions: {
      cancelDialog: () => setDialog(EMPTY_GOAL_DIALOG),
      cancelEditing: () => setDraft(null),
      confirmDialog,
      pause: () => {
        if (!disabled && !isLoading && !mutationsBlocked) {
          void runCommand({ operation: "pause" }, pauseGoalApi);
        }
      },
      refresh: () => void refresh(),
      resume: () => {
        if (!disabled && !isLoading && !mutationsBlocked) {
          void runCommand({ operation: "resume" }, resumeGoalApi);
        }
      },
      setBudget: (budget: string) => setDraft((current) => (
        updateGoalDraft(current, { budget })
      )),
      setObjective: (objective: string) => setDraft((current) => (
        updateGoalDraft(current, { objective })
      )),
      startClearing: () => {
        if (goal && !disabled && !isLoading && !mutationsBlocked && !projection.clearDisabledReason) {
          setDialog({ goal, kind: "clear" });
        }
      },
      startEditing: () => {
        if (goal && !disabled && !isLoading && !mutationsBlocked) {
          setDraft(createGoalDraft(goal));
        }
      },
      submit,
    },
    canResume: projection.canResume,
    clearDisabledReason: projection.clearDisabledReason,
    dialog: projection.dialog,
    draft: projection.draft,
    executionBinding,
    goal,
    isLoading,
    loadingLabel: projection.loadingLabel ?? (isLoading ? t("state.reload_check") : null),
    mutationBlockReason,
    mutationsBlocked,
    pendingAction: phase === "clearing" ? "clear" as const
      : phase === "pausing" ? "pause" as const
      : phase === "resuming" ? "resume" as const
      : phase === "updating" ? "edit" as const : null,
    reliability,
  };
}
