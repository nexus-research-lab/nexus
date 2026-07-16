import {
  Check,
  Copy,
  CornerDownRight,
  Edit2,
  type LucideIcon,
  User,
} from "lucide-react";

import { MessageActionButton } from "../../../ui/message-action-button";
import { MessageAvatar } from "../../../ui/message-avatar";
import type { UserMessagePresentation } from "./user-message-model";

interface UserMessageHeaderProps {
  copied: boolean;
  currentUserAvatar?: string | null;
  onCopy: () => Promise<void>;
  onEdit?: () => void;
  presentation: UserMessagePresentation;
}

interface CopyActionPresentation {
  icon: LucideIcon;
  tone: "default" | "success";
}

const COPY_ACTION_PRESENTATION: Record<"copied" | "idle", CopyActionPresentation> = {
  copied: { icon: Check, tone: "success" },
  idle: { icon: Copy, tone: "default" },
};

export function UserMessageHeader({
  copied,
  currentUserAvatar,
  onCopy,
  onEdit,
  presentation,
}: UserMessageHeaderProps) {
  return (
    <div className={`nexus-chat-message-header flex items-center justify-end gap-2 ${presentation.headerClassName}`}>
      <UserMessageActions copied={copied} onCopy={onCopy} onEdit={onEdit} />
      <span className="nexus-chat-meta hidden shrink-0 text-xs text-(--text-muted) sm:inline">
        {presentation.timestamp}
      </span>
      <UserMessageIdentity
        currentUserAvatar={currentUserAvatar}
        presentation={presentation}
      />
    </div>
  );
}

function UserMessageActions({
  copied,
  onCopy,
  onEdit,
}: Pick<UserMessageHeaderProps, "copied" | "onCopy" | "onEdit">) {
  const action = COPY_ACTION_PRESENTATION[copied ? "copied" : "idle"];
  const CopyIcon = action.icon;
  return (
    <div className="shrink-0 opacity-100 transition-opacity duration-(--motion-duration-fast) sm:opacity-0 sm:group-hover:opacity-100">
      {onEdit ? (
        <MessageActionButton
          aria-label="编辑消息"
          onClick={onEdit}
          tone="default"
        >
          <Edit2 className="h-3 w-3" />
        </MessageActionButton>
      ) : null}
      <MessageActionButton
        aria-label="复制消息"
        onClick={onCopy}
        tone={action.tone}
      >
        <CopyIcon className="h-3 w-3" />
      </MessageActionButton>
    </div>
  );
}

function UserMessageIdentity({
  currentUserAvatar,
  presentation,
}: Pick<UserMessageHeaderProps, "currentUserAvatar" | "presentation">) {
  return (
    <>
      {presentation.guided ? (
        <span className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-(--text-muted)">
          <CornerDownRight className="h-3.5 w-3.5" />
          补充要求
        </span>
      ) : null}
      <span className="nexus-chat-author shrink-0 text-sm font-medium text-(--text-strong)">
        你
      </span>
      <MessageAvatar
        avatarUrl={currentUserAvatar}
        className="nexus-chat-avatar shrink-0"
        size={presentation.avatarSize}
      >
        <User className={presentation.avatarFallbackClassName} />
      </MessageAvatar>
    </>
  );
}
