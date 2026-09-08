/**
 * INPUT: Composer Goal 模式、runtime 活动态及其取消/负责人控件。
 * OUTPUT: 使用公共 caption 排版、可在普通左列与 Goal 独立状态行间重排的唯一状态面。
 * POS: Composer Footer 的唯一运行状态投影。
 */

import type { ReactNode } from "react";
import { Target, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ComposerRuntimeActivity } from "../../composer-model";

import {
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
    <span className={`nexus-chat-composer-goal-mode flex min-w-0 flex-1 items-center gap-1.5 ${getUiTypographyClassName({ role: "caption", tone: "brand", weight: "semibold" })}`}>
      <Target className="h-3.5 w-3.5 shrink-0" />
      <span className="shrink-0 whitespace-nowrap">{t("composer.goal_mode")}</span>
      <span className={`nexus-chat-composer-goal-scope truncate ${getUiTypographyClassName({ role: "caption", tone: "muted", weight: "medium" })}`}>
        {scopeLabel}
      </span>
      {extra}
      <UiIconButton
        aria-label={t("composer.cancel_goal_mode")}
        className="nexus-chat-composer-goal-cancel pointer-events-auto shrink-0"
        disabled={isCreating}
        onClick={onCancel}
        size="xs"
        tooltip={t("composer.cancel_goal_mode")}
        variant="ghost"
      >
        <X className="h-3 w-3" />
      </UiIconButton>
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
    <span
      className={`nexus-chat-composer-runtime-status flex min-w-0 items-center gap-2 ${getUiTypographyClassName({ role: "caption", tone: status.tone })}`}
      data-composer-status={status.kind}
    >
      {status.indicator ? <LoadingOrb variant={status.indicator} /> : null}
      {status.message ? <span className="min-w-0 [overflow-wrap:anywhere]">{status.message}</span> : null}
      {status.hint ? <span className="text-(--text-soft)">{status.hint}</span> : null}
    </span>
  );
}
