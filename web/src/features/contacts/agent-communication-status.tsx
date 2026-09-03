// INPUT: 联络资源的空/加载状态，或服务端已分类的读取失败事实。
// OUTPUT: 使用共享 ResourceState 呈现的一致状态与单一恢复动作。
// POS: Contacts 联络状态投影；不推断失败原因或执行读取命令。

import { RefreshCw, type LucideIcon } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { AgentCommunicationReadFailure } from "@/types/agent/communication";

export function AgentCommunicationEmptyState({
  icon: Icon,
  label,
  loading = false,
}: {
  icon?: LucideIcon;
  label: string;
  loading?: boolean;
}) {
  return (
    <UiResourceState
      className="h-full min-h-44"
      icon={!loading && Icon ? (
        <Icon className="h-5 w-5 text-(--icon-default)" />
      ) : undefined}
      size="sm"
      state={loading ? "loading" : "empty"}
      title={label}
      variant="plain"
    />
  );
}

export function AgentCommunicationReadFailureState({
  compact = false,
  failure,
  onRetry,
}: {
  compact?: boolean;
  failure: AgentCommunicationReadFailure;
  onRetry: () => void;
}) {
  const { t } = useI18n();
  const copy = communicationFailureCopy(failure);
  return (
    <UiResourceState
      className={cn(compact && "mb-2 min-h-0 py-3")}
      impact={t(copy.impact)}
      primaryAction={{
        icon: <RefreshCw className="h-3.5 w-3.5" />,
        label: t(copy.action),
        onClick: onRetry,
      }}
      size="sm"
      state="error"
      title={t(copy.title)}
      urgency="polite"
    />
  );
}

function communicationFailureCopy(failure: AgentCommunicationReadFailure) {
  switch (failure.kind) {
    case "directory":
      return {
        action: "agent_options.contact.retry_directory" as const,
        impact: failure.stale
          ? "agent_options.contact.directory_stale_impact" as const
          : "agent_options.contact.directory_unavailable_impact" as const,
        title: "agent_options.contact.directory_load_failed" as const,
      };
    case "channel":
      return {
        action: "agent_options.contact.retry_channel" as const,
        impact: failure.stale
          ? "agent_options.contact.channel_stale_impact" as const
          : "agent_options.contact.channel_unavailable_impact" as const,
        title: "agent_options.contact.channel_load_failed" as const,
      };
    case "history":
      return {
        action: "agent_options.contact.retry_history" as const,
        impact: failure.stale
          ? "agent_options.contact.history_stale_impact" as const
          : "agent_options.contact.history_unavailable_impact" as const,
        title: "agent_options.contact.history_load_failed" as const,
      };
    case "messages":
      return {
        action: "agent_options.contact.retry_messages" as const,
        impact: failure.stale
          ? "agent_options.contact.messages_stale_impact" as const
          : "agent_options.contact.messages_unavailable_impact" as const,
        title: "agent_options.contact.messages_load_failed" as const,
      };
  }
}
