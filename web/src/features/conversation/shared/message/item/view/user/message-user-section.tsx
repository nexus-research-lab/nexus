"use client";

import { useCallback } from "react";
import {
  Check,
  Copy,
  CornerDownRight,
  Edit2,
  User,
} from "lucide-react";

import { cn } from "@/lib/utils";

import {
  MessageActionButton,
  MessageAvatar,
} from "../../../ui/message-primitives";
import { formatMessageTime } from "../../../message-time";
import type { MessageItemState } from "../../message-item-types";
import { ContentRenderer } from "../content/content-renderer";
import { MessageUserAttachments } from "./message-user-attachments";
import { UserMessageEditor } from "./user-message-editor";
import { useUserMessageEditor } from "./use-user-message-editor";

interface MessageUserSectionProps {
  compact: boolean;
  copiedUser: boolean;
  currentUserAvatar?: string | null;
  onCopyUser: () => Promise<void>;
  onEditUserMessage?: (messageId: string, newContent: string) => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  userAttachments: MessageItemState["userAttachments"];
  userContent: string;
  userMessage: MessageItemState["userMessage"];
  workspaceAgentId?: string | null;
}

export function MessageUserSection({
  compact,
  copiedUser,
  currentUserAvatar,
  onCopyUser,
  onEditUserMessage,
  onOpenWorkspaceFile,
  userAttachments,
  userContent,
  userMessage,
  workspaceAgentId,
}: MessageUserSectionProps) {
  const submitEditedContent = useCallback((content: string) => {
    if (userMessage) {
      onEditUserMessage?.(userMessage.round_id, content);
    }
  }, [onEditUserMessage, userMessage]);
  const editor = useUserMessageEditor({
    compact,
    content: userContent,
    onSubmit: userMessage && onEditUserMessage ? submitEditedContent : undefined,
  });

  if (!userMessage) {
    return null;
  }
  const isGuidedUserMessage =
    userMessage.role === "user" && userMessage.delivery_policy === "guide";

  return (
    <div
      className={cn("nexus-chat-message-section w-full", compact ? "px-0" : "px-2 sm:px-3")}
      data-conversation-round-user-anchor="true"
    >
      <div className="w-full">
        <div className={cn("group flex min-w-0 justify-end", !compact && "gap-3")}>
          <div
            className={cn(
              "relative ml-auto max-w-[min(100%,720px)]",
              editor.isEditing ? "w-full" : "w-fit",
            )}
          >
            {!editor.isEditing ? (
              <div
                className={cn(
                  "nexus-chat-message-header flex items-center justify-end gap-2",
                  compact ? "h-6" : "h-7",
                )}
              >
                <div className="shrink-0 opacity-100 transition-opacity duration-(--motion-duration-fast) sm:opacity-0 sm:group-hover:opacity-100">
                  {onEditUserMessage ? (
                    <MessageActionButton
                      aria-label="编辑消息"
                      onClick={editor.start}
                      tone="default"
                    >
                      <Edit2 className="h-3 w-3" />
                    </MessageActionButton>
                  ) : null}
                  <MessageActionButton
                    aria-label="复制消息"
                    onClick={onCopyUser}
                    tone={copiedUser ? "success" : "default"}
                  >
                    {copiedUser ? (
                      <Check className="h-3 w-3" />
                    ) : (
                      <Copy className="h-3 w-3" />
                    )}
                  </MessageActionButton>
                </div>
                <span className="nexus-chat-meta hidden shrink-0 text-xs text-(--text-muted) sm:inline">
                  {formatMessageTime(userMessage.timestamp)}
                </span>
                {isGuidedUserMessage ? (
                  <span className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-(--text-muted)">
                    <CornerDownRight className="h-3.5 w-3.5" />
                    已引导对话
                  </span>
                ) : null}
                <span className="nexus-chat-author shrink-0 text-sm font-bold text-(--text-strong)">
                  你
                </span>
                <MessageAvatar
                  avatarUrl={currentUserAvatar}
                  className="nexus-chat-avatar shrink-0"
                  size={compact ? "compact" : "full"}
                >
                  {!currentUserAvatar && (
                    <User className={compact ? "h-3 w-3" : "h-4 w-4"} />
                  )}
                </MessageAvatar>
              </div>
            ) : null}

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
              <div className="nexus-chat-user-content-shell ml-auto flex w-fit max-w-full flex-col items-end rounded-2xl px-4 py-3">
                {userContent.trim() ? (
                  <ContentRenderer
                    className={cn(
                      "nexus-chat-user-content w-fit max-w-[min(100%,760px)] self-end break-words text-left text-(--text-strong)",
                      compact
                        ? "text-[15px] leading-6 [&_.katex-display]:my-2"
                        : "text-[16px] leading-7 [&_.katex-display]:my-3",
                    )}
                    content={userContent}
                    onOpenWorkspaceFile={onOpenWorkspaceFile}
                    workspaceAgentId={workspaceAgentId}
                  />
                ) : null}
                <MessageUserAttachments
                  attachments={userAttachments}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  workspaceAgentId={workspaceAgentId}
                />
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
