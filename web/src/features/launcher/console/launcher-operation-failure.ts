/**
 * INPUT: Launcher 已判定的失败阶段、写入结果证据与用户可执行的单一恢复动作。
 * OUTPUT: 不泄露内部错误、完整回答 Problem / Impact / Recovery 的持久反馈条。
 * POS: Launcher 读取、导航准备和结果未知写入之间的用户语义投影；不执行请求或重试。
 */
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import type { MutationFailureEffect } from "@/lib/error-message";

export type LauncherOperationFailure =
  | { kind: "query_read" }
  | { kind: "room_read" }
  | { kind: "target_missing" }
  | { kind: "main_agent_missing" }
  | { effect: MutationFailureEffect; kind: "direct_room" };

export function projectLauncherOperationFailure(
  t: I18nContextValue["t"],
  failure: LauncherOperationFailure,
  onRecover: () => void,
): FeedbackBannerProps {
  switch (failure.kind) {
    case "query_read":
      return failureBanner(
        t("launcher.failure.query_title"),
        t("launcher.failure.query_impact"),
        t("launcher.failure.retry_query"),
        onRecover,
      );
    case "room_read":
      return failureBanner(
        t("launcher.failure.room_title"),
        t("launcher.failure.room_impact"),
        t("launcher.failure.retry_room"),
        onRecover,
      );
    case "target_missing":
      return failureBanner(
        t("launcher.failure.target_title"),
        t("launcher.failure.target_impact"),
        t("launcher.failure.open_workspace"),
        onRecover,
      );
    case "main_agent_missing":
      return failureBanner(
        t("launcher.failure.main_agent_title"),
        t("launcher.failure.main_agent_impact"),
        t("launcher.failure.open_workspace"),
        onRecover,
      );
    case "direct_room": {
      const notApplied = failure.effect === "not_applied";
      return {
        action: {
          label: t(notApplied
            ? "launcher.failure.retry_dm"
            : "launcher.failure.open_workspace"),
          onClick: onRecover,
        },
        impact: directRoomImpact(t, failure.effect),
        title: t("launcher.failure.direct_room_title"),
        tone: notApplied ? "error" : "warning",
      };
    }
  }
}

function directRoomImpact(
  t: I18nContextValue["t"],
  effect: MutationFailureEffect,
): string {
  switch (effect) {
    case "not_applied":
      return t("launcher.failure.direct_room_not_applied_impact");
    case "accepted":
      return t("launcher.failure.direct_room_accepted_impact");
    case "committed":
      return t("launcher.failure.direct_room_committed_impact");
    case "unknown":
      return t("launcher.failure.direct_room_unknown_impact");
  }
}

function failureBanner(
  title: string,
  impact: string,
  actionLabel: string,
  onRecover: () => void,
): FeedbackBannerProps {
  return {
    action: { label: actionLabel, onClick: onRecover },
    impact,
    title,
    tone: "error",
  };
}
