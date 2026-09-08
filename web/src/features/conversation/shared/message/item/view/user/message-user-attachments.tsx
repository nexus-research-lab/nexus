import {
  File,
  FileText,
  Image as ImageIcon,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

const ATTACHMENT_PRESENTATION: Record<
  MessageAttachment["kind"],
  { icon: LucideIcon; labelKey: TranslationKey }
> = {
  file: { icon: File, labelKey: "message.attachment_file" },
  image: { icon: ImageIcon, labelKey: "message.attachment_image" },
  text: { icon: FileText, labelKey: "message.attachment_text" },
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
  const { t } = useI18n();
  const presentation = ATTACHMENT_PRESENTATION[attachment.kind];
  const Icon = presentation.icon;
  const attachmentView = projectMessageUserAttachment(
    attachment,
    Boolean(onOpenWorkspaceFile),
    workspaceAgentId,
  );
  const content = (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="min-w-0 truncate">
        {attachmentView.displayName}
      </span>
      <span className="shrink-0 text-2xs text-(--text-soft)">
        {t(presentation.labelKey)}
      </span>
    </>
  );

  if (!attachmentView.canOpen) {
    return (
      <span className={attachmentView.className} title={attachmentView.title}>
        {content}
      </span>
    );
  }
  return (
    <button
      className={attachmentView.className}
      onClick={() => openMessageUserAttachment(
        attachment,
        onOpenWorkspaceFile,
        workspaceAgentId,
      )}
      title={attachmentView.title}
      type="button"
    >
      {content}
    </button>
  );
}

function projectMessageUserAttachment(
  attachment: MessageAttachment,
  hasOpenHandler: boolean,
  workspaceAgentId?: string | null,
) {
  const displayName = attachment.file_name || attachment.workspace_path;
  const canOpen = [
    hasOpenHandler,
    Boolean(attachment.workspace_path),
    Boolean(workspaceAgentId),
    attachment.workspace_agent_id === workspaceAgentId,
  ].every(Boolean);
  return {
    canOpen,
    className: resolveAttachmentClassName(canOpen),
    displayName,
    title: `${displayName} · ${attachment.workspace_path}`,
  };
}

function resolveAttachmentClassName(canOpen: boolean): string {
  return cn(
    "inline-flex max-w-[260px] items-center radius-control-sm gap-1.5 border px-2.5 py-1 text-xs font-medium",
    "border-(--divider-subtle-color) bg-transparent text-(--text-muted)",
    canOpen
      ? "cursor-pointer transition-colors hover:border-(--surface-interactive-hover-border) hover:text-(--text-strong)"
      : "cursor-default",
  );
}

function openMessageUserAttachment(
  attachment: MessageAttachment,
  onOpenWorkspaceFile:
    | ((path: string, workspaceAgentId?: string | null) => void)
    | undefined,
  workspaceAgentId?: string | null,
): void {
  if (!onOpenWorkspaceFile) {
    return;
  }
  onOpenWorkspaceFile(
    attachment.workspace_path,
    attachment.workspace_agent_id ?? workspaceAgentId,
  );
}
