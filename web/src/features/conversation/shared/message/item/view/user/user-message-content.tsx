import { cn } from "@/shared/ui/class-name";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

import { ContentRenderer } from "../content/content-renderer";
import { MessageUserAttachments } from "./message-user-attachments";
import type { UserMessagePresentation } from "./user-message-model";

interface UserMessageContentProps {
  attachments: MessageAttachment[];
  content: string;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  presentation: UserMessagePresentation;
  workspaceAgentId?: string | null;
}

export function UserMessageContent({
  attachments,
  content,
  onOpenWorkspaceFile,
  presentation,
  workspaceAgentId,
}: UserMessageContentProps) {
  return (
    <div className="nexus-chat-user-content-shell ml-auto flex w-fit max-w-full flex-col items-end rounded-2xl px-4 py-3">
      {presentation.hasContent ? (
        <ContentRenderer
          className={cn(
            "nexus-chat-user-content w-fit max-w-[min(100%,760px)] self-end break-words text-left text-(--text-strong)",
            presentation.contentClassName,
          )}
          content={content}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId={workspaceAgentId}
        />
      ) : null}
      <MessageUserAttachments
        attachments={attachments}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        workspaceAgentId={workspaceAgentId}
      />
    </div>
  );
}
