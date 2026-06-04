"use client";

import { ReactNode, useEffect, useState } from "react";
import {
  CircleSlash,
  GaugeCircle,
  Pause,
  Pencil,
  Play,
  RefreshCw,
  Repeat2,
  Target,
} from "lucide-react";

import { cn, format_tokens } from "@/lib/utils";
import { UiIconButton } from "@/shared/ui/button";
import type { Goal } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import {
  GOAL_STATUS_LABEL,
  goal_budget_percent,
  goal_elapsed_label,
  goal_runtime_label,
  goal_status_tone,
  goal_usage_total,
} from "./goal-panel-model";
import {
  GOAL_PANEL_BADGE_CLASS_NAME,
  GOAL_PANEL_COMPACT_CLASS_NAME,
  GOAL_PANEL_HEADER_CLASS_NAME,
  GOAL_PANEL_HEADER_LABEL_CLASS_NAME,
  GOAL_PANEL_LEADING_ICON_CLASS_NAME,
  GOAL_PANEL_ROW_CLASS_NAME,
  GOAL_PANEL_STRIP_CLASS_NAME,
  GOAL_PANEL_SURFACE_CLASS_NAME,
} from "./goal-panel-styles";

const GOAL_ELAPSED_TICK_MS = 1000;

interface GoalStatusStripProps {
  can_resume: boolean;
  compact: boolean;
  continuation_hold?: GoalContinuationHold | null;
  disabled: boolean;
  error: string | null;
  goal: Goal;
  is_generating: boolean;
  is_loading: boolean;
  scope_label: string;
  on_clear_request: () => void;
  on_edit: () => void;
  on_pause: () => void;
  on_refresh: () => void;
  on_resume: () => void;
}

interface GoalMetricPillProps {
  children: ReactNode;
  className?: string;
  icon: ReactNode;
  title?: string;
}

function GoalMetricPill({
  children,
  className,
  icon,
  title,
}: GoalMetricPillProps) {
  return (
    <span
      className={cn(
        "inline-flex h-6 max-w-full items-center gap-1.5 rounded-[8px] px-1.5 text-[11px] font-medium text-(--text-muted)",
        className,
      )}
      title={title}
    >
      {icon}
      <span className="truncate">{children}</span>
    </span>
  );
}

