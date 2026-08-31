/**
 * INPUT: Conversation reliability 的 transport、provider retry 与用户级失败分类。
 * OUTPUT: 与 Composer 输入框内缘对齐、不含技术详情、完整说明问题/影响/下一步的紧凑状态卡。
 * POS: DM 与 Room 共用的可靠性状态展示；不进入消息 Feed。
 */
"use client";

import { LoaderCircle, TriangleAlert, WifiOff } from "lucide-react";

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
  nextStep: TranslationKey;
  title: TranslationKey;
}

const FAILURE_COPY_KEYS: Record<ConversationFailureCode, ConversationFailureCopyKeys> = {
  connection_unavailable: {
    impact: "conversation.reliability.connection_unavailable_impact",
    nextStep: "conversation.reliability.connection_unavailable_next_step",
    title: "conversation.reliability.connection_unavailable",
  },
  delivery_unknown: {
    impact: "conversation.reliability.delivery_unknown_impact",
    nextStep: "conversation.reliability.delivery_unknown_next_step",
    title: "conversation.reliability.delivery_unknown",
  },
  permission_not_sent: {
    impact: "conversation.reliability.permission_not_sent_impact",
    nextStep: "conversation.reliability.permission_not_sent_next_step",
    title: "conversation.reliability.permission_not_sent",
  },
  provider_configuration: {
    impact: "conversation.reliability.provider_configuration_impact",
    nextStep: "conversation.reliability.provider_configuration_next_step",
    title: "conversation.reliability.provider_configuration",
  },
  provider_unavailable: {
    impact: "conversation.reliability.provider_unavailable_impact",
    nextStep: "conversation.reliability.provider_unavailable_next_step",
    title: "conversation.reliability.provider_unavailable",
  },
  request_rejected: {
    impact: "conversation.reliability.request_rejected_impact",
    nextStep: "conversation.reliability.request_rejected_next_step",
    title: "conversation.reliability.request_rejected",
  },
  round_failed: {
    impact: "conversation.reliability.round_failed_impact",
    nextStep: "conversation.reliability.round_failed_next_step",
    title: "conversation.reliability.round_failed",
  },
  safety_rejected: {
    impact: "conversation.reliability.safety_rejected_impact",
    nextStep: "conversation.reliability.safety_rejected_next_step",
    title: "conversation.reliability.safety_rejected",
  },
  session_load_failed: {
    impact: "conversation.reliability.session_load_failed_impact",
    nextStep: "conversation.reliability.session_load_failed_next_step",
    title: "conversation.reliability.session_load_failed",
  },
  usage_limited: {
    impact: "conversation.reliability.usage_limited_impact",
    nextStep: "conversation.reliability.usage_limited_next_step",
    title: "conversation.reliability.usage_limited",
  },
  validation_failed: {
    impact: "conversation.reliability.validation_failed_impact",
    nextStep: "conversation.reliability.validation_failed_next_step",
    title: "conversation.reliability.validation_failed",
  },
};

export function ConversationReliabilityNotice({
  compact,
  reliability,
}: {
  compact: boolean;
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
          "flex min-h-9 w-full items-center gap-2 rounded-[10px] border px-2.5 py-1.5 text-xs shadow-[0_1px_2px_color-mix(in_srgb,var(--shadow-color)_6%,transparent)]",
        presentation.tone === "failure"
          ? "border-[color:color-mix(in_srgb,var(--destructive)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_5%,var(--surface-control-background))] text-(--destructive)"
          : "border-(--surface-control-border) bg-(--surface-control-background) text-(--text-muted)",
        )}
        role="status"
      >
        <span
          aria-hidden="true"
          className={cn(
            "flex h-6 w-6 shrink-0 items-center justify-center rounded-[7px]",
            presentation.tone === "failure"
              ? "bg-[color:color-mix(in_srgb,var(--destructive)_9%,transparent)]"
              : "bg-(--surface-control-field-background)",
          )}
        >
          <Icon
            className={cn("h-3.5 w-3.5", presentation.spinning && "animate-spin")}
          />
        </span>
        {failureCopy ? (
          <span className="min-w-0 flex-1 space-y-0.5 leading-5">
            <span className="block font-semibold text-(--text-strong)">
              {t(failureCopy.title)}
            </span>
            <span className="block text-(--text-muted)">
              {t(failureCopy.impact)}
            </span>
            <span className="block font-medium text-(--text-default)">
              {t(failureCopy.nextStep)}
            </span>
          </span>
        ) : (
          <span className="min-w-0 flex-1 leading-5">{presentation.message}</span>
        )}
      </div>
    </div>
  );
}
