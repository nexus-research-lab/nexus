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
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiIconButton } from "@/shared/ui/button/button";
import type { Goal } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import {
  buildGoalStatusStripModel,
  GOAL_PANEL_BADGE_CLASS_NAME,
  GOAL_PANEL_COMPACT_CLASS_NAME,
  GOAL_PANEL_LEADING_ICON_CLASS_NAME,
  GOAL_PANEL_ROW_CLASS_NAME,
  GOAL_PANEL_STRIP_CLASS_NAME,
  GOAL_PANEL_SURFACE_CLASS_NAME,
  type GoalStatusAction,
  type GoalStatusStripModel,
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

interface GoalActionPresentation {
  Icon: LucideIcon;
  label: string;
  requiresIdle: boolean;
  tone?: "danger" | "primary";
}

type GoalActionHandlers = Record<GoalStatusAction, () => void>;

const GOAL_ACTION_PRESENTATION: Record<
  GoalStatusAction,
  GoalActionPresentation
> = {
  clear: {
    Icon: CircleSlash,
    label: "清除",
    requiresIdle: true,
    tone: "danger",
  },
  edit: { Icon: Pencil, label: "编辑", requiresIdle: true },
  pause: { Icon: Pause, label: "暂停", requiresIdle: true },
  refresh: { Icon: RefreshCw, label: "刷新", requiresIdle: false },
  resume: {
    Icon: Play,
    label: "继续",
    requiresIdle: true,
    tone: "primary",
  },
};

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
  const model = buildGoalStatusStripModel({
    canResume,
    continuationHold,
    error,
    goal,
    isGenerating,
  });
  const actionHandlers: GoalActionHandlers = {
    clear: onClearRequest,
    edit: onEdit,
    pause: onPause,
    refresh: onRefresh,
    resume: onResume,
  };

  return (
    <div
      className={cn(
        GOAL_PANEL_STRIP_CLASS_NAME,
        compact && GOAL_PANEL_COMPACT_CLASS_NAME,
      )}
    >
      <div className={GOAL_PANEL_SURFACE_CLASS_NAME}>
        <div className={GOAL_PANEL_ROW_CLASS_NAME}>
          <GoalLeadingIcon model={model} />
          <GoalStatusSummary
            model={model}
            objective={goal.objective}
            scopeLabel={scopeLabel}
            statusExtra={statusExtra}
          />
          <GoalBudget label={model.budgetLabel} />
          <GoalStatusActions
            actions={model.actions}
            disabled={disabled}
            handlers={actionHandlers}
            isLoading={isLoading}
          />
        </div>
        <GoalAttentionMessage message={model.attentionMessage} />
        <GoalUsageMeter model={model} />
      </div>
    </div>
  );
}

function GoalLeadingIcon({ model }: { model: GoalStatusStripModel }) {
  return (
    <span className={cn(GOAL_PANEL_LEADING_ICON_CLASS_NAME, model.tone.icon)}>
      <Target className="h-3.5 w-3.5" />
    </span>
  );
}

function GoalStatusSummary({
  model,
  objective,
  scopeLabel,
  statusExtra,
}: {
  model: GoalStatusStripModel;
  objective: string;
  scopeLabel: string;
  statusExtra: ReactNode;
}) {
  return (
    <div className="min-w-0 flex-1">
      <div className="flex min-w-0 items-center gap-1.5 text-[10px] font-medium text-(--text-soft)">
        <span className="truncate">{scopeLabel}</span>
        <span
          className={cn(GOAL_PANEL_BADGE_CLASS_NAME, model.tone.badge)}
          title={model.statusTitle}
        >
          {model.statusLabel}
        </span>
        <GoalExecutionState model={model} />
        {statusExtra}
      </div>
      <div className="mt-0.5 line-clamp-1 text-[12px] font-medium leading-5 text-(--text-strong)">
        {objective}
      </div>
    </div>
  );
}

function GoalExecutionState({ model }: { model: GoalStatusStripModel }) {
  if (!model.isExecuting) {
    return null;
  }
  return <span className={cn("font-semibold", model.tone.text)}>执行中</span>;
}

function GoalBudget({ label }: { label: string | null }) {
  if (!label) {
    return null;
  }
  return (
    <span
      className="hidden h-6 max-w-[128px] shrink-0 items-center gap-1 truncate rounded-[8px] px-1.5 text-[11px] font-medium text-(--text-muted) sm:inline-flex"
      title="Token 使用"
    >
      <GaugeCircle className="h-3.5 w-3.5 shrink-0" />
      <span className="truncate">{label}</span>
    </span>
  );
}

function GoalStatusActions({
  actions,
  disabled,
  handlers,
  isLoading,
}: {
  actions: GoalStatusAction[];
  disabled: boolean;
  handlers: GoalActionHandlers;
  isLoading: boolean;
}) {
  const unavailable = disabled || isLoading;
  return (
    <div className="ml-auto flex shrink-0 items-center gap-1">
      {actions.map((action) => {
        const presentation = GOAL_ACTION_PRESENTATION[action];
        const { Icon } = presentation;
        return (
          <UiIconButton
            key={action}
            aria-label={presentation.label}
            disabled={presentation.requiresIdle && unavailable}
            size="sm"
            title={presentation.label}
            tone={presentation.tone}
            type="button"
            variant="ghost"
            onClick={handlers[action]}
          >
            <Icon
              className={cn(
                "h-4 w-4",
                action === "refresh" && isLoading && "animate-spin",
              )}
            />
          </UiIconButton>
        );
      })}
    </div>
  );
}

function GoalAttentionMessage({ message }: { message: string | null }) {
  if (!message) {
    return null;
  }
  return (
    <div className="ml-7 line-clamp-1 pb-1 text-[11px] leading-4 text-(--destructive)">
      {message}
    </div>
  );
}

function GoalUsageMeter({ model }: { model: GoalStatusStripModel }) {
  if (model.usagePercent === null) {
    return null;
  }
  return (
    <div className="ml-7 h-1 overflow-hidden rounded-full bg-(--surface-interactive-hover-background)">
      <div
        className={cn("h-full", model.tone.meter)}
        style={{ width: `${model.usagePercent}%` }}
      />
    </div>
  );
}