export function GoalStatusStrip({
  can_resume,
  compact,
  continuation_hold = null,
  disabled,
  error,
  goal,
  is_generating,
  is_loading,
  scope_label,
  on_clear_request,
  on_edit,
  on_pause,
  on_refresh,
  on_resume,
}: GoalStatusStripProps) {
  const usage_total = goal_usage_total(goal);
  const budget_value = goal.token_budget ?? null;
  const remaining_tokens =
    budget_value !== null ? Math.max(0, budget_value - usage_total) : null;
  const usage_percent = goal_budget_percent(goal);
  const runtime_label = goal_runtime_label(goal, is_generating);
  const active_continuation_hold =
    goal.status === "active" ? continuation_hold : null;
  const continuation_suppressed =
    goal.status === "active" &&
    (active_continuation_hold !== null || (goal.empty_progress_count ?? 0) > 0);
  const active_needs_attention =
    goal.status === "active" && !is_generating && Boolean(goal.last_error);
  const active_waiting_to_resume =
    goal.status === "active" && !is_generating && continuation_suppressed;
  const tone = active_needs_attention
    ? goal_status_tone("blocked")
    : active_waiting_to_resume
      ? goal_status_tone("paused")
      : goal_status_tone(goal.status);
  const status_label = active_needs_attention
    ? "需处理"
    : active_waiting_to_resume
      ? "待继续"
      : GOAL_STATUS_LABEL[goal.status] ?? goal.status;
  const continuation_metric_title = active_continuation_hold
    ? active_continuation_hold.detail
    : continuation_suppressed
      ? "隐藏续跑无可计入进展，等待新的用户或外部活动"
      : "Goal 自动续跑次数";
  const continuation_metric_label = active_continuation_hold
    ? active_continuation_hold.label
    : continuation_suppressed
      ? "续跑暂停"
      : `续跑 ${goal.continuation_count}`;
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
        GOAL_PANEL_STRIP_CLASS_NAME,
        compact && GOAL_PANEL_COMPACT_CLASS_NAME,
      )}
    >
      <div className={GOAL_PANEL_SURFACE_CLASS_NAME}>
        <div className={GOAL_PANEL_HEADER_CLASS_NAME}>
          <span className={GOAL_PANEL_HEADER_LABEL_CLASS_NAME}>
            <Target className="h-3 w-3 shrink-0 text-(--primary)" />
            {scope_label}
            <span
              className={cn(
                GOAL_PANEL_BADGE_CLASS_NAME,
                tone.badge,
              )}
            >
              {status_label}
            </span>
            <span className={cn("font-semibold", tone.text)}>
              {runtime_label}
            </span>
          </span>
          <span className="shrink-0 tabular-nums">
            {elapsed_label}
          </span>
        </div>

        <div className={GOAL_PANEL_ROW_CLASS_NAME}>
          <span
            className={cn(
              GOAL_PANEL_LEADING_ICON_CLASS_NAME,
              tone.icon,
            )}
          >
            <Target className="h-3.5 w-3.5" />
          </span>
          <span className="min-w-[140px] flex-1 truncate text-[12px] font-medium leading-5 text-(--text-strong)">
            {goal.objective}
          </span>
          <GoalMetricPill
            icon={<GaugeCircle className="h-3.5 w-3.5 shrink-0" />}
            title="已用 token"
          >
            已用 {format_tokens(usage_total)}
            {budget_value ? ` / ${format_tokens(budget_value)}` : ""}
          </GoalMetricPill>
          {remaining_tokens !== null ? (
            <GoalMetricPill
              icon={<GaugeCircle className="h-3.5 w-3.5 shrink-0" />}
              title="剩余 token 预算"
            >
              剩余 {format_tokens(remaining_tokens)}
            </GoalMetricPill>
          ) : null}
          <GoalMetricPill
            className={
              continuation_suppressed
                ? "text-amber-700 dark:text-amber-300"
                : undefined
            }
            icon={
              continuation_suppressed ? (
                <Pause className="h-3.5 w-3.5 shrink-0" />
              ) : (
                <Repeat2 className="h-3.5 w-3.5 shrink-0" />
              )
            }
            title={continuation_metric_title}
          >
            {continuation_metric_label}
          </GoalMetricPill>
          {goal.last_error ? (
            <span
              className="inline-flex h-6 max-w-[220px] items-center truncate rounded-[8px] px-1.5 text-[11px] font-medium text-(--destructive)"
              title={goal.last_error}
            >
              {goal.last_error}
            </span>
          ) : null}
          {error ? (
            <span
              className="inline-flex h-6 max-w-[220px] items-center truncate rounded-[8px] px-1.5 text-[11px] font-medium text-(--destructive)"
              title={error}
            >
              {error}
            </span>
          ) : null}
          <UiIconButton
            aria-label="刷新"
            size="sm"
            title="刷新"
            type="button"
            variant="surface"
            onClick={on_refresh}
          >
            <RefreshCw className={cn("h-4 w-4", is_loading && "animate-spin")} />
          </UiIconButton>
          <UiIconButton
            aria-label={goal.status === "budget_limited" ? "调整预算" : "编辑"}
            disabled={disabled || is_loading}
            size="sm"
            title={goal.status === "budget_limited" ? "调整预算" : "编辑"}
            type="button"
            variant="surface"
            onClick={on_edit}
          >
            <Pencil className="h-4 w-4" />
          </UiIconButton>
          {goal.status === "active" ? (
            <UiIconButton
              aria-label="暂停"
              disabled={disabled || is_loading}
              size="sm"
              title="暂停"
              type="button"
              variant="surface"
              onClick={on_pause}
            >
              <Pause className="h-4 w-4" />
            </UiIconButton>
          ) : null}
          {can_resume ? (
            <UiIconButton
              aria-label="继续"
              disabled={disabled || is_loading}
              size="sm"
              title="继续"
              tone="primary"
              type="button"
              variant="surface"
              onClick={on_resume}
            >
              <Play className="h-4 w-4" />
            </UiIconButton>
          ) : null}
          <UiIconButton
            aria-label="清除"
            disabled={disabled || is_loading}
            size="sm"
            title="清除"
            tone="danger"
            type="button"
            variant="ghost"
            onClick={on_clear_request}
          >
            <CircleSlash className="h-4 w-4" />
          </UiIconButton>
        </div>
        {usage_percent !== null ? (
          <div className="mt-1 h-1 overflow-hidden rounded-full bg-(--surface-interactive-hover-background)">
            <div
              className={cn("h-full", tone.meter)}
              style={{ width: `${usage_percent}%` }}
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}
