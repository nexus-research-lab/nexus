/**
 * INPUT: Echo Problem/Impact/Recovery 文案与当前可安全执行的领域恢复动作。
 * OUTPUT: 持续可发现、窄屏不截断且不会自动重放 PUT 的设置提示。
 * POS: 主动跟进设置恢复视图；不推断保存结果。
 */
"use client";

import { RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type {
  EchoSettingsFeedback,
  EchoSettingsRecoveryControls,
} from "../model/echo-settings-reliability-model";

export function EchoSettingsReliabilityNotice({
  feedback,
  recovery,
}: {
  feedback: EchoSettingsFeedback | null;
  recovery: EchoSettingsRecoveryControls;
}) {
  const { t } = useI18n();
  if (!feedback) {
    return null;
  }
  const success = feedback.tone === "success";
  const primaryAction = success
    ? undefined
    : recovery.canFinishDisabling
      ? {
          busy: recovery.repairing,
          busyLabel: t("settings.general.echo_finishing_disable"),
          disabled: recovery.checking,
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("settings.general.echo_finish_disable"),
          onClick: recovery.finishDisabling,
        }
      : recovery.canCompare
        ? {
            disabled: recovery.checking,
            label: t("settings.general.echo_reapply"),
            onClick: recovery.reapplyChange,
          }
        : recovery.canCheckLatest
          ? {
              busy: recovery.checking,
              busyLabel: t("settings.general.echo_checking"),
              icon: <RefreshCw className="h-3.5 w-3.5" />,
              label: t("settings.general.echo_check_latest"),
              onClick: recovery.checkLatest,
            }
          : undefined;
  if (success) {
    return (
      <UiResourceState
        className="min-h-0 px-3 py-2.5"
        data-echo-reliability={feedback.tone}
        description={feedback.message}
        size="sm"
        state="success"
        title={feedback.title}
        urgency="polite"
        variant="card"
      />
    );
  }

  return (
    <UiResourceState
      className="min-h-0 px-3 py-2.5"
      data-echo-reliability={feedback.tone}
      impact={feedback.impact}
      primaryAction={primaryAction}
      size="sm"
      state="error"
      title={feedback.title}
      tone={feedback.tone === "warning" ? "warning" : "danger"}
      urgency="polite"
      variant="card"
    />
  );
}
