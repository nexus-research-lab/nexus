import type { ReactNode } from "react";
import { Bot, Square } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import {
  MessageActionButton,
  MessageAvatar,
} from "../../../ui/message-primitives";
import { formatMessageTime } from "../../../message-time";

interface AssistantMessageHeaderProps {
  avatarUrl?: string | null;
  canOpenContact: boolean;
  canStop: boolean;
  compact: boolean;
  headerAction?: ReactNode;
  model?: string;
  name?: string | null;
  onOpenContact: () => void;
  onStop: () => void;
  timestamp?: number;
}

export function AssistantMessageHeader({
  avatarUrl,
  canOpenContact,
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
  const openContact = canOpenContact ? onOpenContact : undefined;
  return (
    <div
      className={cn(
        "nexus-chat-message-header flex min-w-0 items-center gap-2",
        compact ? "min-h-6 pb-0" : "h-7 pb-0.5",
      )}
    >
      {compact ? (
        <AssistantMessageAvatar
          avatarUrl={avatarUrl}
          compact
          displayName={displayName}
          onOpenContact={openContact}
        />
      ) : null}
          <span className="nexus-chat-author shrink-0 text-sm font-bold text-(--text-strong)">
            {displayName}
          </span>
          {timestamp ? (
            <span className="nexus-chat-meta hidden shrink-0 text-xs text-(--text-muted) sm:inline">
              {formatMessageTime(timestamp)}
            </span>
          ) : null}
          {model ? (
            <span className="nexus-chat-meta min-w-0 truncate text-xs text-(--text-soft)">
              {model}
            </span>
          ) : null}
          <div className="flex-1" />
          {headerAction ? <div className="shrink-0">{headerAction}</div> : null}
          {canStop ? (
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
          ) : null}
    </div>
  );
}

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
  return (
    <MessageAvatar
      ariaLabel={`打开 ${displayName} 的联络`}
      avatarUrl={avatarUrl}
      className={cn("nexus-chat-avatar", compact && "shrink-0")}
      onClick={onOpenContact}
      size={compact ? "compact" : "full"}
      title={`打开 ${displayName} 的联络`}
    >
      {!avatarUrl && <Bot className={compact ? "h-3 w-3" : "h-4 w-4"} />}
    </MessageAvatar>
  );
}
