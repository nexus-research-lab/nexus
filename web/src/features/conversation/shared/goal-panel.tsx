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
  clear_goal_api,
  get_current_goal_api,
  pause_goal_api,
  resume_goal_api,
  update_goal_api,
} from "@/lib/api/goal-api";
import { ApiRequestError } from "@/lib/api/http";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import type { Goal, GoalStatus } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import { GoalDraftForm } from "./goal-panel-draft-form";
import { GoalStatusStrip } from "./goal-panel-status-strip";

type GoalDraftSavePhase = "idle" | "updating";

interface GoalPanelProps {
  session_key: string | null;
  compact?: boolean;
  disabled?: boolean;
  activity_key?: string | number | null;
  continuation_hold?: GoalContinuationHold | null;
  is_generating?: boolean;
  scope_label?: string;
  status_extra?: ReactNode;
  on_goal_change?: (goal: Goal | null) => void;
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
  session_key: sessionKey,
  compact = false,
  continuation_hold: continuationHold = null,
  disabled = false,
  activity_key: activityKey = null,
  is_generating: isGenerating = false,
  scope_label: scopeLabel = "会话 Goal",
  status_extra: statusExtra = null,
  on_goal_change: onGoalChange,
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
      const current = await get_current_goal_api(sessionKey);
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
      const updated = await update_goal_api(goal.id, {
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
      const result = await clear_goal_api(goal.id);
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
    void mutateGoal(resume_goal_api);
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
        can_resume={canResumeCurrentGoal}
        compact={compact}
        continuation_hold={continuationHold}
        disabled={disabled}
        error={error}
        goal={goal}
        is_generating={isGenerating}
        is_loading={isLoading}
        scope_label={scopeLabel}
        status_extra={statusExtra}
        on_clear_request={() => setIsClearConfirmOpen(true)}
        on_edit={startEditingGoal}
        on_pause={() => void mutateGoal(pause_goal_api)}
        on_refresh={() => void refreshGoal()}
        on_resume={() => void mutateGoal(resume_goal_api)}
      />
      {isEditing ? (
        <GoalDraftForm
          budget={budget}
          disabled={disabled}
          error={error}
          is_loading={isLoading}
          loading_label={draftSaveLoadingLabel(draftSavePhase)}
          objective={objective}
          on_budget_change={setBudget}
          on_cancel={cancelEditingGoal}
          on_objective_change={setObjective}
          on_submit={submitGoal}
        />
      ) : null}
      <ConfirmDialog
        cancel_text="取消"
        confirm_text="清除"
        is_open={isClearConfirmOpen}
        message={`Goal：${goal.objective}`}
        title="清除当前 Goal?"
        variant="danger"
        on_cancel={() => setIsClearConfirmOpen(false)}
        on_confirm={confirmClearGoal}
      />
      <ConfirmDialog
        cancel_text="暂不继续"
        confirm_text="继续"
        is_open={resumePromptGoal !== null}
        message={`Goal：${resumePromptGoal?.objective ?? ""}`}
        title="继续当前 Goal?"
        on_cancel={cancelResumePrompt}
        on_confirm={confirmResumePrompt}
      />
    </>
  );
}
