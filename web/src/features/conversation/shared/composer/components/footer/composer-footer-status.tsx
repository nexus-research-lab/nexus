/**
 * INPUT: Composer Goal 模式、runtime 活动态及其取消/负责人控件。
 * OUTPUT: 可在普通模式左列与 Goal 模式独立状态行间重排的 Footer 状态。
 * POS: Composer Footer 的唯一运行状态投影。
 */

import type { ReactNode } from "react";
import { Target, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import type { ComposerRuntimeActivity } from "../../composer-model";

import {
  type ComposerFooterStatusProjection,
  projectComposerFooterStatus,
} from "./composer-footer-model";

export function ComposerGoalModeIndicator({
  extra,
  isCreating,
  onCancel,
  scopeLabel,
  visible,
}: {
  extra: ReactNode;
  isCreating: boolean;
  onCancel: () => void;
  scopeLabel: string;
  visible: boolean;
}) {
  const { t } = useI18n();
  if (!visible) {
    return null;
  }
  return (
    <span className="nexus-chat-composer-goal-mode flex min-w-0 flex-1 items-center gap-1.5 font-semibold text-(--primary)">
      <Target className="h-3.5 w-3.5 shrink-0" />
      <span className="shrink-0 whitespace-nowrap">{t("composer.goal_mode")}</span>
      <span className="nexus-chat-composer-goal-scope truncate font-medium text-(--text-muted)">
        {scopeLabel}
      </span>
      {extra}
      <button
        aria-label={t("composer.cancel_goal_mode")}
        className="nexus-chat-composer-goal-cancel pointer-events-auto inline-flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
        disabled={isCreating}
        onClick={onCancel}
        type="button"
      >
        <X className="h-3 w-3" />
      </button>
    </span>
  );
}

export function ComposerFooterStatus({
  activeError,
  isGoalConfirming,
  isGoalCreating,
  isPreparingAttachments,
  runtimeActivity,
}: {
  activeError: string | null;
  isGoalConfirming: boolean;
  isGoalCreating: boolean;
  isPreparingAttachments: boolean;
  runtimeActivity: ComposerRuntimeActivity;
}) {
  const { t } = useI18n();
  const status = projectComposerFooterStatus({
    activeError,
    copy: {
      compacting: t("composer.compacting_context"),
      goalCreating: t("composer.goal_normalizing"),
      goalConfirming: t("composer.goal_confirming"),
      preparingAttachments: t("composer.preparing_attachments"),
      replying: t("status.replying"),
      sending: t("status.sending"),
      stopHint: `[${t("composer.esc_stop")}]`,
    },
    isGoalCreating,
    isGoalConfirming,
    isPreparingAttachments,
    runtimeActivity,
  });
  if (!status) {
    return null;
  }
  return (
    <span className={`nexus-chat-composer-runtime-status flex min-w-0 items-center gap-2 ${status.className}`}>
      <ComposerStatusIndicator frames={status.frames} />
      <ComposerStatusMessage status={status} />
      <ComposerStatusHint status={status} />
    </span>
  );
}

function ComposerStatusIndicator({ frames }: { frames: string[] | null }) {
  if (!frames) {
    return null;
  }
  return <LoadingOrb frames={frames} />;
}

function ComposerStatusMessage({
  status,
}: {
  status: ComposerFooterStatusProjection;
}) {
  if (!status.message) {
    return null;
  }
  return <span className={status.messageClassName}>{status.message}</span>;
}

function ComposerStatusHint({
  status,
}: {
  status: ComposerFooterStatusProjection;
}) {
  if (!status.hint) {
    return null;
  }
  return <span className="text-(--text-soft)">{status.hint}</span>;
}
