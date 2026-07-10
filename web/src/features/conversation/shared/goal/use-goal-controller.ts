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
} from "@/lib/api/goal-api";
import type { Goal, GoalStatus } from "@/types/conversation/goal";

import { useGoalResource } from "./use-goal-resource";

interface GoalDraft {
  budget: string;
  goalId: string;
  objective: string;
}

type GoalDialog =
  | { kind: "clear"; goalId: string }
  | { kind: "none" }
  | { goal: Goal; kind: "resume" };

interface GoalControllerOptions {
  activityKey?: number | string | null;
  disabled: boolean;
  onGoalChange?: (goal: Goal | null) => void;
  sessionKey: string | null;
}

const EMPTY_DIALOG: GoalDialog = { kind: "none" };
const RESUMABLE_STATUSES: GoalStatus[] = ["blocked", "paused", "usage_limited"];

function normalizeBudget(value: string): number | null {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function nextBudgetInput(
  goal: Goal,
  value: string,
): number | null | undefined {
  if (value.trim()) {
    return normalizeBudget(value);
  }
  return goal.token_budget ? null : undefined;
}

function shouldPromptResumeGoal(status: GoalStatus): boolean {
  return status === "blocked" || status === "usage_limited";
}

function canResumeGoal(goal: Goal): boolean {
  return RESUMABLE_STATUSES.includes(goal.status)
    || (goal.status === "active" && (goal.empty_progress_count ?? 0) > 0);
}

function resumePromptKey(goal: Goal): string {
  return `${goal.id}:${goal.status}:${goal.updated_at}`;
}

export function useGoalController({
  activityKey = null,
  disabled,
  onGoalChange,
  sessionKey,
}: GoalControllerOptions) {
  const [draft, setDraft] = useState<GoalDraft | null>(null);
  const [dialog, setDialog] = useState<GoalDialog>(EMPTY_DIALOG);
  const resumePromptKeyRef = useRef<string | null>(null);

  const syncResumeDialog = useCallback((current: Goal | null) => {
    if (!current || disabled || !shouldPromptResumeGoal(current.status)) {
      setDialog((value) => value.kind === "resume" ? EMPTY_DIALOG : value);
      return;
    }
    const key = resumePromptKey(current);
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
  const visibleDraft = draft?.goalId === goal?.id ? draft : null;
  const dialogGoalId = dialog.kind === "none"
    ? null
    : dialog.kind === "resume" ? dialog.goal.id : dialog.goalId;
  const dialogMatchesGoal = dialog.kind === "none" || dialogGoalId === goal?.id;
  const visibleDialog = dialogMatchesGoal && !(disabled && dialog.kind === "resume")
    ? dialog
    : EMPTY_DIALOG;

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
    if (!goal || !visibleDraft?.objective.trim() || disabled) {
      return;
    }
    const outcome = await runCommand(
      "updating",
      (goalId) => updateGoalApi(goalId, {
        objective: visibleDraft.objective.trim(),
        token_budget: nextBudgetInput(goal, visibleDraft.budget),
      }),
      "Goal 保存失败",
    );
    if (outcome.ok) {
      setDraft(null);
    }
  }, [disabled, goal, runCommand, visibleDraft]);

  const confirmDialog = useCallback(() => {
    const currentDialog = visibleDialog;
    setDialog(EMPTY_DIALOG);
    if (currentDialog.kind === "clear") {
      void clearGoal();
    }
    if (currentDialog.kind === "resume") {
      void runCommand("resuming", resumeGoalApi, "Goal 操作失败");
    }
  }, [clearGoal, runCommand, visibleDialog]);

  useEffect(() => {
    void refresh();
  }, [activityKey, refresh]);

  useEffect(() => {
    onGoalChange?.(goal);
  }, [goal, onGoalChange]);

  return {
    actions: {
      cancelDialog: () => setDialog(EMPTY_DIALOG),
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
      setBudget: (budget: string) => setDraft((current) => current
        ? { ...current, budget }
        : current),
      setObjective: (objective: string) => setDraft((current) => current
        ? { ...current, objective }
        : current),
      startClearing: () => goal && setDialog({ goalId: goal.id, kind: "clear" }),
      startEditing: () => goal && setDraft({
        budget: goal.token_budget ? String(goal.token_budget) : "",
        goalId: goal.id,
        objective: goal.objective,
      }),
      submit,
    },
    canResume: goal ? canResumeGoal(goal) : false,
    dialog: visibleDialog,
    draft: visibleDraft,
    error,
    goal,
    isAvailable: available,
    isLoading,
    loadingLabel: phase === "updating" ? "正在更新目标" : null,
  };
}
