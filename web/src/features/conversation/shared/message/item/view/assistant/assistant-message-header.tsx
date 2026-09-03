/**
 * INPUT: Assistant 身份、结算元数据与宿主 public handoff reply 来源。
 * OUTPUT: 消息头部、不可点击的“回应 @Agent”标注与可选停止动作。
 * POS: Assistant 消息头视图，不解析正文 mention 也不发起通讯。
 */
import type { ReactNode } from "react";
import { Bot, Clock3, RadioTower, Square } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

import {
  AgentHandoffReplyChip,
  type AgentMentionDirectory,
} from "../../../agent-mention-chip";
import { formatMessageTime } from "../../../message-time";
import { MessageActionButton } from "../../../ui/message-action-button";
import { MessageAvatar } from "../../../ui/message-avatar";

interface AssistantMessageHeaderProps {
  avatarUrl?: string | null;
  automationTaskName?: string | null;
  canStop: boolean;
  compact: boolean;
  echo: boolean;
  agentMentionDirectory?: AgentMentionDirectory;
  headerAction?: ReactNode;
  handoffReplySourceAgentId?: string | null;
  model?: string;
  name?: string | null;
  onOpenContact?: () => void;
  onStop: () => void;
  showMetadata: boolean;
  timestamp?: number;
}

const HEADER_LAYOUTS = {
  compact: "min-h-6 pb-0",
  expanded: "min-h-8 pb-0",
} as const;

export function AssistantMessageHeader({
  avatarUrl,
  automationTaskName,
  canStop,
  compact,
  echo,
  agentMentionDirectory,
  headerAction,
  handoffReplySourceAgentId,
  model,
  name,
  onOpenContact,
  onStop,
  showMetadata,
  timestamp,
}: AssistantMessageHeaderProps) {
  const { t } = useI18n();
  const displayName = name || t("message.assistant_fallback");
  const layout = HEADER_LAYOUTS[compact ? "compact" : "expanded"];
  return (
    <div
      className={cn(
        "nexus-chat-message-header flex min-w-0 items-center gap-2",
        layout,
      )}
    >
      <AssistantHeaderAvatar
        avatarUrl={avatarUrl}
        compact={compact}
        displayName={displayName}
        onOpenContact={onOpenContact}
      />
      <span className="nexus-chat-author shrink-0 text-sm font-medium text-(--text-strong)">
        {displayName}
      </span>
      <AssistantAutomationBadge taskName={automationTaskName} />
      <AssistantEchoBadge visible={echo} />
      {handoffReplySourceAgentId ? (
        <AgentHandoffReplyChip
          agentId={handoffReplySourceAgentId}
          directory={agentMentionDirectory}
        />
      ) : null}
      {showMetadata ? (
        <>
          <AssistantTimestamp timestamp={timestamp} />
          <AssistantModel model={model} />
        </>
      ) : null}
      <div className="flex-1" />
      <AssistantHeaderAction action={headerAction} />
      <AssistantStopAction canStop={canStop} onStop={onStop} />
    </div>
  );
}

function AssistantEchoBadge({ visible }: { visible: boolean }) {
  const { t } = useI18n();
  if (!visible) {
    return null;
  }
  return (
    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-(--divider-subtle-color) bg-(--surface-control-field-background) px-1.5 py-0.5 text-2xs font-medium leading-none text-(--text-muted)">
      <RadioTower className="h-2.5 w-2.5" />
      {t("message.echo")}
    </span>
  );
}

function AssistantAutomationBadge({ taskName }: { taskName?: string | null }) {
  const { t } = useI18n();
  if (taskName == null) {
    return null;
  }
  return (
    <span
      className="inline-flex shrink-0 items-center gap-1 rounded-full border border-(--divider-subtle-color) bg-(--surface-control-field-background) px-1.5 py-0.5 text-2xs font-medium leading-none text-(--text-muted)"
      title={taskName || t("message.scheduled_task")}
    >
      <Clock3 className="h-2.5 w-2.5" />
      {t("message.scheduled_task")}
    </span>
  );
}

function AssistantHeaderAvatar({
  avatarUrl,
  compact,
  displayName,
  onOpenContact,
}: {
  avatarUrl?: string | null;
  compact: boolean;
  displayName: string;
  onOpenContact?: () => void;
}) {
  return (
    <AssistantMessageAvatar
      avatarUrl={avatarUrl}
      compact={compact}
      displayName={displayName}
      onOpenContact={onOpenContact}
    />
  );
}

function AssistantTimestamp({ timestamp }: { timestamp?: number }) {
  if (!timestamp) {
    return null;
  }
  return (
    <span className="nexus-chat-meta hidden shrink-0 text-xs text-(--text-muted) sm:inline">
      {formatMessageTime(timestamp)}
    </span>
  );
}

function AssistantModel({ model }: { model?: string }) {
  if (!model) {
    return null;
  }
  return (
    <span className="nexus-chat-meta min-w-0 truncate text-xs text-(--text-soft)">
      {model}
    </span>
  );
}

function AssistantHeaderAction({ action }: { action?: ReactNode }) {
  if (!action) {
    return null;
  }
  return <div className="shrink-0">{action}</div>;
}

function AssistantStopAction({
  canStop,
  onStop,
}: {
  canStop: boolean;
  onStop: () => void;
}) {
  const { t } = useI18n();
  if (!canStop) {
    return null;
  }
  return (
    <MessageActionButton
      aria-label={t("composer.stop_generation")}
      className="flex items-center gap-1 px-1.5 py-0.5 text-xs"
      onClick={onStop}
      tone="default"
      type="button"
    >
      <Square className="h-3 w-3 fill-current" />
      <span>{t("composer.stop_generation")}</span>
    </MessageActionButton>
  );
}

const AVATAR_PRESENTATION = {
  compact: {
    bot: "h-3 w-3",
    className: "nexus-chat-avatar shrink-0",
    size: "compact",
  },
  full: {
    bot: "h-4 w-4",
    className: "nexus-chat-avatar h-8 w-8 shrink-0",
    size: "full",
  },
} as const;

function AssistantMessageAvatar({
  avatarUrl,
  compact = false,
  displayName,
  onOpenContact,
}: {
  avatarUrl?: string | null;
  compact?: boolean;
  displayName: string;
  onOpenContact?: () => void;
}) {
  const { t } = useI18n();
  const presentation = AVATAR_PRESENTATION[compact ? "compact" : "full"];
  return (
    <MessageAvatar
      ariaLabel={t("room.agent_contact_open", { name: displayName })}
      avatarUrl={avatarUrl}
      className={presentation.className}
      onClick={onOpenContact}
      radius="control"
      size={presentation.size}
      title={t("room.agent_contact_open", { name: displayName })}
    >
      <AssistantAvatarFallback
        avatarUrl={avatarUrl}
        className={presentation.bot}
      />
    </MessageAvatar>
  );
}

function AssistantAvatarFallback({
  avatarUrl,
  className,
}: {
  avatarUrl?: string | null;
  className: string;
}) {
  if (avatarUrl) {
    return null;
  }
  return <Bot className={className} />;
}
