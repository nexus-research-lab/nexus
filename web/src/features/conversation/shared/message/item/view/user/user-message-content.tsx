import { cn } from "@/shared/ui/class-name";
import { Target } from "lucide-react";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { MessageAttachment } from "@/types/conversation/message/attachment";
import type { AgentMention } from "@/types/conversation/message/entity";

import { ContentRenderer } from "../content/content-renderer";
import { MessageUserAttachments } from "./message-user-attachments";
import type { UserMessagePresentation } from "./user-message-model";
import type { AgentMentionDirectory } from "../../../agent-mention-chip";

interface UserMessageContentProps {
  attachments: MessageAttachment[];
  agentMentions?: AgentMention[];
  agentMentionDirectory?: AgentMentionDirectory;
  content: string;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  presentation: UserMessagePresentation;
  workspaceAgentId?: string | null;
}

export function UserMessageContent({
  attachments,
  agentMentions,
  agentMentionDirectory,
  content,
  onOpenAgentContact,
  onOpenWorkspaceFile,
  presentation,
  workspaceAgentId,
}: UserMessageContentProps) {
  const { t } = useI18n();
  return (
    <div
      className="nexus-chat-user-content-shell ml-auto flex w-fit max-w-full flex-col items-end rounded-[12px] bg-(--surface-message-user-background) px-3.5 py-2.5"
      data-goal-control={String(presentation.goal)}
    >
      {presentation.goal ? (
        <span className="mb-1.5 inline-flex items-center gap-1 self-start text-xs font-semibold text-(--text-muted)">
          <Target className="h-3.5 w-3.5" />
          {t("composer.goal_mode")}
        </span>
      ) : null}
      {presentation.hasContent ? (
        <ContentRenderer
          className={cn(
            "nexus-chat-user-content w-fit max-w-[min(100%,760px)] self-end break-words text-left text-(--text-strong)",
            presentation.contentClassName,
          )}
          content={content}
          agentMentions={agentMentions}
          agentMentionDirectory={agentMentionDirectory}
          onOpenAgentContact={onOpenAgentContact}
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
