"use client";

import type { ReactNode } from "react";
import {
  CircleSlash,
  GaugeCircle,
  Pause,
  Pencil,
  Play,
  RefreshCw,
  Target,
} from "lucide-react";

import { cn, format_tokens } from "@/lib/utils";
import { UiIconButton } from "@/shared/ui/button";
import type { Goal, GoalStatus } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import {
  GOAL_STATUS_LABEL,
  goal_budget_percent,
  goal_status_tone,
  goal_usage_total,
} from "./goal-panel-model";
import {
  GOAL_PANEL_BADGE_CLASS_NAME,
  GOAL_PANEL_COMPACT_CLASS_NAME,
  GOAL_PANEL_LEADING_ICON_CLASS_NAME,
  GOAL_PANEL_ROW_CLASS_NAME,
  GOAL_PANEL_STRIP_CLASS_NAME,
  GOAL_PANEL_SURFACE_CLASS_NAME,
} from "./goal-panel-styles";

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
  status_extra?: ReactNode;
  on_clear_request: () => void;
  on_edit: () => void;
  on_pause: () => void;
  on_refresh: () => void;
  on_resume: () => void;
}

function visible_goal_status({
  continuation_hold,
  goal,
  is_generating,
}: {
  continuation_hold: GoalContinuationHold | null;
  goal: Goal;
  is_generating: boolean;
}): { label: string; status: GoalStatus } {
  if (goal.status === "active" && !is_generating && goal.last_error) {
    return { label: "需处理", status: "blocked" };
  }
  if (
    goal.status === "active" &&
    !is_generating &&
    (continuation_hold !== null || (goal.empty_progress_count ?? 0) > 0)
  ) {
    return { label: "待继续", status: "paused" };
  }
  return {
    label: GOAL_STATUS_LABEL[goal.status] ?? goal.status,
    status: goal.status,
  };
}

function goal_budget_label(goal: Goal): string | null {
  const usage_total = goal_usage_total(goal);
  const budget = goal.token_budget ?? null;
  if (budget && budget > 0) {
    return `${format_tokens(usage_total)} / ${format_tokens(budget)}`;
  }
  if (usage_total > 0) {
    return format_tokens(usage_total);
  }
  return null;
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
  status_extra = null,
  on_clear_request,
  on_edit,
  on_pause,
  on_refresh,
  on_resume,
}: GoalStatusStripProps) {
  const active_continuation_hold =
    goal.status === "active" ? continuation_hold : null;
  const visible_status = visible_goal_status({
    continuation_hold: active_continuation_hold,
    goal,
    is_generating,
  });
  const tone = goal_status_tone(visible_status.status);
  const budget_label = goal_budget_label(goal);
  const usage_percent = goal_budget_percent(goal);
  const attention_message = error ?? goal.last_error ?? null;
  const status_title = active_continuation_hold?.detail ?? visible_status.label;

  return (
    <div
      className={cn(
        GOAL_PANEL_STRIP_CLASS_NAME,
        compact && GOAL_PANEL_COMPACT_CLASS_NAME,
      )}
    >
      <div className={GOAL_PANEL_SURFACE_CLASS_NAME}>
        <div className={GOAL_PANEL_ROW_CLASS_NAME}>
          <span
            className={cn(
              GOAL_PANEL_LEADING_ICON_CLASS_NAME,
              tone.icon,
            )}
          >
            <Target className="h-3.5 w-3.5" />
          </span>

          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-1.5 text-[10px] font-medium text-(--text-soft)">
              <span className="truncate">{scope_label}</span>
              <span
                className={cn(GOAL_PANEL_BADGE_CLASS_NAME, tone.badge)}
                title={status_title}
              >
                {visible_status.label}
              </span>
              {is_generating && goal.status === "active" ? (
                <span className={cn("font-semibold", tone.text)}>执行中</span>
              ) : null}
              {status_extra}
            </div>
            <div className="mt-0.5 line-clamp-1 text-[12px] font-medium leading-5 text-(--text-strong)">
              {goal.objective}
            </div>
          </div>

          {budget_label ? (
            <span
              className="hidden h-6 max-w-[128px] shrink-0 items-center gap-1 truncate rounded-[8px] px-1.5 text-[11px] font-medium text-(--text-muted) sm:inline-flex"
              title="Token 使用"
            >
              <GaugeCircle className="h-3.5 w-3.5 shrink-0" />
              <span className="truncate">{budget_label}</span>
            </span>
          ) : null}

          <div className="ml-auto flex shrink-0 items-center gap-1">
            <UiIconButton
              aria-label="刷新"
              size="sm"
              title="刷新"
              type="button"
              variant="ghost"
              onClick={on_refresh}
            >
              <RefreshCw className={cn("h-4 w-4", is_loading && "animate-spin")} />
            </UiIconButton>
            <UiIconButton
              aria-label="编辑"
              disabled={disabled || is_loading}
              size="sm"
              title="编辑"
              type="button"
              variant="ghost"
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
                variant="ghost"
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
                variant="ghost"
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
        </div>

        {attention_message ? (
          <div className="ml-7 line-clamp-1 pb-1 text-[11px] leading-4 text-(--destructive)">
            {attention_message}
          </div>
        ) : null}
        {usage_percent !== null ? (
          <div className="ml-7 h-1 overflow-hidden rounded-full bg-(--surface-interactive-hover-background)">
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
