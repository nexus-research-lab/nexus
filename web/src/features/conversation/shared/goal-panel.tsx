"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  CheckCircle2,
  CircleSlash,
  Pause,
  Play,
  RefreshCw,
  Target,
  X,
} from "lucide-react";

import {
  clear_goal_api,
  complete_goal_api,
  create_goal_api,
  get_current_goal_api,
  pause_goal_api,
  resume_goal_api,
} from "@/lib/api/goal-api";
import { ApiRequestError } from "@/lib/api/http";
import { cn, format_tokens } from "@/lib/utils";
import type { Goal, GoalStatus } from "@/types/conversation/goal";

interface GoalPanelProps {
  session_key: string | null;
  compact?: boolean;
  disabled?: boolean;
  activity_key?: string | number | null;
}

const STATUS_LABEL: Record<GoalStatus, string> = {
  active: "运行中",
  paused: "已暂停",
  complete: "已完成",
  blocked: "已阻塞",
  budget_limited: "预算耗尽",
  usage_limited: "续跑受限",
  cleared: "已清除",
};

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

function goal_usage_total(goal: Goal | null): number {
  return goal?.usage?.total_tokens ?? 0;
}

export function GoalPanel({
  session_key,
  compact = false,
  disabled = false,
  activity_key = null,
}: GoalPanelProps) {
  const [goal, set_goal] = useState<Goal | null>(null);
  const [is_available, set_is_available] = useState(true);
  const [is_loading, set_is_loading] = useState(false);
  const [is_editing, set_is_editing] = useState(false);
  const [objective, set_objective] = useState("");
  const [budget, set_budget] = useState("");
  const [error, set_error] = useState<string | null>(null);

  const usage_total = goal_usage_total(goal);
  const budget_value = goal?.token_budget ?? null;
  const usage_percent = useMemo(() => {
    if (!budget_value || budget_value <= 0) return null;
    return Math.min(100, Math.round((usage_total / budget_value) * 100));
  }, [budget_value, usage_total]);

  const refresh_goal = useCallback(async () => {
    if (!session_key) {
      set_goal(null);
      return;
    }
    set_is_loading(true);
    try {
      const current = await get_current_goal_api(session_key);
      set_goal(current);
      set_is_available(true);
      set_error(null);
    } catch (err) {
      if (is_goal_unavailable(err)) {
        set_is_available(false);
        set_goal(null);
        return;
      }
      if (is_goal_missing(err)) {
        set_goal(null);
        set_error(null);
        return;
      }
      set_error(err instanceof Error ? err.message : "Goal 状态读取失败");
    } finally {
      set_is_loading(false);
    }
  }, [session_key]);

  useEffect(() => {
    void refresh_goal();
  }, [refresh_goal, activity_key]);

  const create_goal = async (event: FormEvent) => {
    event.preventDefault();
    if (!session_key || !objective.trim()) return;
    set_is_loading(true);
    try {
      const created = await create_goal_api({
        session_key,
        objective: objective.trim(),
        token_budget: normalize_budget(budget),
      });
      set_goal(created);
      set_objective("");
      set_budget("");
      set_is_editing(false);
      set_error(null);
    } catch (err) {
      set_error(err instanceof Error ? err.message : "Goal 创建失败");
    } finally {
      set_is_loading(false);
    }
  };

  const mutate_goal = async (action: (goal_id: string) => Promise<Goal>) => {
    if (!goal || disabled) return;
    set_is_loading(true);
    try {
      const updated = await action(goal.id);
      set_goal(updated.status === "cleared" ? null : updated);
      set_error(null);
    } catch (err) {
      set_error(err instanceof Error ? err.message : "Goal 操作失败");
    } finally {
      set_is_loading(false);
    }
  };

  if (!is_available || !session_key) {
    return null;
  }

  if (!goal || is_editing) {
    return (
      <form
        className={cn(
          "mx-auto mb-2 flex w-full max-w-[980px] flex-wrap items-end gap-2 rounded-lg border border-border/70 bg-background/88 px-3 py-2 shadow-sm backdrop-blur",
          compact && "mx-2 max-w-none",
        )}
        onSubmit={create_goal}
      >
        <Target className="mb-2 h-4 w-4 shrink-0 text-muted-foreground" />
        <div className="min-w-[160px] flex-1">
          <input
            className="h-8 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            disabled={disabled || is_loading}
            placeholder="Goal"
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
          title="创建 Goal"
          type="submit"
        >
          <Target className="h-4 w-4" />
        </button>
        {is_editing ? (
          <button
            className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground"
            title="取消"
            type="button"
            onClick={() => set_is_editing(false)}
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </form>
    );
  }

  return (
    <div
      className={cn(
        "mx-auto mb-2 flex w-full max-w-[980px] flex-wrap items-center gap-3 rounded-lg border border-border/70 bg-background/88 px-3 py-2 shadow-sm backdrop-blur",
        compact && "mx-2 max-w-none",
      )}
    >
      <Target className="h-4 w-4 shrink-0 text-primary" />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-sm font-medium text-foreground">
            {goal.objective}
          </span>
          <span className="shrink-0 rounded border border-border/70 px-1.5 py-0.5 text-[11px] text-muted-foreground">
            {STATUS_LABEL[goal.status] ?? goal.status}
          </span>
        </div>
        <div className="mt-1 flex items-center gap-2 text-[11px] text-muted-foreground">
          <span>{format_tokens(usage_total)} tokens</span>
          {budget_value ? <span>/ {format_tokens(budget_value)}</span> : null}
          <span>{goal.continuation_count} turns</span>
          {goal.last_error ? (
            <span className="truncate text-destructive">{goal.last_error}</span>
          ) : null}
        </div>
        {usage_percent !== null ? (
          <div className="mt-1 h-1 overflow-hidden rounded bg-muted">
            <div
              className="h-full bg-primary"
              style={{ width: `${usage_percent}%` }}
            />
          </div>
        ) : null}
        {error ? (
          <div className="mt-1 text-[11px] text-destructive">{error}</div>
        ) : null}
      </div>
      <div className="ml-auto flex shrink-0 items-center gap-1">
        <button
          className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground"
          title="刷新"
          type="button"
          onClick={() => void refresh_goal()}
        >
          <RefreshCw className={cn("h-4 w-4", is_loading && "animate-spin")} />
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
        {goal.status === "paused" || goal.status === "blocked" ? (
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
        {goal.status === "budget_limited" || goal.status === "usage_limited" ? (
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
          onClick={() => void mutate_goal(clear_goal_api)}
        >
          <CircleSlash className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
