"use client";

/**
 * INPUT: Goal status projection inputs, server-derived clear reason, mutation block reason and action callbacks.
 * OUTPUT: 本地化且可换行的 Goal 状态条、完整阻塞说明与精确动作忙碌反馈。
 * POS: Goal panel renderer; lifecycle and server-derived binding policy remain in the pure model/controller.
 */

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import type { ReactNode } from "react";
import {
  CircleSlash,
  GaugeCircle,
  Loader2,
  Pause,
  Pencil,
  Play,
  RefreshCw,
  Target,
  type LucideIcon,
} from "lucide-react";

import type { TranslationKey } from "@/shared/i18n/messages";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { Goal, GoalExecutionBinding } from "@/types/conversation/goal";
import type { GoalContinuationHold } from "./goal-continuation-hold";
import type { GoalMutationBlockReason } from "./goal-lifecycle-recovery";
import {
  GOAL_PANEL_COMPACT_CLASS_NAME,
  GOAL_PANEL_STRIP_CLASS_NAME,
} from "./goal-panel-layout";
import {
  buildGoalStatusStripModel,
  type GoalBindingBadgeModel,
  type GoalStatusAction,
  type GoalStatusStripModel,
} from "./goal-model";

const GOAL_PANEL_ROW_CLASS_NAME =
  "group -mx-1 flex min-h-8 flex-wrap items-center gap-x-2 gap-y-1 px-1 py-0.5 text-(--text-default)";

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
  pendingAction?: GoalStatusAction | null;
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
  labelKey: TranslationKey;
  tone?: "danger" | "primary";
}

type GoalActionHandlers = Record<GoalStatusAction, () => void>;

const GOAL_ACTION_PRESENTATION: Record<
  GoalStatusAction,
  GoalActionPresentation
> = {
  clear: {
    Icon: CircleSlash,
    labelKey: "common.clear",
    tone: "danger",
  },
  edit: { Icon: Pencil, labelKey: "common.edit" },
  pause: { Icon: Pause, labelKey: "goal.action_pause" },
  refresh: { Icon: RefreshCw, labelKey: "common.refresh" },
  resume: {
    Icon: Play,
    labelKey: "goal.action_resume",
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
  pendingAction = null,
  scopeLabel,
  statusExtra = null,
  onClearRequest,
  onEdit,
  onPause,
  onRefresh,
  onResume,
}: GoalStatusStripProps) {
  const { locale, t } = useI18n();
  const model = buildGoalStatusStripModel({
    locale,
    t,
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
      <UiPanel className="px-3 py-1.5" padding="none" radius="lg">
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
            pendingAction={pendingAction}
          />
        </div>
        <GoalAttentionMessage
          blocker={goal.status === "blocked" ? goal.blocker ?? null : null}
          message={model.attentionMessage}
          tone={model.attentionTone}
        />
      </UiPanel>
    </div>
  );
}

function GoalLeadingIcon({ model }: { model: GoalStatusStripModel }) {
  return (
    <UiBadge
      aria-hidden="true"
      className="h-5 w-5 !p-0"
      size="xs"
      tone={model.tone}
    >
      <Target aria-hidden="true" className="h-3.5 w-3.5" />
    </UiBadge>
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
    <div className="min-w-0 flex-1 basis-40">
      <div className={cn(
        "flex min-w-0 flex-wrap items-center gap-1.5",
        getUiTypographyClassName({ role: "caption", tone: "soft", weight: "medium" }),
      )}>
        <span className="truncate">{scopeLabel}</span>
        <UiBadge
          size="xs"
          title={model.statusTitle}
          tone={model.tone}
        >
          {model.statusLabel}
        </UiBadge>
        {model.bindingBadge ? <GoalBindingBadge model={model.bindingBadge} /> : null}
        {statusExtra}
      </div>
      <UiTooltip label={objective}><div
        className={cn(
          "mt-0.5 line-clamp-1",
          getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "medium" }),
        )}
      >
        {objective}
      </div></UiTooltip>
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
      <GaugeCircle aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />
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
  pendingAction,
}: {
  actionDisabledReasons: GoalStatusStripModel["actionDisabledReasons"];
  actions: GoalStatusAction[];
  disabled: boolean;
  handlers: GoalActionHandlers;
  isLoading: boolean;
  mutationBlockReason: GoalMutationBlockReason | null;
  mutationBlocked: boolean;
  pendingAction: GoalStatusAction | null;
}) {
  const { t } = useI18n();
  return (
    <div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-1">
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
        const label = t(presentation.labelKey);
        const busy = isLoading && action === (pendingAction ?? "refresh");
        const Icon = busy ? Loader2 : presentation.Icon;
        return (
          <UiIconButton
            key={action}
            aria-busy={busy || undefined}
            aria-label={disabledReason ? `${label}: ${disabledReason}` : label}
            disabled={Boolean(disabledReason) || unavailable}
            size="sm"
            tooltip={disabledReason ?? label}
            tone={presentation.tone}
            type="button"
            variant="ghost"
            onClick={handlers[action]}
          >
            <Icon
              aria-hidden="true"
              className={busy
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
        "ml-7 whitespace-pre-wrap pb-1 [overflow-wrap:anywhere]",
        getUiTypographyClassName({
          role: "supporting",
          tone: tone === "warning" ? "warning" : "danger",
        }),
      )}
    >
      {resolvedMessage}
    </div>
  );
}
