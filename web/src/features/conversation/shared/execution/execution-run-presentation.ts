// INPUT: 权威节点/NodeRun 状态、耗时、可选观测时间与当前界面语言。
// OUTPUT: 已知运行态的可读标签与合法耗时进位；未知状态不回显协议值，缺失耗时回退合法观测时间。
// POS: WorkGraph 节点及运行详情的状态标签/时间展示唯一所有者，不推断执行状态。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { ExecutionAttemptView, ExecutionGraphNodeRunView } from "@/types/conversation/execution";

type ExecutionTimeLocale = Pick<I18nContextValue, "locale" | "t">;


const RUN_STATUS_LABEL_KEY: Record<ExecutionAttemptView["status"], TranslationKey> = {
  cancelled: "execution.attempt_cancelled",
  failed: "execution.attempt_failed",
  interrupted: "execution.attempt_interrupted",
  pending: "execution.attempt_pending",
  running: "execution.attempt_running",
  succeeded: "execution.attempt_succeeded",
  timed_out: "execution.attempt_timed_out",
};

export function getExecutionRunStatusLabel(status: string | undefined, t: I18nContextValue["t"]): string {
  const key = status?.trim() ?? "";
  return t(Object.hasOwn(RUN_STATUS_LABEL_KEY, key)
    ? RUN_STATUS_LABEL_KEY[key as keyof typeof RUN_STATUS_LABEL_KEY]
    : "execution.attempt_unknown");
}

export function formatExecutionDuration(
  durationMs: number | undefined,
  { locale, t }: ExecutionTimeLocale,
): string | null {
  if (typeof durationMs !== "number" || !Number.isFinite(durationMs) || durationMs < 0) {
    return null;
  }
  const milliseconds = Math.round(durationMs);
  if (!Number.isSafeInteger(milliseconds)) return null;
  if (milliseconds < 1_000) {
    return t("execution.duration_milliseconds", { milliseconds });
  }
  const seconds = Math.round(milliseconds / 1_000);
  if (seconds < 60) {
    const precise = milliseconds < 10_000;
    return t("execution.duration_seconds", {
      seconds: new Intl.NumberFormat(locale, {
        minimumFractionDigits: precise ? 1 : 0,
        maximumFractionDigits: precise ? 1 : 0,
      }).format(precise ? milliseconds / 1_000 : seconds),
    });
  }
  return t("execution.duration_minutes", {
    minutes: Math.floor(seconds / 60),
    seconds: seconds % 60,
  });
}

export function formatExecutionRunTime(
  run: ExecutionGraphNodeRunView,
  context: ExecutionTimeLocale,
): string | null {
  const duration = formatExecutionDuration(run.duration_ms, context);
  if (duration !== null) return duration;
  for (const value of [run.finished_at, run.started_at]) {
    if (!value) continue;
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) continue;
    return new Intl.DateTimeFormat(context.locale, {
      hour: "2-digit", minute: "2-digit",
    }).format(date);
  }
  return null;
}
