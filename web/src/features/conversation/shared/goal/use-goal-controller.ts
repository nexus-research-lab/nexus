"use client";

import {
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
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
  goalResumePromptKey,
  nextGoalBudgetInput,
  shouldPromptResumeGoal,
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
  const resumePromptKeyRef = useRef<string | null>(null);

  const syncResumeDialog = useCallback((current: Goal | null) => {
    if (!current || disabled || !shouldPromptResumeGoal(current.status)) {
      setDialog((value) => value.kind === "resume" ? EMPTY_GOAL_DIALOG : value);
      return;
    }
    const key = goalResumePromptKey(current);
    if (resumePromptKeyRef.current === key) {
      return;
    }
    resumePromptKeyRef.current = key;
    setDialog((value) => value.kind === "clear"
      ? value
      : { goal: current, kind: "resume" });
  }, [disabled]);

  const resource = useGoalResource({
    onGoalResolved: syncResumeDialog,
    sessionKey,
  });
  const {
    available,
    error,
    goal,
    isLoading,
    phase,
    refresh,
    runCommand,
  } = resource;
  const projection = buildGoalControllerProjection({
    dialog,
    disabled,
    draft,
    goal,
    phase,
  });

  const clearGoal = useCallback(async () => {
    if (!goal || disabled) {
      return;
    }
    const outcome = await runCommand(
      "clearing",
      async (goalId) => {
        const result = await clearGoalApi(goalId);
        return result.cleared ? null : goal;
      },
      "Goal 操作失败",
    );
    if (outcome.ok && !outcome.goal) {
      setDraft(null);
    }
  }, [disabled, goal, runCommand]);

  const submit = useCallback(async (event: FormEvent) => {
    event.preventDefault();
    const currentDraft = projection.draft;
    if (!goal || !currentDraft?.objective.trim() || disabled) {
      return;
    }
    const outcome = await runCommand(
      "updating",
      (goalId) => updateGoalApi(goalId, {
        objective: currentDraft.objective.trim(),
        token_budget: nextGoalBudgetInput(goal, currentDraft.budget),
      }),
      "Goal 保存失败",
    );
    if (outcome.ok) {
      setDraft(null);
    }
  }, [disabled, goal, projection.draft, runCommand]);

  const confirmDialog = useCallback(() => {
    const currentDialog = projection.dialog;
    setDialog(EMPTY_GOAL_DIALOG);
    if (currentDialog.kind === "clear") {
      void clearGoal();
    }
    if (currentDialog.kind === "resume") {
      void runCommand("resuming", resumeGoalApi, "Goal 操作失败");
    }
  }, [clearGoal, projection.dialog, runCommand]);

  useEffect(() => {
    void refresh();
  }, [activityKey, refresh]);

  useEffect(() => {
    onGoalChange?.(goal);
  }, [goal, onGoalChange]);

  return {
    actions: {
      cancelDialog: () => setDialog(EMPTY_GOAL_DIALOG),
      cancelEditing: () => setDraft(null),
      confirmDialog,
      pause: () => {
        if (!disabled) {
          void runCommand("pausing", pauseGoalApi, "Goal 操作失败");
        }
      },
      refresh: () => void refresh(),
      resume: () => {
        if (!disabled) {
          void runCommand("resuming", resumeGoalApi, "Goal 操作失败");
        }
      },
      setBudget: (budget: string) => setDraft((current) => (
        updateGoalDraft(current, { budget })
      )),
      setObjective: (objective: string) => setDraft((current) => (
        updateGoalDraft(current, { objective })
      )),
      startClearing: () => goal && setDialog({ goal, kind: "clear" }),
      startEditing: () => goal && setDraft(createGoalDraft(goal)),
      submit,
    },
    canResume: projection.canResume,
    dialog: projection.dialog,
    draft: projection.draft,
    error,
    goal,
    isAvailable: available,
    isLoading,
    loadingLabel: projection.loadingLabel,
  };
}
