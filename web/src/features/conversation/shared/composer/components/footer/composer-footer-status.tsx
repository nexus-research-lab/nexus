import type { ReactNode } from "react";
import { Target, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";

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
    <span className="inline-flex min-w-0 items-center gap-1.5 font-semibold text-(--primary)">
      <Target className="h-3.5 w-3.5 shrink-0" />
      <span>{t("composer.goal_mode")}</span>
      <span className="truncate font-medium text-(--text-muted)">
        {scopeLabel}
      </span>
      {extra}
      <button
        aria-label={t("composer.cancel_goal_mode")}
        className="pointer-events-auto inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-[5px] text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
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
  canStopGeneration,
  isDispatching,
  isGoalCreating,
  isPreparingAttachments,
}: {
  activeError: string | null;
  canStopGeneration: boolean;
  isDispatching: boolean;
  isGoalCreating: boolean;
  isPreparingAttachments: boolean;
}) {
  const { t } = useI18n();
  const status = projectComposerFooterStatus({
    activeError,
    canStopGeneration,
    copy: {
      goalCreating: t("composer.goal_normalizing"),
      preparingAttachments: t("composer.preparing_attachments"),
      replying: t("status.replying"),
      sending: t("status.sending"),
      stopHint: `[${t("composer.esc_stop")}]`,
    },
    isDispatching,
    isGoalCreating,
    isPreparingAttachments,
  });
  if (!status) {
    return null;
  }
  return (
    <span className={`flex items-center gap-2 ${status.className}`}>
      <ComposerStatusIndicator frames={status.frames} />
      <span className={status.messageClassName}>{status.message}</span>
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
