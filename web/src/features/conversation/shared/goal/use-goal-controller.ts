"use client";

/**
 * INPUT: Goal/binding/reliability resource, scope-bound UI draft/dialog state and user lifecycle actions.
 * OUTPUT: Goal panel controller with exact mutation intents, stale-read gates and safe read-only recovery.
 * POS: Goal interaction orchestrator; the backend remains the final mutation authority.
 */

import {
  type FormEvent,
  useCallback,
  useEffect,
  useState,
} from "react";

import {
  clearGoalApi,
  pauseGoalApi,
  resumeGoalApi,
  updateGoalApi,
} from "@/lib/api/conversation/goal-api";
import type { Goal } from "@/types/conversation/goal";

import {
  buildGoalControllerProjection,
  createGoalDraft,
  EMPTY_GOAL_DIALOG,
  nextGoalBudgetInput,
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
  const [draft, setDraft] = useState<GoalDraft | null>(null);
  const [dialog, setDialog] = useState<GoalDialog>(EMPTY_GOAL_DIALOG);

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
  const projection = buildGoalControllerProjection({
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
      || mutationsBlocked
      || projection.clearDisabledReason
    ) {
      return;
    }
    const outcome = await runCommand(
      { operation: "clear" },
      async (goalId) => {
        await clearGoalApi(goalId);
        // A successful response means the exact target is no longer available:
        // `cleared=false` is the service's concurrent-already-absent outcome.
        return null;
      },
    );
    if (outcome.ok && !outcome.goal) {
      setDraft(null);
    }
  }, [
    disabled,
    goal,
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
      || disabled
      || mutationsBlocked
    ) {
      return;
    }
    const objective = currentDraft.objective.trim();
    const tokenBudget = nextGoalBudgetInput(goal, currentDraft.budget);
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
      setDraft(null);
    }
  }, [disabled, goal, mutationsBlocked, projection.draft, runCommand]);

  const confirmDialog = useCallback(() => {
    const currentDialog = projection.dialog;
    setDialog(EMPTY_GOAL_DIALOG);
    if (currentDialog.kind === "clear") {
      void clearGoal();
    }
  }, [clearGoal, projection.dialog]);

  useEffect(() => {
    void refresh();
  }, [activityKey, refresh]);

  useEffect(() => {
    setDraft(null);
    setDialog(EMPTY_GOAL_DIALOG);
  }, [ownerScopeGeneration, sessionKey]);

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
  }, [reliability?.kind, reliability?.operation]);

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
        if (!disabled && !mutationsBlocked) {
          void runCommand({ operation: "pause" }, pauseGoalApi);
        }
      },
      refresh: () => void refresh(),
      resume: () => {
        if (!disabled && !mutationsBlocked) {
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
        if (goal && !mutationsBlocked && !projection.clearDisabledReason) {
          setDialog({ goal, kind: "clear" });
        }
      },
      startEditing: () => {
        if (goal && !mutationsBlocked) {
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
    loadingLabel: projection.loadingLabel,
    mutationBlockReason,
    mutationsBlocked,
    reliability,
  };
}
