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
  CheckCircle2,
  CircleSlash,
  Clock3,
  GaugeCircle,
  Pause,
  Pencil,
  Play,
  RefreshCw,
  Repeat2,
  Save,
  Target,
  X,
} from "lucide-react";

import {
  clear_goal_api,
  complete_goal_api,
  create_goal_api,
  get_current_goal_api,
  list_goal_events_api,
  pause_goal_api,
  resume_goal_api,
  update_goal_api,
} from "@/lib/api/goal-api";
import { ApiRequestError } from "@/lib/api/http";
import { cn, format_tokens } from "@/lib/utils";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import type { Goal, GoalEvent, GoalStatus } from "@/types/conversation/goal";
import {
  GOAL_STATUS_LABEL,
  goal_budget_percent,
  goal_elapsed_label,
  goal_event_label,
  goal_runtime_label,
  goal_status_tone,
  goal_usage_total,
} from "./goal-panel-model";

interface GoalPanelProps {
  session_key: string | null;
  compact?: boolean;
  disabled?: boolean;
  activity_key?: string | number | null;
  edit_request_key?: string | number | null;
  is_generating?: boolean;
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
  return can_resume_goal(status);
}

function can_resume_goal(status: GoalStatus): boolean {
  return status === "paused" || status === "blocked" || status === "usage_limited";
}

function resume_prompt_key(goal: Goal): string {
  return `${goal.id}:${goal.status}:${goal.updated_at}`;
}

