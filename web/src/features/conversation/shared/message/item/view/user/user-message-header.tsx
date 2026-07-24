import {
  Check,
  Copy,
  CornerDownRight,
  Edit2,
  RotateCcw,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import { MessageActionButton } from "../../../ui/message-action-button";
import type { UserMessagePresentation } from "./user-message-model";

interface UserMessageHeaderProps {
  copied: boolean;
  onCopy: () => Promise<void>;
  onEdit?: () => void;
  onRerun?: () => void;
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
  onCopy,
  onEdit,
  onRerun,
  presentation,
}: UserMessageHeaderProps) {
  return (
    <div
      className={cn(
        "nexus-chat-user-actions mt-1.5 flex items-center justify-end gap-1 text-(--text-muted) transition-[opacity,transform] duration-(--motion-duration-fast)",
        "sm:pointer-events-none sm:translate-y-0.5 sm:opacity-0 sm:group-focus-within:pointer-events-auto sm:group-focus-within:translate-y-0 sm:group-focus-within:opacity-100 sm:group-hover:pointer-events-auto sm:group-hover:translate-y-0 sm:group-hover:opacity-100",
        presentation.headerClassName,
      )}
    >
      {presentation.guided ? (
        <span className="mr-1 inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-(--text-muted)">
          <CornerDownRight className="h-3.5 w-3.5" />
          补充要求
        </span>
      ) : null}
      <span className="nexus-chat-meta shrink-0 text-xs text-(--text-muted)">
        {presentation.timestamp}
      </span>
      <UserMessageActions
        copied={copied}
        onCopy={onCopy}
        onEdit={onEdit}
        onRerun={onRerun}
      />
    </div>
  );
}

function UserMessageActions({
  copied,
  onCopy,
  onEdit,
  onRerun,
}: Pick<UserMessageHeaderProps, "copied" | "onCopy" | "onEdit" | "onRerun">) {
  const action = COPY_ACTION_PRESENTATION[copied ? "copied" : "idle"];
  const CopyIcon = action.icon;
  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {onRerun ? (
        <MessageActionButton
          aria-label="重新运行"
          onClick={onRerun}
          title="重新运行"
          tone="default"
        >
          <RotateCcw className="h-3.5 w-3.5" />
        </MessageActionButton>
      ) : null}
      {onEdit ? (
        <MessageActionButton
          aria-label="编辑消息"
          onClick={onEdit}
          title="编辑消息"
          tone="default"
        >
          <Edit2 className="h-3.5 w-3.5" />
        </MessageActionButton>
      ) : null}
      <MessageActionButton
        aria-label="复制消息"
        onClick={onCopy}
        title="复制消息"
        tone={action.tone}
      >
        <CopyIcon className="h-3.5 w-3.5" />
      </MessageActionButton>
    </div>
  );
}
