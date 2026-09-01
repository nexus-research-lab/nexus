/**
 * INPUT: Preferences 失败的 Problem/Impact/Recovery 文案和已确认安全的对账动作。
 * OUTPUT: 持续、可读、不自动消失的设置恢复提示。
 * POS: General/Runtime Preferences 共用视图；不推断结果或自动重放 PATCH。
 */
"use client";

import { CircleAlert, CircleCheck, RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";

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

  return (
    <section
      aria-live="polite"
      className={cn(
        "flex min-w-0 items-start gap-2.5 rounded-[10px] border px-3 py-2.5 text-xs",
        success
          ? "border-[color:color-mix(in_srgb,var(--success)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_5%,transparent)]"
          : "border-[color:color-mix(in_srgb,var(--warning)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_5%,transparent)]",
      )}
      data-preferences-reliability={feedback.tone}
      role="status"
    >
      {success ? (
        <CircleCheck aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-(--success)" />
      ) : (
        <CircleAlert aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)" />
      )}
      <div className="min-w-0 flex-1">
        <p className="font-semibold leading-5 text-(--text-strong)">{feedback.title}</p>
        {success ? (
          <p className="mt-0.5 break-words leading-5 text-(--text-muted)">
            {feedback.message}
          </p>
        ) : <RecoverySummary className="mt-0.5" impact={feedback.impact} />}
        {!success && recovery ? (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {recovery.canRepairProjection ? (
              <UiButton
                aria-busy={recovery.repairing}
                disabled={recovery.repairing || recovery.checking}
                onClick={recovery.repairProjection}
                size="xs"
                tone="primary"
                variant="surface"
              >
                <RefreshCw
                  aria-hidden="true"
                  className={cn("h-3.5 w-3.5", recovery.repairing && "animate-spin")}
                />
                {recovery.repairing
                  ? t("settings.general.preferences_repairing_projection")
                  : t("settings.general.preferences_repair_projection")}
              </UiButton>
            ) : recovery.canCompare ? (
              <UiButton
                disabled={recovery.checking}
                onClick={recovery.useLatest}
                size="xs"
                variant="text"
              >
                {t("settings.general.preferences_use_latest")}
              </UiButton>
            ) : (
              <UiButton
                aria-busy={recovery.checking}
                disabled={recovery.checking}
                onClick={recovery.checkLatest}
                size="xs"
                tone="primary"
                variant="surface"
              >
                <RefreshCw
                  aria-hidden="true"
                  className={cn("h-3.5 w-3.5", recovery.checking && "animate-spin")}
                />
                {recovery.checking
                  ? t("settings.general.preferences_checking")
                  : t("settings.general.preferences_check_latest")}
              </UiButton>
            )}
          </div>
        ) : null}
      </div>
    </section>
  );
}
