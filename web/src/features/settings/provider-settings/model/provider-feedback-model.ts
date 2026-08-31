// INPUT: Provider 读取、校验、mutation 与后续刷新阶段的可验证结果。
// OUTPUT: 区分未发送、未应用、结果未知、已提交但页面过期的反馈事实。
// POS: Provider Settings 的纯失败展示模型；不执行刷新或重复 mutation。
import {
  getErrorMessage,
  projectMutationFailure,
} from "@/lib/error-message";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import type { FeedbackState } from "./provider-settings-types";

export function buildProviderErrorFeedback(
  error: unknown,
  title: string,
  fallbackMessage: string,
  t: I18nContextValue["t"],
): FeedbackState {
  const failure = projectMutationFailure(error, fallbackMessage);
  if (failure.effect === "not_applied") {
    return {
      impact: t("settings.providers.mutation_not_applied_impact"),
      message: failure.message,
      mutationEffect: failure.effect,
      nextStep: t("settings.providers.mutation_not_applied_next_step"),
      tone: "error",
      title,
    };
  }
  const copy = failure.effect === "committed"
    ? {
        impact: t("state.committed_refresh_impact"),
        title: t("settings.providers.mutation_committed_title"),
      }
    : failure.effect === "accepted"
      ? {
          impact: t("settings.providers.mutation_accepted_impact"),
          title: t("settings.providers.mutation_accepted_title"),
        }
      : {
          impact: t("feedback.unconfirmed_impact"),
          title: t("settings.providers.mutation_unknown_title"),
        };
  return {
    impact: copy.impact,
    message: failure.message,
    mutationEffect: failure.effect,
    nextStep: failure.effect === "committed"
      ? t("state.committed_refresh_next_step")
      : t("feedback.unconfirmed_next_step"),
    recoveryAction: "refresh",
    tone: "warning",
    title: copy.title,
  };
}

export function buildProviderValidationFeedback(
  title: string,
  message: string,
  t: I18nContextValue["t"],
): FeedbackState {
  return {
    impact: t("state.validation_failure_impact"),
    message,
    nextStep: t("state.validation_failure_next_step"),
    tone: "error",
    title,
  };
}

export function buildProviderReadFailureFeedback(
  error: unknown,
  fallbackMessage: string,
  t: I18nContextValue["t"],
): FeedbackState {
  return {
    impact: t("state.read_failure_impact"),
    message: getErrorMessage(error, fallbackMessage),
    nextStep: t("state.retry_next_step"),
    recoveryAction: "refresh",
    tone: "error",
    title: t("settings.providers.load_failed_title"),
  };
}

export function buildProviderCommittedRefreshFeedback(
  message: string,
  t: I18nContextValue["t"],
): FeedbackState {
  return {
    impact: t("state.committed_refresh_impact"),
    message,
    nextStep: t("state.committed_refresh_next_step"),
    recoveryAction: "refresh",
    tone: "warning",
    title: t("settings.providers.refresh_after_change_failed_title"),
  };
}

export function buildProviderFollowupRefreshFailureFeedback(
  t: I18nContextValue["t"],
): FeedbackState {
  return {
    impact: t("settings.providers.action_refresh_failed_impact"),
    message: t("settings.providers.action_refresh_failed_message"),
    nextStep: t("settings.providers.action_refresh_failed_next_step"),
    recoveryAction: "refresh",
    tone: "warning",
    title: t("settings.providers.action_refresh_failed_title"),
  };
}
