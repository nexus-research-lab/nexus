// INPUT: 历史命令/剪贴板的结果事实与当前翻译函数。
// OUTPUT: 当前语言的标题、影响、下一步和语义色；成功命令与历史刷新分别表达。
// POS: Scheduled 历史反馈纯投影；不保存旧翻译或内部错误，不判断 mutation 是否可以重放。

import type { MutationFailureEffect } from "@/lib/error-message";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

export type RunHistoryAction = "recover" | "retry" | "retryDelivery";

export type RunHistoryFeedback =
  | { action: RunHistoryAction; kind: "completed" }
  | { kind: "refresh_failed" }
  | { action: RunHistoryAction; effect: MutationFailureEffect; kind: "failed" }
  | { deletion: "deleting" | "review_required" | null; kind: "blocked" }
  | { copied: boolean; kind: "clipboard" };

interface RunHistoryFeedbackPresentation {
  impact: string;
  nextStep: string;
  title: string;
  tone: "success" | "warning" | "error";
}

const ACTION_COPY: Record<RunHistoryAction, { failure: TranslationKey; success: TranslationKey }> = {
  recover: { failure: "capability.scheduled_history_recover_failed", success: "capability.scheduled_history_recover_completed" },
  retry: { failure: "capability.scheduled_history_retry_failed", success: "capability.scheduled_history_retry_completed" },
  retryDelivery: { failure: "capability.scheduled_history_delivery_retry_failed", success: "capability.scheduled_history_delivery_retry_completed" },
};

export function projectRunHistoryFeedback(
  feedback: RunHistoryFeedback | null,
  t: I18nContextValue["t"],
): RunHistoryFeedbackPresentation | null {
  if (!feedback) return null;
  switch (feedback.kind) {
    case "completed":
      return {
        impact: t("capability.scheduled_history_completed_impact"),
        nextStep: t("capability.scheduled_history_completed_next_step"),
        title: t(ACTION_COPY[feedback.action].success),
        tone: "success",
      };
    case "refresh_failed":
      return {
        impact: t("capability.scheduled_history_submitted_stale_impact"),
        nextStep: t("capability.scheduled_history_submitted_stale_next_step"),
        title: t("capability.scheduled_history_submitted_stale_title"),
        tone: "warning",
      };
    case "failed": {
      const effect = feedback.effect;
      return {
        impact: t(`capability.scheduled_mutation_${effect}_impact`),
        nextStep: t(effect === "not_applied"
          ? "capability.scheduled_mutation_not_applied_next_step"
          : "capability.scheduled_mutation_unknown_next_step"),
        title: t(effect === "not_applied" ? ACTION_COPY[feedback.action].failure : `capability.scheduled_mutation_${effect}_title`),
        tone: effect === "not_applied" ? "error" : "warning",
      };
    }
    case "blocked":
      return {
        impact: t("capability.scheduled_history_blocked_impact"),
        nextStep: t(feedback.deletion === "review_required"
          ? "capability.scheduled_history_blocked_review_next_step"
          : feedback.deletion
            ? "capability.scheduled_history_blocked_deleting_next_step"
            : "capability.scheduled_history_blocked_next_step"),
        title: t("capability.scheduled_history_blocked_title"),
        tone: "warning",
      };
    case "clipboard":
      return {
        impact: t(feedback.copied ? "capability.scheduled_history_copy_success_impact" : "capability.scheduled_history_copy_failed_impact"),
        nextStep: t(feedback.copied ? "capability.scheduled_history_copy_success_next_step" : "capability.scheduled_history_copy_failed_next_step"),
        title: t(feedback.copied ? "capability.scheduled_history_copy_success_title" : "capability.scheduled_history_copy_failed_title"),
        tone: feedback.copied ? "success" : "error",
      };
  }
}
