import { useCallback } from "react";

import { cn } from "@/shared/ui/class-name";
import type { MessageAttachment } from "@/types/conversation/message/attachment";
import type { UserMessage } from "@/types/conversation/message/entity";

import { UserMessageContent } from "./user-message-content";
import { UserMessageEditor } from "./user-message-editor";
import { UserMessageHeader } from "./user-message-header";
import {
  projectAvailableUserMessageAction,
  projectUserMessagePresentation,
} from "./user-message-model";
import { useUserMessageEditor } from "./use-user-message-editor";

interface MessageUserState {
  attachments: MessageAttachment[];
  content: string;
  copied: boolean;
  copy: () => Promise<void>;
  message: UserMessage | undefined;
}

interface MessageUserSectionProps {
  compact: boolean;
  currentUserAvatar?: string | null;
  onEditUserMessage?: (messageId: string, newContent: string) => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  user: MessageUserState;
  workspaceAgentId?: string | null;
}

export function MessageUserSection(props: MessageUserSectionProps) {
  const message = props.user.message;
  if (!message) {
    return null;
  }
  return <MessageUserSectionContent {...props} message={message} />;
}

function MessageUserSectionContent({
  compact,
  currentUserAvatar,
  message,
  onEditUserMessage,
  onOpenWorkspaceFile,
  user,
  workspaceAgentId,
}: MessageUserSectionProps & { message: UserMessage }) {
  const submitEditedContent = useCallback((content: string) => {
    onEditUserMessage?.(message.round_id, content);
  }, [message.round_id, onEditUserMessage]);
  const editor = useUserMessageEditor({
    compact,
    content: user.content,
    onSubmit: projectAvailableUserMessageAction(
      Boolean(onEditUserMessage),
      submitEditedContent,
    ),
  });
  const presentation = projectUserMessagePresentation(
    compact,
    user.content,
    message,
  );

  return (
    <div
      className={cn(
        "nexus-chat-message-section w-full",
        presentation.sectionClassName,
      )}
      data-conversation-round-user-anchor="true"
    >
      <div className="w-full">
        <div className={cn("group flex min-w-0 justify-end", presentation.rowClassName)}>
          <div
            className="relative ml-auto w-fit max-w-[min(100%,720px)] data-[editing=true]:w-full"
            data-editing={String(editor.isEditing)}
          >
            {editor.isEditing ? (
              <UserMessageEditor
                canSubmit={editor.canSubmit}
                compact={compact}
                draftContent={editor.draftContent}
                onCancel={editor.cancel}
                onChange={editor.setDraftContent}
                onSubmit={editor.submit}
                textareaRef={editor.textareaRef}
              />
            ) : (
              <>
                <UserMessageHeader
                  copied={user.copied}
                  currentUserAvatar={currentUserAvatar}
                  onCopy={user.copy}
                  onEdit={projectAvailableUserMessageAction(
                    Boolean(onEditUserMessage),
                    editor.start,
                  )}
                  presentation={presentation}
                />
                <UserMessageContent
                  attachments={user.attachments}
                  content={user.content}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  presentation={presentation}
                  workspaceAgentId={workspaceAgentId}
                />
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