export function GoalPanel({
  session_key,
  compact = false,
  disabled = false,
  activity_key = null,
  edit_request_key = null,
  is_generating = false,
}: GoalPanelProps) {
  const [goal, set_goal] = useState<Goal | null>(null);
  const [events, set_events] = useState<GoalEvent[]>([]);
  const [is_available, set_is_available] = useState(true);
  const [is_loading, set_is_loading] = useState(false);
  const [is_editing, set_is_editing] = useState(false);
  const [objective, set_objective] = useState("");
  const [budget, set_budget] = useState("");
  const [error, set_error] = useState<string | null>(null);
  const [resume_prompt_goal, set_resume_prompt_goal] = useState<Goal | null>(null);
  const [is_clear_confirm_open, set_is_clear_confirm_open] = useState(false);
  const resume_prompt_key_ref = useRef<string | null>(null);
  const edit_request_key_ref = useRef<string | number | null>(null);

  const usage_total = goal_usage_total(goal);
  const budget_value = goal?.token_budget ?? null;
  const usage_percent = useMemo(() => goal_budget_percent(goal), [goal]);

  const refresh_goal_events = useCallback(async (goal_id: string) => {
    try {
      const loaded = await list_goal_events_api(goal_id);
      set_events(loaded.slice(0, 5));
    } catch {
      set_events([]);
    }
  }, []);

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
      set_events([]);
      return;
    }
    set_is_loading(true);
    try {
      const current = await get_current_goal_api(session_key);
      set_goal(current);
      maybe_prompt_resume_goal(current);
      await refresh_goal_events(current.id);
      set_is_available(true);
      set_error(null);
    } catch (err) {
      if (is_goal_unavailable(err)) {
        set_is_available(false);
        set_goal(null);
        set_events([]);
        set_resume_prompt_goal(null);
        return;
      }
      if (is_goal_missing(err)) {
        set_goal(null);
        set_events([]);
        set_resume_prompt_goal(null);
        set_error(null);
        return;
      }
      set_error(err instanceof Error ? err.message : "Goal 状态读取失败");
    } finally {
      set_is_loading(false);
    }
  }, [maybe_prompt_resume_goal, refresh_goal_events, session_key]);

  useEffect(() => {
    void refresh_goal();
  }, [refresh_goal, activity_key]);

  const begin_editing_goal = useCallback((current: Goal) => {
    set_objective(current.objective);
    set_budget(current.token_budget ? String(current.token_budget) : "");
    set_is_editing(true);
  }, []);

  useEffect(() => {
    if (edit_request_key === null || edit_request_key === undefined || disabled || !goal) {
      return;
    }
    if (edit_request_key_ref.current === edit_request_key) {
      return;
    }
    edit_request_key_ref.current = edit_request_key;
    begin_editing_goal(goal);
  }, [begin_editing_goal, disabled, edit_request_key, goal]);

  const submit_goal = async (event: FormEvent) => {
    event.preventDefault();
    if (!session_key || !objective.trim()) return;
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
      await refresh_goal_events(updated.id);
      set_objective("");
      set_budget("");
      set_is_editing(false);
      set_error(null);
    } catch (err) {
      set_error(err instanceof Error ? err.message : "Goal 保存失败");
    } finally {
      set_is_loading(false);
    }
  };

  const mutate_goal = async (action: (goal_id: string) => Promise<Goal>) => {
    if (!goal || disabled) return;
    set_is_loading(true);
    try {
      const updated = await action(goal.id);
      if (updated.status === "cleared") {
        set_goal(null);
        set_events([]);
      } else {
        set_goal(updated);
        await refresh_goal_events(updated.id);
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
    void mutate_goal(clear_goal_api);
  };

  const start_editing_goal = () => {
    if (!goal) return;
    begin_editing_goal(goal);
  };

  const cancel_editing_goal = () => {
    set_objective("");
    set_budget("");
    set_is_editing(false);
  };

  const recent_events = events.slice(0, 3);

  if (!is_available || !session_key) {
    return null;
  }

  if (!goal || is_editing) {
    return (
      <form
        className={cn(
          "mx-auto mb-2 flex w-full max-w-[980px] flex-wrap items-end gap-2 rounded-lg border border-border/70 bg-background/95 px-3 py-2 shadow-sm backdrop-blur",
          compact && "mx-2 max-w-none",
        )}
        onSubmit={submit_goal}
      >
        <div className="mb-1.5 grid h-8 w-8 shrink-0 place-items-center rounded-md border border-border/70 bg-muted/50 text-muted-foreground">
          <Target className="h-4 w-4" />
        </div>
        <div className="min-w-[160px] flex-1">
          <input
            className="h-8 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            disabled={disabled || is_loading}
            placeholder="输入 Goal"
            value={objective}
            onChange={(event) => set_objective(event.target.value)}
          />
        </div>
        <input
          className="h-8 w-24 rounded-md border border-border/70 bg-background px-2 text-xs outline-none placeholder:text-muted-foreground"
          disabled={disabled || is_loading}
          inputMode="numeric"
          placeholder="Token"
          value={budget}
          onChange={(event) => set_budget(event.target.value)}
        />
        <button
          className="grid h-8 w-8 place-items-center rounded-md bg-primary text-primary-foreground disabled:cursor-not-allowed disabled:opacity-50"
          disabled={disabled || is_loading || !objective.trim()}
          title={is_editing ? "保存 Goal" : "创建 Goal"}
          type="submit"
        >
          {is_editing ? (
            <Save className="h-4 w-4" />
          ) : (
            <Target className="h-4 w-4" />
          )}
        </button>
        {is_editing ? (
          <button
            className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground"
            title="取消"
            type="button"
            onClick={cancel_editing_goal}
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </form>
    );
  }

  const tone = goal_status_tone(goal.status);
  const runtime_label = goal_runtime_label(goal, is_generating);
  const latest_event = recent_events[0] ?? null;
  const elapsed_label = goal_elapsed_label(goal.time_used_seconds);

  return (
    <>
      <div
        className={cn(
          "mx-auto mb-2 w-full max-w-[980px] overflow-hidden rounded-lg border border-border/70 bg-background/95 shadow-sm backdrop-blur",
          compact && "mx-2 max-w-none",
        )}
      >
        <div className={cn("h-1 w-full", tone.rail)} />
        <div className="flex flex-col gap-2 px-3 py-2 sm:flex-row sm:items-center">
          <div className="flex min-w-0 flex-1 items-start gap-3">
            <div className={cn("grid h-9 w-9 shrink-0 place-items-center rounded-md border", tone.icon)}>
              <Target className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                <span className="text-[11px] font-medium text-muted-foreground">
                  Goal 模式
                </span>
                <span className={cn("shrink-0 rounded border px-1.5 py-0.5 text-[11px] font-medium", tone.badge)}>
                  {GOAL_STATUS_LABEL[goal.status] ?? goal.status}
                </span>
                <span className={cn("text-[11px] font-medium", tone.text)}>
                  {runtime_label}
                </span>
                {latest_event ? (
                  <span className="min-w-0 truncate text-[11px] text-muted-foreground">
                    {goal_event_label(latest_event)}
                  </span>
                ) : null}
              </div>
              <div className="mt-1 min-w-0 break-words text-sm font-medium leading-5 text-foreground">
                {goal.objective}
              </div>
              <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                <span className="inline-flex h-6 items-center gap-1 rounded-md border border-border/60 bg-muted/30 px-2">
                  <GaugeCircle className="h-3.5 w-3.5" />
                  {format_tokens(usage_total)}
                  {budget_value ? ` / ${format_tokens(budget_value)}` : ""}
                </span>
                <span className="inline-flex h-6 items-center gap-1 rounded-md border border-border/60 bg-muted/30 px-2">
                  <Clock3 className="h-3.5 w-3.5" />
                  {elapsed_label}
                </span>
                <span className="inline-flex h-6 items-center gap-1 rounded-md border border-border/60 bg-muted/30 px-2">
                  <Repeat2 className="h-3.5 w-3.5" />
                  续跑 {goal.continuation_count}
                </span>
                {goal.last_error ? (
                  <span className="inline-flex h-6 max-w-full items-center truncate rounded-md border border-destructive/20 bg-destructive/10 px-2 text-destructive">
                    {goal.last_error}
                  </span>
                ) : null}
              </div>
              {usage_percent !== null ? (
                <div className="mt-2 h-1.5 overflow-hidden rounded bg-muted">
                  <div
                    className={cn("h-full", tone.meter)}
                    style={{ width: `${usage_percent}%` }}
                  />
                </div>
              ) : null}
              {recent_events.length > 1 ? (
                <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-1 text-[11px] text-muted-foreground">
                  {recent_events.slice(1).map((event) => (
                    <span
                      key={event.id}
                      className="rounded border border-border/60 px-1.5 py-0.5"
                    >
                      {goal_event_label(event)}
                    </span>
                  ))}
                </div>
              ) : null}
              {error ? (
                <div className="mt-1 text-[11px] text-destructive">{error}</div>
              ) : null}
            </div>
          </div>
          <div className="flex shrink-0 items-center justify-end gap-1 sm:ml-auto">
            <button
              className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground"
              title="刷新"
              type="button"
              onClick={() => void refresh_goal()}
            >
              <RefreshCw className={cn("h-4 w-4", is_loading && "animate-spin")} />
            </button>
            <button
              className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground disabled:opacity-50"
              disabled={disabled || is_loading}
              title={goal.status === "budget_limited" ? "调整预算" : "编辑"}
              type="button"
              onClick={start_editing_goal}
            >
              <Pencil className="h-4 w-4" />
            </button>
            {goal.status === "active" ? (
              <button
                className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground disabled:opacity-50"
                disabled={disabled || is_loading}
                title="暂停"
                type="button"
                onClick={() => void mutate_goal(pause_goal_api)}
              >
                <Pause className="h-4 w-4" />
              </button>
            ) : null}
            {can_resume_goal(goal.status) ? (
              <button
                className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground disabled:opacity-50"
                disabled={disabled || is_loading}
                title="继续"
                type="button"
                onClick={() => void mutate_goal(resume_goal_api)}
              >
                <Play className="h-4 w-4" />
              </button>
            ) : null}
            {goal.status === "active" ? (
              <button
                className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground disabled:opacity-50"
                disabled={disabled || is_loading}
                title="完成"
                type="button"
                onClick={() => void mutate_goal(complete_goal_api)}
              >
                <CheckCircle2 className="h-4 w-4" />
              </button>
            ) : null}
            <button
              className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground disabled:opacity-50"
              disabled={disabled || is_loading}
              title="清除"
              type="button"
              onClick={() => set_is_clear_confirm_open(true)}
            >
              <CircleSlash className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
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
