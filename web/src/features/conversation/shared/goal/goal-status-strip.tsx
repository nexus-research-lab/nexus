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

import { cn, formatTokens } from "@/lib/utils";
import { UiIconButton } from "@/shared/ui/button";
import type { Goal, GoalStatus } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import {
  GOAL_PANEL_BADGE_CLASS_NAME,
  GOAL_PANEL_COMPACT_CLASS_NAME,
  GOAL_PANEL_LEADING_ICON_CLASS_NAME,
  GOAL_PANEL_ROW_CLASS_NAME,
  GOAL_PANEL_STRIP_CLASS_NAME,
  GOAL_PANEL_SURFACE_CLASS_NAME,
  GOAL_STATUS_LABEL,
  goalBudgetPercent,
  goalStatusTone,
  goalUsageTotal,
} from "./goal-model";

interface GoalStatusStripProps {
  canResume: boolean;
  compact: boolean;
  continuationHold?: GoalContinuationHold | null;
  disabled: boolean;
  error: string | null;
  goal: Goal;
  isGenerating: boolean;
  isLoading: boolean;
  scopeLabel: string;
  statusExtra?: ReactNode;
  onClearRequest: () => void;
  onEdit: () => void;
  onPause: () => void;
  onRefresh: () => void;
  onResume: () => void;
}

function visibleGoalStatus({
  continuationHold,
  goal,
  isGenerating,
}: {
  continuationHold: GoalContinuationHold | null;
  goal: Goal;
  isGenerating: boolean;
}): { label: string; status: GoalStatus } {
  if (goal.status === "active" && !isGenerating && goal.last_error) {
    return { label: "需处理", status: "blocked" };
  }
  if (
    goal.status === "active" &&
    !isGenerating &&
    (continuationHold !== null || (goal.empty_progress_count ?? 0) > 0)
  ) {
    return { label: "待继续", status: "paused" };
  }
  return {
    label: GOAL_STATUS_LABEL[goal.status],
    status: goal.status,
  };
}

function goalBudgetLabel(goal: Goal): string | null {
  const usageTotal = goalUsageTotal(goal);
  const budget = goal.token_budget ?? null;
  if (budget && budget > 0) {
    return `${formatTokens(usageTotal)} / ${formatTokens(budget)}`;
  }
  if (usageTotal > 0) {
    return formatTokens(usageTotal);
  }
  return null;
}

export function GoalStatusStrip({
  canResume,
  compact,
  continuationHold = null,
  disabled,
  error,
  goal,
  isGenerating,
  isLoading,
  scopeLabel,
  statusExtra = null,
  onClearRequest,
  onEdit,
  onPause,
  onRefresh,
  onResume,
}: GoalStatusStripProps) {
  const activeContinuationHold =
    goal.status === "active" ? continuationHold : null;
  const visibleStatus = visibleGoalStatus({
    continuationHold: activeContinuationHold,
    goal,
    isGenerating,
  });
  const tone = goalStatusTone(visibleStatus.status);
  const budgetLabel = goalBudgetLabel(goal);
  const usagePercent = goalBudgetPercent(goal);
  const attentionMessage = error ?? goal.last_error ?? null;
  const statusTitle = activeContinuationHold?.detail ?? visibleStatus.label;

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
              <span className="truncate">{scopeLabel}</span>
              <span
                className={cn(GOAL_PANEL_BADGE_CLASS_NAME, tone.badge)}
                title={statusTitle}
              >
                {visibleStatus.label}
              </span>
              {isGenerating && goal.status === "active" ? (
                <span className={cn("font-semibold", tone.text)}>执行中</span>
              ) : null}
              {statusExtra}
            </div>
            <div className="mt-0.5 line-clamp-1 text-[12px] font-medium leading-5 text-(--text-strong)">
              {goal.objective}
            </div>
          </div>

          {budgetLabel ? (
            <span
              className="hidden h-6 max-w-[128px] shrink-0 items-center gap-1 truncate rounded-[8px] px-1.5 text-[11px] font-medium text-(--text-muted) sm:inline-flex"
              title="Token 使用"
            >
              <GaugeCircle className="h-3.5 w-3.5 shrink-0" />
              <span className="truncate">{budgetLabel}</span>
            </span>
          ) : null}

          <div className="ml-auto flex shrink-0 items-center gap-1">
            <UiIconButton
              aria-label="刷新"
              size="sm"
              title="刷新"
              type="button"
              variant="ghost"
              onClick={onRefresh}
            >
              <RefreshCw className={cn("h-4 w-4", isLoading && "animate-spin")} />
            </UiIconButton>
            <UiIconButton
              aria-label="编辑"
              disabled={disabled || isLoading}
              size="sm"
              title="编辑"
              type="button"
              variant="ghost"
              onClick={onEdit}
            >
              <Pencil className="h-4 w-4" />
            </UiIconButton>
            {goal.status === "active" ? (
              <UiIconButton
                aria-label="暂停"
                disabled={disabled || isLoading}
                size="sm"
                title="暂停"
                type="button"
                variant="ghost"
                onClick={onPause}
              >
                <Pause className="h-4 w-4" />
              </UiIconButton>
            ) : null}
            {canResume ? (
              <UiIconButton
                aria-label="继续"
                disabled={disabled || isLoading}
                size="sm"
                title="继续"
                tone="primary"
                type="button"
                variant="ghost"
                onClick={onResume}
              >
                <Play className="h-4 w-4" />
              </UiIconButton>
            ) : null}
            <UiIconButton
              aria-label="清除"
              disabled={disabled || isLoading}
              size="sm"
              title="清除"
              tone="danger"
              type="button"
              variant="ghost"
              onClick={onClearRequest}
            >
              <CircleSlash className="h-4 w-4" />
            </UiIconButton>
          </div>
        </div>

        {attentionMessage ? (
          <div className="ml-7 line-clamp-1 pb-1 text-[11px] leading-4 text-(--destructive)">
            {attentionMessage}
          </div>
        ) : null}
        {usagePercent !== null ? (
          <div className="ml-7 h-1 overflow-hidden rounded-full bg-(--surface-interactive-hover-background)">
            <div
              className={cn("h-full", tone.meter)}
              style={{ width: `${usagePercent}%` }}
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}
