/**
 * INPUT: Preferences 失败的 Problem/Impact/Recovery 文案和已确认安全的对账动作。
 * OUTPUT: 持续、可读、不自动消失的设置恢复提示。
 * POS: General/Runtime Preferences 共用视图；不推断结果或自动重放 PATCH。
 */
"use client";

import { RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type {
  PreferenceFeedback,
  PreferenceRecoveryControls,
} from "../model/settings-preferences-model";

export function PreferencesReliabilityNotice({
  feedback,
  recovery,
}: {
  feedback: PreferenceFeedback | null;
  recovery?: PreferenceRecoveryControls;
}) {
  const { t } = useI18n();
  if (!feedback) {
    return null;
  }
  const success = feedback.tone === "success";
  const primaryAction = !success && recovery
    ? recovery.canRepairProjection
      ? {
          busy: recovery.repairing,
          busyLabel: t("settings.general.preferences_repairing_projection"),
          disabled: recovery.checking,
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("settings.general.preferences_repair_projection"),
          onClick: recovery.repairProjection,
        }
      : recovery.canCompare
        ? {
            disabled: recovery.checking,
            label: t("settings.general.preferences_reapply_draft"),
            onClick: recovery.reapplyDraft,
          }
        : {
            busy: recovery.checking,
            busyLabel: t("settings.general.preferences_checking"),
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("settings.general.preferences_check_latest"),
            onClick: recovery.checkLatest,
          }
    : undefined;
  if (success) {
    return (
      <UiResourceState
        className="min-h-0 px-3 py-2.5"
        data-preferences-reliability={feedback.tone}
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
      data-preferences-reliability={feedback.tone}
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
