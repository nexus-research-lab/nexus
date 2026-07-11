import {
  File,
  FileText,
  Image as ImageIcon,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

const ATTACHMENT_PRESENTATION: Record<
  MessageAttachment["kind"],
  { icon: LucideIcon; label: string }
> = {
  file: { icon: File, label: "文件" },
  image: { icon: ImageIcon, label: "图片" },
  text: { icon: FileText, label: "文本" },
};

interface MessageUserAttachmentsProps {
  attachments: MessageAttachment[];
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  workspaceAgentId?: string | null;
}

export function MessageUserAttachments({
  attachments,
  onOpenWorkspaceFile,
  workspaceAgentId,
}: MessageUserAttachmentsProps) {
  if (attachments.length === 0) {
    return null;
  }

  return (
    <div className="mt-2 flex flex-wrap justify-end gap-1.5">
      {attachments.map((attachment, index) => (
        <MessageUserAttachment
          attachment={attachment}
          key={`${attachment.workspace_path}-${index}`}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId={workspaceAgentId}
        />
      ))}
    </div>
  );
}

function MessageUserAttachment({
  attachment,
  onOpenWorkspaceFile,
  workspaceAgentId,
}: {
  attachment: MessageAttachment;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  workspaceAgentId?: string | null;
}) {
  const presentation = ATTACHMENT_PRESENTATION[attachment.kind];
  const Icon = presentation.icon;
  const canOpen = Boolean(
    onOpenWorkspaceFile &&
    attachment.workspace_path &&
    workspaceAgentId &&
    attachment.workspace_agent_id === workspaceAgentId,
  );
  const title = `${attachment.file_name || attachment.workspace_path} · ${attachment.workspace_path}`;
  const className = cn(
    "inline-flex max-w-[260px] items-center gap-1.5 rounded-[7px] border px-2.5 py-1 text-xs font-medium",
    "border-(--divider-subtle-color) bg-transparent text-(--text-muted)",
    canOpen
      ? "cursor-pointer transition-colors hover:border-(--accent-color) hover:text-(--text-strong)"
      : "cursor-default",
  );
  const content = (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="min-w-0 truncate">
        {attachment.file_name || attachment.workspace_path}
      </span>
      <span className="shrink-0 text-[10px] text-(--text-faint)">
        {presentation.label}
      </span>
    </>
  );

  if (!canOpen) {
    return <span className={className} title={title}>{content}</span>;
  }
  return (
    <button
      className={className}
      onClick={() => onOpenWorkspaceFile?.(
        attachment.workspace_path,
        attachment.workspace_agent_id ?? workspaceAgentId,
      )}
      title={title}
      type="button"
    >
      {content}
    </button>
  );
}
