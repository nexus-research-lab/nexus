"use client";

import {
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  clear_goal_api,
  create_goal_api,
  get_current_goal_api,
  pause_goal_api,
  resume_goal_api,
  update_goal_api,
} from "@/lib/api/goal-api";
import { ApiRequestError } from "@/lib/api/http";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import type { Goal, GoalStatus } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import {
  GoalDraftForm,
  GoalEmptyStrip,
  GoalStartLauncher,
  GoalStatusStrip,
} from "./goal-panel-view";

type GoalPanelEmptyStateVariant = "hidden" | "launcher" | "strip";
type GoalDraftSavePhase = "idle" | "creating" | "updating";

interface GoalPanelProps {
  session_key: string | null;
  compact?: boolean;
  disabled?: boolean;
  activity_key?: string | number | null;
  continuation_hold?: GoalContinuationHold | null;
  empty_state_variant?: GoalPanelEmptyStateVariant;
  is_generating?: boolean;
  scope_label?: string;
}

function is_goal_unavailable(error: unknown) {
  return error instanceof ApiRequestError && error.status === 403;
}

function is_goal_missing(error: unknown) {
  return error instanceof ApiRequestError && error.status === 404;
}

function normalize_budget(value: string): number | null {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function next_budget_input(goal: Goal | null, value: string): number | null | undefined {
  if (value.trim() !== "") {
    return normalize_budget(value);
  }
  return goal?.token_budget ? null : undefined;
}

function should_prompt_resume_goal(status: GoalStatus): boolean {
  return status === "blocked" || status === "usage_limited";
}

function can_resume_status(status: GoalStatus): boolean {
  return status === "paused" || status === "blocked" || status === "usage_limited";
}

function can_resume_goal(goal: Goal): boolean {
  return (
    can_resume_status(goal.status) ||
    (goal.status === "active" && (goal.empty_progress_count ?? 0) > 0)
  );
}

function draft_save_loading_label(phase: GoalDraftSavePhase): string | null {
  switch (phase) {
    case "creating":
      return "正在整理目标";
    case "updating":
      return "正在更新目标";
    default:
      return null;
  }
}

function resume_prompt_key(goal: Goal): string {
  return `${goal.id}:${goal.status}:${goal.updated_at}`;
}

export function GoalPanel({
  session_key,
  compact = false,
  continuation_hold = null,
  disabled = false,
  activity_key = null,
  empty_state_variant = "hidden",
  is_generating = false,
  scope_label = "会话 Goal",
}: GoalPanelProps) {
  const [goal, set_goal] = useState<Goal | null>(null);
  const [is_available, set_is_available] = useState(true);
  const [is_loading, set_is_loading] = useState(false);
  const [is_creating, set_is_creating] = useState(false);
  const [is_editing, set_is_editing] = useState(false);
  const [draft_save_phase, set_draft_save_phase] =
    useState<GoalDraftSavePhase>("idle");
  const [objective, set_objective] = useState("");
  const [budget, set_budget] = useState("");
  const [error, set_error] = useState<string | null>(null);
  const [resume_prompt_goal, set_resume_prompt_goal] = useState<Goal | null>(null);
  const [is_clear_confirm_open, set_is_clear_confirm_open] = useState(false);
  const resume_prompt_key_ref = useRef<string | null>(null);

  const maybe_prompt_resume_goal = useCallback(
    (current: Goal) => {
      if (disabled || !should_prompt_resume_goal(current.status)) {
        set_resume_prompt_goal(null);
        return;
      }
      const key = resume_prompt_key(current);
      if (resume_prompt_key_ref.current === key) {
        return;
      }
      resume_prompt_key_ref.current = key;
      set_resume_prompt_goal(current);
    },
    [disabled],
  );

  const refresh_goal = useCallback(async () => {
    if (!session_key) {
      set_goal(null);
      set_is_creating(false);
      set_is_editing(false);
      return;
    }
    set_is_loading(true);
    try {
      const current = await get_current_goal_api(session_key);
      if (!current) {
        set_goal(null);
        set_resume_prompt_goal(null);
        set_is_available(true);
        set_error(null);
        return;
      }
      set_goal(current);
      set_is_creating(false);
      maybe_prompt_resume_goal(current);
      set_is_available(true);
      set_error(null);
    } catch (err) {
      if (is_goal_unavailable(err)) {
        set_is_available(false);
        set_goal(null);
        set_is_creating(false);
        set_is_editing(false);
        set_resume_prompt_goal(null);
        return;
      }
      if (is_goal_missing(err)) {
        set_goal(null);
        set_resume_prompt_goal(null);
        set_error(null);
        return;
      }
      set_error(err instanceof Error ? err.message : "Goal 状态读取失败");
    } finally {
      set_is_loading(false);
    }
  }, [maybe_prompt_resume_goal, session_key]);

  useEffect(() => {
    void refresh_goal();
  }, [refresh_goal, activity_key]);

  const begin_editing_goal = useCallback((current: Goal) => {
    set_objective(current.objective);
    set_budget(current.token_budget ? String(current.token_budget) : "");
    set_is_creating(false);
    set_is_editing(true);
  }, []);

  const submit_goal = async (event: FormEvent) => {
    event.preventDefault();
    if (!session_key || !objective.trim()) return;
    set_error(null);
    set_draft_save_phase(is_editing ? "updating" : "creating");
    set_is_loading(true);
    try {
      const token_budget =
        is_editing ? next_budget_input(goal, budget) : normalize_budget(budget);
      const updated =
        is_editing && goal
          ? await update_goal_api(goal.id, {
              objective: objective.trim(),
              token_budget,
            })
          : await create_goal_api({
              session_key,
              objective: objective.trim(),
              token_budget: token_budget ?? null,
            });
      set_goal(updated);
      set_objective("");
      set_budget("");
      set_is_creating(false);
      set_is_editing(false);
      set_error(null);
    } catch (err) {
      set_error(err instanceof Error ? err.message : "Goal 保存失败");
    } finally {
      set_draft_save_phase("idle");
      set_is_loading(false);
    }
  };

  const mutate_goal = async (action: (goal_id: string) => Promise<Goal>) => {
    if (!goal || disabled) return;
    set_is_loading(true);
    try {
      const updated = await action(goal.id);
      set_goal(updated);
      set_error(null);
    } catch (err) {
      set_error(err instanceof Error ? err.message : "Goal 操作失败");
    } finally {
      set_is_loading(false);
    }
  };

  const clear_current_goal = async () => {
    if (!goal || disabled) return;
    set_is_loading(true);
    try {
      const result = await clear_goal_api(goal.id);
      if (result.cleared) {
        set_goal(null);
        set_is_creating(false);
        set_is_editing(false);
      }
      set_error(null);
    } catch (err) {
      set_error(err instanceof Error ? err.message : "Goal 操作失败");
    } finally {
      set_is_loading(false);
    }
  };

  const confirm_resume_prompt = () => {
    set_resume_prompt_goal(null);
    void mutate_goal(resume_goal_api);
  };

  const cancel_resume_prompt = () => {
    set_resume_prompt_goal(null);
  };

  const confirm_clear_goal = () => {
    set_is_clear_confirm_open(false);
    void clear_current_goal();
  };

  const start_editing_goal = () => {
    if (!goal) return;
    begin_editing_goal(goal);
  };

  const start_creating_goal = () => {
    set_objective("");
    set_budget("");
    set_is_editing(false);
    set_is_creating(true);
  };

  const cancel_editing_goal = () => {
    set_objective("");
    set_budget("");
    set_draft_save_phase("idle");
    set_is_creating(false);
    set_is_editing(false);
  };

  const can_resume_current_goal = useMemo(
    () => (goal ? can_resume_goal(goal) : false),
    [goal],
  );

  if (!is_available || !session_key) {
    return null;
  }

  if (!goal && !is_creating) {
    if (empty_state_variant === "hidden") {
      return null;
    }
    if (empty_state_variant === "launcher") {
      return (
        <GoalStartLauncher
          compact={compact}
          disabled={disabled}
          is_loading={is_loading}
          scope_label={scope_label}
          on_create={start_creating_goal}
        />
      );
    }
    return (
      <GoalEmptyStrip
        compact={compact}
        disabled={disabled}
        error={error}
        is_loading={is_loading}
        scope_label={scope_label}
        on_create={start_creating_goal}
        on_refresh={() => void refresh_goal()}
      />
    );
  }

  if (is_creating || is_editing) {
    return (
      <GoalDraftForm
        budget={budget}
        can_cancel={is_creating}
        compact={compact}
        disabled={disabled}
        error={error}
        is_editing={is_editing}
        is_loading={is_loading}
        loading_label={draft_save_loading_label(draft_save_phase)}
        objective={objective}
        scope_label={scope_label}
        on_budget_change={set_budget}
        on_cancel={cancel_editing_goal}
        on_objective_change={set_objective}
        on_submit={submit_goal}
      />
    );
  }

  if (!goal) {
    return null;
  }

  return (
    <>
      <GoalStatusStrip
        can_resume={can_resume_current_goal}
        compact={compact}
        continuation_hold={continuation_hold}
        disabled={disabled}
        error={error}
        goal={goal}
        is_generating={is_generating}
        is_loading={is_loading}
        scope_label={scope_label}
        on_clear_request={() => set_is_clear_confirm_open(true)}
        on_edit={start_editing_goal}
        on_pause={() => void mutate_goal(pause_goal_api)}
        on_refresh={() => void refresh_goal()}
        on_resume={() => void mutate_goal(resume_goal_api)}
      />
      <ConfirmDialog
        cancel_text="取消"
        confirm_text="清除"
        is_open={is_clear_confirm_open}
        message={`Goal：${goal.objective}`}
        title="清除当前 Goal?"
        variant="danger"
        on_cancel={() => set_is_clear_confirm_open(false)}
        on_confirm={confirm_clear_goal}
      />
      <ConfirmDialog
        cancel_text="暂不继续"
        confirm_text="继续"
        is_open={resume_prompt_goal !== null}
        message={`Goal：${resume_prompt_goal?.objective ?? ""}`}
        title="继续当前 Goal?"
        on_cancel={cancel_resume_prompt}
        on_confirm={confirm_resume_prompt}
      />
    </>
  );
}
