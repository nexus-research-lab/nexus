/**
 * INPUT: Conversation reliability 的 transport、provider retry 与用户级失败分类。
 * OUTPUT: 与 Composer 输入框内缘对齐、不含技术详情的单句状态和必要刷新动作。
 * POS: DM 与 Room 共用的可靠性状态展示；不进入消息 Feed。
 */
"use client";

import { LoaderCircle, RefreshCw, TriangleAlert, WifiOff } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import type {
  ConversationFailureCode,
  ConversationReliabilitySnapshot,
} from "@/types/agent/agent-conversation-reliability";

import { CONVERSATION_COMPOSER_LANE_CLASS_NAME } from "./conversation-panel-styles";

interface ConversationFailureCopyKeys {
  impact: TranslationKey;
  title: TranslationKey;
}

const FAILURE_COPY_KEYS: Record<ConversationFailureCode, ConversationFailureCopyKeys> = {
  connection_unavailable: {
    impact: "conversation.reliability.connection_unavailable_impact",
    title: "conversation.reliability.connection_unavailable",
  },
  delivery_unknown: {
    impact: "conversation.reliability.delivery_unknown_impact",
    title: "conversation.reliability.delivery_unknown",
  },
  permission_not_sent: {
    impact: "conversation.reliability.permission_not_sent_impact",
    title: "conversation.reliability.permission_not_sent",
  },
  provider_configuration: {
    impact: "conversation.reliability.provider_configuration_impact",
    title: "conversation.reliability.provider_configuration",
  },
  provider_unavailable: {
    impact: "conversation.reliability.provider_unavailable_impact",
    title: "conversation.reliability.provider_unavailable",
  },
  request_rejected: {
    impact: "conversation.reliability.request_rejected_impact",
    title: "conversation.reliability.request_rejected",
  },
  round_failed: {
    impact: "conversation.reliability.round_failed_impact",
    title: "conversation.reliability.round_failed",
  },
  safety_rejected: {
    impact: "conversation.reliability.safety_rejected_impact",
    title: "conversation.reliability.safety_rejected",
  },
  session_load_failed: {
    impact: "conversation.reliability.session_load_failed_impact",
    title: "conversation.reliability.session_load_failed",
  },
  usage_limited: {
    impact: "conversation.reliability.usage_limited_impact",
    title: "conversation.reliability.usage_limited",
  },
  validation_failed: {
    impact: "conversation.reliability.validation_failed_impact",
    title: "conversation.reliability.validation_failed",
  },
};

const WARNING_FAILURE_CODES = new Set<ConversationFailureCode>([
  "connection_unavailable",
  "delivery_unknown",
  "provider_unavailable",
  "session_load_failed",
  "usage_limited",
]);

const RECONCILABLE_FAILURE_CODES = new Set<ConversationFailureCode>([
  "delivery_unknown",
  "session_load_failed",
]);

export function ConversationReliabilityNotice({
  compact,
  isReconciling,
  onReconcile,
  reliability,
}: {
  compact: boolean;
  isReconciling: boolean;
  onReconcile: () => void;
  reliability: ConversationReliabilitySnapshot;
}) {
  const { t } = useI18n();
  const presentation = reliability.transport_phase === "recovering"
    ? {
        icon: LoaderCircle,
        message: t("conversation.reliability.connection_recovering"),
        spinning: true,
        tone: "recovering" as const,
        failureCode: null,
      }
    : reliability.transport_phase === "unavailable"
    ? {
        icon: WifiOff,
        failureCode: "connection_unavailable" as const,
        spinning: false,
        tone: "failure" as const,
      }
    : reliability.provider_retry
    ? {
        icon: LoaderCircle,
        message: t("conversation.reliability.provider_retrying"),
        spinning: true,
        tone: "recovering" as const,
        failureCode: null,
      }
    : reliability.failure
    ? {
        icon: TriangleAlert,
        failureCode: reliability.failure.code,
        spinning: false,
        tone: "failure" as const,
      }
    : null;

  if (!presentation) {
    return null;
  }
  const Icon = presentation.icon;
  const failureCopy = presentation.failureCode
    ? FAILURE_COPY_KEYS[presentation.failureCode]
    : null;
  const warningFailure = presentation.failureCode
    ? WARNING_FAILURE_CODES.has(presentation.failureCode)
    : false;
  const canReconcile = presentation.failureCode
    ? RECONCILABLE_FAILURE_CODES.has(presentation.failureCode)
    : false;
  return (
    <div
      className={cn(
        compact
          ? "px-4 pt-1"
          : `${CONVERSATION_COMPOSER_LANE_CLASS_NAME} px-3 pt-1 sm:px-5 xl:px-6`,
      )}
      data-conversation-failure-code={presentation.failureCode ?? undefined}
      data-conversation-reliability={presentation.tone}
    >
      <div
        aria-atomic="true"
        aria-live="polite"
        className={cn(
          "flex min-h-8 w-full items-start gap-2 rounded-[9px] border border-(--surface-control-border) bg-(--surface-control-background) px-2.5 py-1.5 text-xs text-(--text-muted)",
        )}
        role="status"
      >
        <Icon
          aria-hidden="true"
          className={cn(
            "mt-0.5 h-3.5 w-3.5 shrink-0",
            presentation.tone !== "failure"
              ? "text-(--icon-muted)"
              : warningFailure
                ? "text-(--warning)"
                : "text-(--destructive)",
            presentation.spinning && "animate-spin motion-reduce:animate-none",
          )}
        />
        {failureCopy ? (
          <div className="min-w-0 flex-1">
            <span className="block font-medium leading-5 text-(--text-strong)">
              {t(failureCopy.title)}
            </span>
            <p className="mt-0.5 break-words leading-5 text-(--text-muted) [overflow-wrap:anywhere]">
              {t(failureCopy.impact)}
            </p>
          </div>
        ) : (
          <span className="min-w-0 flex-1 leading-5">{presentation.message}</span>
        )}
        {canReconcile ? (
          <button
            className="inline-flex min-h-7 shrink-0 items-center gap-1 rounded-[7px] px-2 font-medium text-(--primary) transition-colors hover:bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)] disabled:cursor-wait disabled:opacity-60 motion-reduce:transition-none"
            disabled={isReconciling}
            onClick={onReconcile}
            type="button"
          >
            <RefreshCw
              aria-hidden="true"
              className={cn(
                "h-3.5 w-3.5",
                isReconciling && "animate-spin motion-reduce:animate-none",
              )}
            />
            {t("common.refresh")}
          </button>
        ) : null}
      </div>
    </div>
  );
}
