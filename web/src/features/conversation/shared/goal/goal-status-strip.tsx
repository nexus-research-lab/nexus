"use client";

/**
 * INPUT: Goal status projection inputs, server-derived clear reason, mutation block reason and action callbacks.
 * OUTPUT: accessible Goal status strip with primary lifecycle, descender-safe compact labels and only meaningful WorkGraph binding state.
 * POS: Goal panel renderer; lifecycle and server-derived binding policy remain in the pure model/controller.
 */

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

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { Goal, GoalExecutionBinding } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import type { GoalMutationBlockReason } from "./goal-lifecycle-recovery";
import {
  buildGoalStatusStripModel,
  GOAL_PANEL_COMPACT_CLASS_NAME,
  GOAL_PANEL_LEADING_ICON_CLASS_NAME,
  GOAL_PANEL_ROW_CLASS_NAME,
  GOAL_PANEL_STRIP_CLASS_NAME,
  GOAL_PANEL_SURFACE_CLASS_NAME,
  type GoalBindingBadgeModel,
  type GoalStatusAction,
  type GoalStatusStripModel,
} from "./goal-model";

interface GoalStatusStripProps {
  canResume: boolean;
  clearDisabledReason?: string | null;
  compact: boolean;
  continuationHold?: GoalContinuationHold | null;
  disabled: boolean;
  executionBinding?: GoalExecutionBinding | null;
  goal: Goal;
  isGenerating: boolean;
  isLoading: boolean;
  mutationBlockReason: GoalMutationBlockReason | null;
  mutationBlocked: boolean;
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

const GOAL_BINDING_BADGE_TONE: Record<
  GoalBindingBadgeModel["tone"],
  "danger" | "idle" | "info" | "warning"
> = {
  conflict: "danger",
  confirmed: "info",
  pending: "warning",
  unavailable: "idle",
};

export function GoalStatusStrip({
  canResume,
  clearDisabledReason = null,
  compact,
  continuationHold = null,
  disabled,
  executionBinding = null,
  goal,
  isGenerating,
  isLoading,
  mutationBlockReason,
  mutationBlocked,
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
    clearDisabledReason,
    continuationHold,
    executionBinding,
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
      className={
        compact ? GOAL_PANEL_COMPACT_CLASS_NAME : GOAL_PANEL_STRIP_CLASS_NAME
      }
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
          <GoalUsage label={model.usageLabel} />
          <GoalStatusActions
            actions={model.actions}
            actionDisabledReasons={model.actionDisabledReasons}
            disabled={disabled}
            handlers={actionHandlers}
            isLoading={isLoading}
            mutationBlockReason={mutationBlockReason}
            mutationBlocked={mutationBlocked}
          />
        </div>
        <GoalAttentionMessage
          blocker={goal.status === "blocked" ? goal.blocker ?? null : null}
          message={model.attentionMessage}
          tone={model.attentionTone}
        />
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
      <div className={cn(
        "flex min-w-0 items-center gap-1.5",
        getUiTypographyClassName({ role: "caption", tone: "soft", weight: "medium" }),
      )}>
        <span className="truncate">{scopeLabel}</span>
        <UiBadge
          size="xs"
          title={model.statusTitle}
          tone={model.tone.badge}
        >
          {model.statusLabel}
        </UiBadge>
        {model.bindingBadge ? <GoalBindingBadge model={model.bindingBadge} /> : null}
        {statusExtra}
      </div>
      <div
        className={cn(
          "mt-0.5 line-clamp-1",
          getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "medium" }),
        )}
        title={objective}
      >
        {objective}
      </div>
    </div>
  );
}

function GoalBindingBadge({
  model,
}: {
  model: GoalBindingBadgeModel;
}) {
  const { t } = useI18n();
  const title = t(model.titleKey);
  return (
    <UiBadge
      aria-label={title}
      className="max-w-32 truncate"
      data-goal-binding-state={model.state}
      size="xs"
      title={title}
      tone={GOAL_BINDING_BADGE_TONE[model.tone]}
    >
      {t(model.labelKey)}
    </UiBadge>
  );
}

function GoalUsage({ label }: { label: string | null }) {
  if (!label) {
    return null;
  }
  return (
    <span className={cn(
      "hidden shrink-0 items-center gap-1 tabular-nums sm:inline-flex",
      getUiTypographyClassName({ role: "caption", tone: "muted", weight: "medium" }),
    )}>
      <GaugeCircle className="h-3.5 w-3.5 shrink-0" />
      <span>{label}</span>
    </span>
  );
}

function GoalStatusActions({
  actionDisabledReasons,
  actions,
  disabled,
  handlers,
  isLoading,
  mutationBlockReason,
  mutationBlocked,
}: {
  actionDisabledReasons: GoalStatusStripModel["actionDisabledReasons"];
  actions: GoalStatusAction[];
  disabled: boolean;
  handlers: GoalActionHandlers;
  isLoading: boolean;
  mutationBlockReason: GoalMutationBlockReason | null;
  mutationBlocked: boolean;
}) {
  const { t } = useI18n();
  return (
    <div className="ml-auto flex shrink-0 items-center gap-1">
      {actions.map((action) => {
        const presentation = GOAL_ACTION_PRESENTATION[action];
        const disabledReason = actionDisabledReasons[action]
          ?? (mutationBlocked && action !== "refresh"
            ? t(mutationBlockReason === "stale_read"
              ? "goal.reliability.action_stale"
              : "goal.reliability.action_locked")
            : null);
        const unavailable = isLoading || (
          action !== "refresh" && disabled
        );
        const { Icon } = presentation;
        return (
          <UiIconButton
            key={action}
            aria-label={disabledReason
              ? `${presentation.label}：${disabledReason}`
              : presentation.label}
            disabled={Boolean(disabledReason) || unavailable}
            size="sm"
            title={disabledReason ?? presentation.label}
            tone={presentation.tone}
            type="button"
            variant="ghost"
            onClick={handlers[action]}
          >
            <Icon
              className={action === "refresh" && isLoading
                ? getUiSpinnerClassName({ size: "md" })
                : "h-4 w-4"}
            />
          </UiIconButton>
        );
      })}
    </div>
  );
}

function GoalAttentionMessage({
  blocker,
  message,
  tone,
}: {
  blocker: Goal["blocker"];
  message: string | null;
  tone: GoalStatusStripModel["attentionTone"];
}) {
  const { t } = useI18n();
  const localizedBlocker = tone === "warning" && blocker
    ? t("goal.blocker_attention", {
      neededInput: blocker.needed_input,
      reason: blocker.reason,
    })
    : null;
  const resolvedMessage = localizedBlocker ?? message;
  if (!resolvedMessage) {
    return null;
  }
  return (
    <div
      className={cn(
        "ml-7 line-clamp-1 pb-1",
        getUiTypographyClassName({
          role: "caption",
          tone: tone === "warning" ? "warning" : "danger",
        }),
      )}
    >
      {resolvedMessage}
    </div>
  );
}
