// INPUT: User attachments and the current exact workspace Agent scope.
// OUTPUT: Shared attachment actions or static badges without cross-workspace opening.
// POS: User message attachment projection; callbacks retain workspace ownership.
import {
  File,
  FileText,
  Image as ImageIcon,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
      <span className={`shrink-0 ${getUiTypographyClassName({ role: "caption", tone: "soft" })}`}>
        {t(presentation.labelKey)}
      </span>
    </>
  );

  if (!attachmentView.canOpen) {
    return (
      <UiBadge className="max-w-[260px] min-w-0 gap-1.5" size="sm" title={attachmentView.title}>
        {content}
      </UiBadge>
    );
  }
  return (
    <UiButton
      size="xs"
      variant="outline"
      className="max-w-[260px] min-w-0 gap-1.5"
      onClick={() => openMessageUserAttachment(
        attachment,
        onOpenWorkspaceFile,
        workspaceAgentId,
      )}
      title={attachmentView.title}
      type="button"
    >
      {content}
    </UiButton>
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
    displayName,
    title: `${displayName} · ${attachment.workspace_path}`,
  };
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
