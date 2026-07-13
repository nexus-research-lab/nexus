import type { ReactNode } from "react";
import { Bot, Square } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import { formatMessageTime } from "../../../message-time";
import { MessageActionButton } from "../../../ui/message-action-button";
import { MessageAvatar } from "../../../ui/message-avatar";

interface AssistantMessageHeaderProps {
  avatarUrl?: string | null;
  canStop: boolean;
  compact: boolean;
  headerAction?: ReactNode;
  model?: string;
  name?: string | null;
  onOpenContact?: () => void;
  onStop: () => void;
  timestamp?: number;
}

const HEADER_LAYOUTS = {
  compact: "min-h-6 pb-0",
  expanded: "h-7 pb-0.5",
} as const;

export function AssistantMessageHeader({
  avatarUrl,
  canStop,
  compact,
  headerAction,
  model,
  name,
  onOpenContact,
  onStop,
  timestamp,
}: AssistantMessageHeaderProps) {
  const displayName = name || "协作成员";
  const layout = HEADER_LAYOUTS[compact ? "compact" : "expanded"];
  return (
    <div
      className={cn(
        "nexus-chat-message-header flex min-w-0 items-center gap-2",
        layout,
      )}
    >
      <CompactAssistantAvatar
        avatarUrl={avatarUrl}
        displayName={displayName}
        onOpenContact={onOpenContact}
        visible={compact}
      />
      <span className="nexus-chat-author shrink-0 text-sm font-bold text-(--text-strong)">
        {displayName}
      </span>
      <AssistantTimestamp timestamp={timestamp} />
      <AssistantModel model={model} />
      <div className="flex-1" />
      <AssistantHeaderAction action={headerAction} />
      <AssistantStopAction canStop={canStop} onStop={onStop} />
    </div>
  );
}

function CompactAssistantAvatar({
  avatarUrl,
  displayName,
  onOpenContact,
  visible,
}: {
  avatarUrl?: string | null;
  displayName: string;
  onOpenContact?: () => void;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return (
    <AssistantMessageAvatar
      avatarUrl={avatarUrl}
      compact
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
  if (!canStop) {
    return null;
  }
  return (
    <MessageActionButton
      aria-label="停止生成"
      className="flex items-center gap-1 px-1.5 py-0.5 text-xs"
      onClick={onStop}
      tone="default"
      type="button"
    >
      <Square className="h-3 w-3 fill-current" />
      <span>停止</span>
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
    className: "nexus-chat-avatar",
    size: "full",
  },
} as const;

export function AssistantMessageAvatar({
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
  const presentation = AVATAR_PRESENTATION[compact ? "compact" : "full"];
  return (
    <MessageAvatar
      ariaLabel={`打开 ${displayName} 的联络`}
      avatarUrl={avatarUrl}
      className={presentation.className}
      onClick={onOpenContact}
      size={presentation.size}
      title={`打开 ${displayName} 的联络`}
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
