/**
 * INPUT: 单条 durable user message 与编辑、复制、附件回调。
 * OUTPUT: 可独立渲染的用户消息区块。
 * POS: DM / Room 共用的 user message 视图。
 */
import { useCallback } from "react";

import { cn } from "@/shared/ui/class-name";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import type { UserMessage } from "@/types/conversation/message/entity";

import { UserMessageContent } from "./user-message-content";
import { UserMessageEditor } from "./user-message-editor";
import { UserMessageHeader } from "./user-message-header";
import {
  projectAvailableUserMessageAction,
  projectUserMessagePresentation,
} from "./user-message-model";
import { useUserMessageEditor } from "./use-user-message-editor";

interface MessageUserSectionProps {
  compact: boolean;
  currentUserAvatar?: string | null;
  message: UserMessage;
  onEditUserMessage?: (messageId: string, newContent: string) => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  workspaceAgentId?: string | null;
}

export function MessageUserSection({
  compact,
  currentUserAvatar,
  message,
  onEditUserMessage,
  onOpenWorkspaceFile,
  workspaceAgentId,
}: MessageUserSectionProps) {
  const { copied, copy } = useCopyToClipboard();
  const attachments = message.attachments ?? [];
  const handleCopy = useCallback(async () => {
    await copy(message.content);
  }, [copy, message.content]);
  const submitEditedContent = useCallback((content: string) => {
    onEditUserMessage?.(message.round_id, content);
  }, [message.round_id, onEditUserMessage]);
  const editor = useUserMessageEditor({
    compact,
    content: message.content,
    onSubmit: projectAvailableUserMessageAction(
      Boolean(onEditUserMessage),
      submitEditedContent,
    ),
  });
  const presentation = projectUserMessagePresentation(
    compact,
    message.content,
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
                  copied={copied}
                  currentUserAvatar={currentUserAvatar}
                  onCopy={handleCopy}
                  onEdit={projectAvailableUserMessageAction(
                    Boolean(onEditUserMessage),
                    editor.start,
                  )}
                  presentation={presentation}
                />
                <UserMessageContent
                  attachments={attachments}
                  content={message.content}
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
