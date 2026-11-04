/**
 * INPUT: Assistant 身份、结算元数据与宿主 public handoff reply 来源。
 * OUTPUT: 消息头部（DM 可省略头像与名称）、不可点击的“回应 @Agent”标注与可选停止动作。
 * POS: Assistant 消息头视图，不解析正文 mention 也不发起通讯。
 */
import type { ReactNode } from "react";
import { Bot, Clock3, RadioTower, Square } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";

import {
  AgentHandoffReplyChip,
  MessageReplyChip,
  type AgentMentionDirectory,
} from "../../../agent-mention-chip";
import { formatMessageTime } from "../../../message-time";
import { MessageAuthorAvatar } from "../../../ui/message-avatar";

interface AssistantMessageHeaderProps {
  avatarUrl?: string | null;
  automationTaskName?: string | null;
  canStop: boolean;
  echo: boolean;
  agentMentionDirectory?: AgentMentionDirectory;
  headerAction?: ReactNode;
  replyTarget?: {name: string; avatar?: string | null; message?: string};
  handoffReplySourceAgentId?: string | null;
  model?: string;
  name?: string | null;
  onOpenContact?: () => void;
  onStop: () => void;
  showIdentity: boolean;
  showMetadata: boolean;
  timestamp?: number;
}

export function AssistantMessageHeader({
  avatarUrl,
  automationTaskName,
  canStop,
  echo,
  agentMentionDirectory,
  headerAction,
  replyTarget,
  handoffReplySourceAgentId,
  model,
  name,
  onOpenContact,
  onStop,
  showIdentity,
  showMetadata,
  timestamp,
}: AssistantMessageHeaderProps) {
  const { t } = useI18n();
  const displayName = name || t("message.assistant_fallback");
  return (
    <div
      className={cn(
        "nexus-chat-message-header flex min-h-6 min-w-0 items-center gap-2 pb-0",
      )}
    >
      {showIdentity ? (
        <>
          <AssistantMessageAvatar
            avatarUrl={avatarUrl}
            displayName={displayName}
            onOpenContact={onOpenContact}
          />
          <span className="nexus-chat-author shrink-0 text-sm font-medium text-(--text-strong)">
            {displayName}
          </span>
        </>
      ) : null}
      <AssistantAutomationBadge taskName={automationTaskName} />
      <AssistantEchoBadge visible={echo} />
      {replyTarget ? <MessageReplyChip {...replyTarget} /> : handoffReplySourceAgentId ? (
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
    <UiBadge shape="pill" size="xs">
      <RadioTower className="h-2.5 w-2.5" />
      {t("message.echo")}
    </UiBadge>
  );
}

function AssistantAutomationBadge({ taskName }: { taskName?: string | null }) {
  const { t } = useI18n();
  if (taskName == null) {
    return null;
  }
  return (
    <UiBadge
      shape="pill"
      size="xs"
      title={taskName || t("message.scheduled_task")}
    >
      <Clock3 className="h-2.5 w-2.5" />
      {t("message.scheduled_task")}
    </UiBadge>
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
    <UiButton
      aria-label={t("composer.stop_generation")}
      onClick={onStop}
      size="2xs"
      tone="danger"
      variant="text"
    >
      <Square className="h-3 w-3 fill-current" />
      <span>{t("composer.stop_generation")}</span>
    </UiButton>
  );
}

function AssistantMessageAvatar({
  avatarUrl,
  displayName,
  onOpenContact,
}: {
  avatarUrl?: string | null;
  displayName: string;
  onOpenContact?: () => void;
}) {
  const { t } = useI18n();
  return (
    <MessageAuthorAvatar
      ariaLabel={t("room.agent_contact_open", { name: displayName })}
      avatarUrl={avatarUrl}
      onClick={onOpenContact}
      title={t("room.agent_contact_open", { name: displayName })}
    >
      <Bot className="h-3.5 w-3.5" />
    </MessageAuthorAvatar>
  );
}
