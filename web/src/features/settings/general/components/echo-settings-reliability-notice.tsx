/**
 * INPUT: Echo Problem/Impact/Recovery 文案与当前可安全执行的领域恢复动作。
 * OUTPUT: 持续可发现、窄屏不截断且不会自动重放 PUT 的设置提示。
 * POS: 主动跟进设置恢复视图；不推断保存结果。
 */
"use client";

import { CircleAlert, CircleCheck, RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";

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

  return (
    <section
      aria-live="polite"
      className={cn(
        "flex min-w-0 items-start gap-2.5 rounded-[10px] border px-3 py-2.5 text-xs",
        success
          ? "border-[color:color-mix(in_srgb,var(--success)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_5%,transparent)]"
          : "border-[color:color-mix(in_srgb,var(--warning)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_5%,transparent)]",
      )}
      data-echo-reliability={feedback.tone}
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
        {!success ? (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {recovery.canFinishDisabling ? (
              <UiButton
                aria-busy={recovery.repairing}
                disabled={recovery.repairing || recovery.checking}
                onClick={recovery.finishDisabling}
                size="xs"
                tone="primary"
                variant="surface"
              >
                <RefreshCw
                  aria-hidden="true"
                  className={cn("h-3.5 w-3.5", recovery.repairing && "animate-spin")}
                />
                {recovery.repairing
                  ? t("settings.general.echo_finishing_disable")
                  : t("settings.general.echo_finish_disable")}
              </UiButton>
            ) : recovery.canCompare ? (
              <UiButton
                disabled={recovery.checking}
                onClick={recovery.reapplyChange}
                size="xs"
                tone="primary"
                variant="surface"
              >
                {t("settings.general.echo_reapply")}
              </UiButton>
            ) : recovery.canCheckLatest ? (
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
                  ? t("settings.general.echo_checking")
                  : t("settings.general.echo_check_latest")}
              </UiButton>
            ) : null}
          </div>
        ) : null}
      </div>
    </section>
  );
}
