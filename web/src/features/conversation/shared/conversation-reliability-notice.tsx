/**
 * INPUT: Conversation reliability 的 transport、provider retry 与用户级失败分类。
 * OUTPUT: Composer 上方不含技术详情、可随恢复证据自动消失的紧凑状态提示。
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
      }
    : reliability.transport_phase === "unavailable"
    ? {
        icon: WifiOff,
        message: t("conversation.reliability.connection_unavailable"),
        spinning: false,
        tone: "failure" as const,
      }
    : reliability.provider_retry
    ? {
        icon: LoaderCircle,
        message: t("conversation.reliability.provider_retrying"),
        spinning: true,
        tone: "recovering" as const,
      }
    : reliability.failure
    ? {
        icon: TriangleAlert,
        message: t(FAILURE_MESSAGE_KEYS[reliability.failure.code]),
        spinning: false,
        tone: "failure" as const,
      }
    : null;

  if (!presentation) {
    return null;
  }
  const Icon = presentation.icon;
  return (
    <div
      aria-live="polite"
      className={cn(
        "mx-auto flex min-h-8 w-full items-center gap-2 border-y px-3 py-1.5 text-xs",
        compact ? "max-w-full" : "max-w-[880px] sm:px-5 xl:px-6",
        presentation.tone === "failure"
          ? "border-[color:color-mix(in_srgb,var(--destructive)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_6%,transparent)] text-(--destructive)"
          : "border-(--surface-control-border) bg-(--surface-control-background) text-(--text-muted)",
      )}
      data-conversation-reliability={presentation.tone}
      role={presentation.tone === "failure" ? "status" : undefined}
    >
      <Icon
        aria-hidden="true"
        className={cn("h-3.5 w-3.5 shrink-0", presentation.spinning && "animate-spin")}
      />
      <span>{presentation.message}</span>
    </div>
  );
}
