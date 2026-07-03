"use client";

import {
  FormEvent,
  ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  clearGoalApi,
  getCurrentGoalApi,
  pauseGoalApi,
  resumeGoalApi,
  updateGoalApi,
} from "@/lib/api/goal-api";
import { ApiRequestError } from "@/lib/api/http";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import type { Goal, GoalStatus } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import { GoalDraftForm } from "./goal-panel-draft-form";
import { GoalStatusStrip } from "./goal-panel-status-strip";

type GoalDraftSavePhase = "idle" | "updating";

interface GoalPanelProps {
  sessionKey: string | null;
  compact?: boolean;
  disabled?: boolean;
  activityKey?: string | number | null;
  continuationHold?: GoalContinuationHold | null;
  isGenerating?: boolean;
  scopeLabel?: string;
  statusExtra?: ReactNode;
  onGoalChange?: (goal: Goal | null) => void;
}

function isGoalUnavailable(error: unknown) {
  return error instanceof ApiRequestError && error.status === 403;
}

function isGoalMissing(error: unknown) {
  return error instanceof ApiRequestError && error.status === 404;
}

function normalizeBudget(value: string): number | null {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function nextBudgetInput(goal: Goal | null, value: string): number | null | undefined {
  if (value.trim() !== "") {
    return normalizeBudget(value);
  }
  return goal?.token_budget ? null : undefined;
}

function shouldPromptResumeGoal(status: GoalStatus): boolean {
  return status === "blocked" || status === "usage_limited";
}

function canResumeStatus(status: GoalStatus): boolean {
  return status === "paused" || status === "blocked" || status === "usage_limited";
}

function canResumeGoal(goal: Goal): boolean {
  return (
    canResumeStatus(goal.status) ||
    (goal.status === "active" && (goal.empty_progress_count ?? 0) > 0)
  );
}

function draftSaveLoadingLabel(phase: GoalDraftSavePhase): string | null {
  switch (phase) {
    case "updating":
      return "正在更新目标";
    default:
      return null;
  }
}

function resumePromptKey(goal: Goal): string {
  return `${goal.id}:${goal.status}:${goal.updated_at}`;
}

export function GoalPanel({
  sessionKey: sessionKey,
  compact = false,
  continuationHold: continuationHold = null,
  disabled = false,
  activityKey: activityKey = null,
  isGenerating: isGenerating = false,
  scopeLabel: scopeLabel = "会话 Goal",
  statusExtra: statusExtra = null,
  onGoalChange: onGoalChange,
}: GoalPanelProps) {
  const [goal, setGoal] = useState<Goal | null>(null);
  const [isAvailable, setIsAvailable] = useState(true);
  const [isLoading, setIsLoading] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [draftSavePhase, setDraftSavePhase] =
    useState<GoalDraftSavePhase>("idle");
  const [objective, setObjective] = useState("");
  const [budget, setBudget] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [resumePromptGoal, setResumePromptGoal] = useState<Goal | null>(null);
  const [isClearConfirmOpen, setIsClearConfirmOpen] = useState(false);
  const resumePromptKeyRef = useRef<string | null>(null);

  useEffect(() => {
    onGoalChange?.(goal);
  }, [goal, onGoalChange]);

  const maybePromptResumeGoal = useCallback(
    (current: Goal) => {
      if (disabled || !shouldPromptResumeGoal(current.status)) {
        setResumePromptGoal(null);
        return;
      }
      const key = resumePromptKey(current);
      if (resumePromptKeyRef.current === key) {
        return;
      }
      resumePromptKeyRef.current = key;
      setResumePromptGoal(current);
    },
    [disabled],
  );

  const refreshGoal = useCallback(async () => {
    if (!sessionKey) {
      setGoal(null);
      setIsEditing(false);
      return;
    }
    setIsLoading(true);
    try {
      const current = await getCurrentGoalApi(sessionKey);
      if (!current) {
        setGoal(null);
        setResumePromptGoal(null);
        setIsAvailable(true);
        setError(null);
        return;
      }
      setGoal(current);
      maybePromptResumeGoal(current);
      setIsAvailable(true);
      setError(null);
    } catch (err) {
      if (isGoalUnavailable(err)) {
        setIsAvailable(false);
        setGoal(null);
        setIsEditing(false);
        setResumePromptGoal(null);
        return;
      }
      if (isGoalMissing(err)) {
        setGoal(null);
        setResumePromptGoal(null);
        setError(null);
        return;
      }
      setError(err instanceof Error ? err.message : "Goal 状态读取失败");
    } finally {
      setIsLoading(false);
    }
  }, [maybePromptResumeGoal, sessionKey]);

  useEffect(() => {
    void refreshGoal();
  }, [refreshGoal, activityKey]);

  const beginEditingGoal = useCallback((current: Goal) => {
    setObjective(current.objective);
    setBudget(current.token_budget ? String(current.token_budget) : "");
    setIsEditing(true);
  }, []);

  const submitGoal = async (event: FormEvent) => {
    event.preventDefault();
    if (!sessionKey || !goal || !objective.trim()) return;
    setError(null);
    setDraftSavePhase("updating");
    setIsLoading(true);
    try {
      const tokenBudget = nextBudgetInput(goal, budget);
      const updated = await updateGoalApi(goal.id, {
        objective: objective.trim(),
        token_budget: tokenBudget,
      });
      setGoal(updated);
      setObjective("");
      setBudget("");
      setIsEditing(false);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Goal 保存失败");
    } finally {
      setDraftSavePhase("idle");
      setIsLoading(false);
    }
  };

  const mutateGoal = async (action: (goalId: string) => Promise<Goal>) => {
    if (!goal || disabled) return;
    setIsLoading(true);
    try {
      const updated = await action(goal.id);
      setGoal(updated);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Goal 操作失败");
    } finally {
      setIsLoading(false);
    }
  };

  const clearCurrentGoal = async () => {
    if (!goal || disabled) return;
    setIsLoading(true);
    try {
      const result = await clearGoalApi(goal.id);
      if (result.cleared) {
        setGoal(null);
        setIsEditing(false);
      }
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Goal 操作失败");
    } finally {
      setIsLoading(false);
    }
  };

  const confirmResumePrompt = () => {
    setResumePromptGoal(null);
    void mutateGoal(resumeGoalApi);
  };

  const cancelResumePrompt = () => {
    setResumePromptGoal(null);
  };

  const confirmClearGoal = () => {
    setIsClearConfirmOpen(false);
    void clearCurrentGoal();
  };

  const startEditingGoal = () => {
    if (!goal) return;
    beginEditingGoal(goal);
  };

  const cancelEditingGoal = () => {
    setObjective("");
    setBudget("");
    setDraftSavePhase("idle");
    setIsEditing(false);
  };

  const canResumeCurrentGoal = useMemo(
    () => (goal ? canResumeGoal(goal) : false),
    [goal],
  );

  if (!isAvailable || !sessionKey) {
    return null;
  }

  if (!goal) {
    return null;
  }

  return (
    <>
      <GoalStatusStrip
        canResume={canResumeCurrentGoal}
        compact={compact}
        continuationHold={continuationHold}
        disabled={disabled}
        error={error}
        goal={goal}
        isGenerating={isGenerating}
        isLoading={isLoading}
        scopeLabel={scopeLabel}
        statusExtra={statusExtra}
        onClearRequest={() => setIsClearConfirmOpen(true)}
        onEdit={startEditingGoal}
        onPause={() => void mutateGoal(pauseGoalApi)}
        onRefresh={() => void refreshGoal()}
        onResume={() => void mutateGoal(resumeGoalApi)}
      />
      {isEditing ? (
        <GoalDraftForm
          budget={budget}
          disabled={disabled}
          error={error}
          isLoading={isLoading}
          loadingLabel={draftSaveLoadingLabel(draftSavePhase)}
          objective={objective}
          onBudgetChange={setBudget}
          onCancel={cancelEditingGoal}
          onObjectiveChange={setObjective}
          onSubmit={submitGoal}
        />
      ) : null}
      <ConfirmDialog
        cancelText="取消"
        confirmText="清除"
        isOpen={isClearConfirmOpen}
        message={`Goal：${goal.objective}`}
        title="清除当前 Goal?"
        variant="danger"
        onCancel={() => setIsClearConfirmOpen(false)}
        onConfirm={confirmClearGoal}
      />
      <ConfirmDialog
        cancelText="暂不继续"
        confirmText="继续"
        isOpen={resumePromptGoal !== null}
        message={`Goal：${resumePromptGoal?.objective ?? ""}`}
        title="继续当前 Goal?"
        onCancel={cancelResumePrompt}
        onConfirm={confirmResumePrompt}
      />
    </>
  );
}
