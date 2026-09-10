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
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
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
    >
      <UiInlineNotice
        action={canReconcile
          ? {
              icon: <RefreshCw />,
              label: t("common.refresh"),
              onClick: onReconcile,
              pending: isReconciling,
            }
          : undefined}
        data-conversation-failure-code={presentation.failureCode ?? undefined}
        data-conversation-reliability={presentation.tone}
        icon={(
          <Icon
            className={cn(
              presentation.spinning && getUiSpinnerClassName({ size: "sm" }),
            )}
          />
        )}
        message={failureCopy
          ? t(failureCopy.impact)
          : presentation.message}
        title={failureCopy ? t(failureCopy.title) : undefined}
        tone={presentation.tone !== "failure"
          ? "neutral"
          : warningFailure
            ? "warning"
            : "danger"}
      />
    </div>
  );
}
