/**
 * INPUT: Conversation reliability 的 transport、provider retry 与用户级失败分类。
 * OUTPUT: 与 Composer 输入框内缘对齐、不含技术详情、可随恢复证据自动消失的紧凑状态卡。
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

const FAILURE_MESSAGE_KEYS: Record<ConversationFailureCode, TranslationKey> = {
  connection_unavailable: "conversation.reliability.connection_unavailable",
  delivery_unknown: "conversation.reliability.delivery_unknown",
  permission_not_sent: "conversation.reliability.permission_not_sent",
  provider_configuration: "conversation.reliability.provider_configuration",
  provider_unavailable: "conversation.reliability.provider_unavailable",
  request_rejected: "conversation.reliability.request_rejected",
  round_failed: "conversation.reliability.round_failed",
  safety_rejected: "conversation.reliability.safety_rejected",
  session_load_failed: "conversation.reliability.session_load_failed",
  usage_limited: "conversation.reliability.usage_limited",
  validation_failed: "conversation.reliability.validation_failed",
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
        message: t("conversation.reliability.connection_unavailable"),
        spinning: false,
        tone: "failure" as const,
        failureCode: "connection_unavailable" as const,
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
        message: t(FAILURE_MESSAGE_KEYS[reliability.failure.code]),
        spinning: false,
        tone: "failure" as const,
        failureCode: reliability.failure.code,
      }
    : null;

  if (!presentation) {
    return null;
  }
  const Icon = presentation.icon;
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
        aria-live={presentation.tone === "failure" ? "assertive" : "polite"}
        className={cn(
          "flex min-h-9 w-full items-center gap-2 rounded-[10px] border px-2.5 py-1.5 text-xs shadow-[0_1px_2px_color-mix(in_srgb,var(--shadow-color)_6%,transparent)]",
        presentation.tone === "failure"
          ? "border-[color:color-mix(in_srgb,var(--destructive)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_5%,var(--surface-control-background))] text-(--destructive)"
          : "border-(--surface-control-border) bg-(--surface-control-background) text-(--text-muted)",
        )}
        role={presentation.tone === "failure" ? "alert" : "status"}
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
        <span className="min-w-0 flex-1 leading-5">{presentation.message}</span>
      </div>
    </div>
  );
}
