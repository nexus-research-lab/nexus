"use client";

import { ReactNode, useEffect, useState } from "react";
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
  Target,
} from "lucide-react";

import { cn, format_tokens } from "@/lib/utils";
import type { Goal, GoalEvent } from "@/types/conversation/goal";
import {
  GOAL_STATUS_LABEL,
  goal_budget_percent,
  goal_elapsed_label,
  goal_event_label,
  goal_runtime_label,
  goal_status_tone,
  goal_usage_total,
} from "./goal-panel-model";

const GOAL_ELAPSED_TICK_MS = 1000;

interface GoalStatusStripProps {
  can_resume: boolean;
  compact: boolean;
  disabled: boolean;
  error: string | null;
  goal: Goal;
  is_generating: boolean;
  is_loading: boolean;
  recent_events: GoalEvent[];
  scope_label: string;
  on_clear_request: () => void;
  on_complete: () => void;
  on_edit: () => void;
  on_pause: () => void;
  on_refresh: () => void;
  on_resume: () => void;
}

interface GoalActionButtonProps {
  children: ReactNode;
  disabled?: boolean;
  title: string;
  on_click: () => void;
}

function GoalActionButton({
  children,
  disabled = false,
  title,
  on_click,
}: GoalActionButtonProps) {
  return (
    <button
      aria-label={title}
      className="grid h-8 w-8 place-items-center rounded-md border border-border/70 text-muted-foreground transition-colors hover:bg-muted/70 disabled:cursor-not-allowed disabled:opacity-50"
      disabled={disabled}
      title={title}
      type="button"
      onClick={on_click}
    >
      {children}
    </button>
  );
}

export function GoalStatusStrip({
  can_resume,
  compact,
  disabled,
  error,
  goal,
  is_generating,
  is_loading,
  recent_events,
  scope_label,
  on_clear_request,
  on_complete,
  on_edit,
  on_pause,
  on_refresh,
  on_resume,
}: GoalStatusStripProps) {
  const usage_total = goal_usage_total(goal);
  const budget_value = goal.token_budget ?? null;
  const usage_percent = goal_budget_percent(goal);
  const tone = goal_status_tone(goal.status);
  const runtime_label = goal_runtime_label(goal, is_generating);
  const latest_event = recent_events[0] ?? null;
  const [observed_at_ms, set_observed_at_ms] = useState(() => Date.now());
  const [active_turn_started_at_ms, set_active_turn_started_at_ms] = useState<
    number | null
  >(null);
  const [now_ms, set_now_ms] = useState(() => Date.now());

  useEffect(() => {
    const now = Date.now();
    set_observed_at_ms(now);
    set_now_ms(now);
  }, [goal.id, goal.status, goal.time_used_seconds, goal.updated_at]);

  useEffect(() => {
    if (goal.status !== "active" || !is_generating) {
      set_active_turn_started_at_ms(null);
      return;
    }
    set_active_turn_started_at_ms((current) => current ?? Date.now());
  }, [goal.id, goal.status, is_generating]);

  useEffect(() => {
    if (active_turn_started_at_ms === null) return;
    const timer = window.setInterval(() => {
      set_now_ms(Date.now());
    }, GOAL_ELAPSED_TICK_MS);
    return () => window.clearInterval(timer);
  }, [active_turn_started_at_ms]);

  const active_elapsed_seconds =
    active_turn_started_at_ms !== null
      ? Math.max(
          0,
          Math.floor(
            (now_ms - Math.max(observed_at_ms, active_turn_started_at_ms)) /
              1000,
          ),
        )
      : 0;
  const elapsed_label = goal_elapsed_label(
    (goal.time_used_seconds ?? 0) + active_elapsed_seconds,
  );

  return (
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
                {scope_label}
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
            {error ? <div className="mt-1 text-[11px] text-destructive">{error}</div> : null}
          </div>
        </div>
        <div className="flex shrink-0 items-center justify-end gap-1 sm:ml-auto">
          <GoalActionButton title="刷新" on_click={on_refresh}>
            <RefreshCw className={cn("h-4 w-4", is_loading && "animate-spin")} />
          </GoalActionButton>
          <GoalActionButton
            disabled={disabled || is_loading}
            title={goal.status === "budget_limited" ? "调整预算" : "编辑"}
            on_click={on_edit}
          >
            <Pencil className="h-4 w-4" />
          </GoalActionButton>
          {goal.status === "active" ? (
            <GoalActionButton
              disabled={disabled || is_loading}
              title="暂停"
              on_click={on_pause}
            >
              <Pause className="h-4 w-4" />
            </GoalActionButton>
          ) : null}
          {can_resume ? (
            <GoalActionButton
              disabled={disabled || is_loading}
              title="继续"
              on_click={on_resume}
            >
              <Play className="h-4 w-4" />
            </GoalActionButton>
          ) : null}
          {goal.status === "active" ? (
            <GoalActionButton
              disabled={disabled || is_loading}
              title="完成"
              on_click={on_complete}
            >
              <CheckCircle2 className="h-4 w-4" />
            </GoalActionButton>
          ) : null}
          <GoalActionButton
            disabled={disabled || is_loading}
            title="清除"
            on_click={on_clear_request}
          >
            <CircleSlash className="h-4 w-4" />
          </GoalActionButton>
        </div>
      </div>
    </div>
  );
}
