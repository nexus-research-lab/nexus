// INPUT: 运行错误原文与当前翻译函数。
// OUTPUT: 可见的本地化运行问题摘要，以及按原文保留的显式诊断详情。
// POS: Scheduled 已知错误语义的唯一展示映射；未知详情不回退为卡片摘要，不按文本猜测恢复权限。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";

interface ScheduledTaskErrorCopy {
  detail: string;
  summary: string;
}

export function getScheduledTaskErrorCopy(
  value: string | null | undefined,
  t: I18nContextValue["t"],
): ScheduledTaskErrorCopy | null {
  const message = value?.trim();
  if (!message) {
    return null;
  }
  if (message === "previous run is still running; overlap_policy=skip") {
    return {
      detail: t("capability.scheduled_error_overlap_detail", { error: message }),
      summary: t("capability.scheduled_error_overlap_summary"),
    };
  }
  if (message === "Permission request timeout") {
    return {
      detail: t("capability.scheduled_error_permission_detail", { error: message }),
      summary: t("capability.scheduled_error_permission_summary"),
    };
  }
  return { detail: message, summary: t("capability.scheduled_error_unknown_summary") };
}
