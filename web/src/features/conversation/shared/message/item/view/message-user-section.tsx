"use client";

import { useEffect, useRef, useState } from "react";
import {
  Check,
  Copy,
  CornerDownRight,
  Edit2,
  File,
  FileText,
  Image as ImageIcon,
  User,
} from "lucide-react";

import { useTextareaHeight } from "@/hooks/ui/use-textarea-height";
import { cn } from "@/lib/utils";
import { getUiButtonClassName } from "@/shared/ui/button-styles";
import { MessageActionButton, MessageAvatar } from "../../ui/message-primitives";
import { ContentRenderer } from "./content-renderer";
import { formatMessageTime } from "../message-item-support";
import type { MessageItemState } from "../message-item-types";
import type { MessageAttachment } from "@/types/conversation/message";

interface MessageUserSectionProps {
  compact: boolean;
  userMessage: MessageItemState["userMessage"];
  userContent: string;
  userAttachments: MessageAttachment[];
  currentUserAvatar?: string | null;
  copiedUser: boolean;
  onCopyUser: () => Promise<void>;
  onEditUserMessage?: (messageId: string, newContent: string) => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  workspaceAgentId?: string | null;
}

function getUserAttachmentIcon(kind: MessageAttachment["kind"]) {
  if (kind === "image") {
    return ImageIcon;
  }
  if (kind === "text") {
    return FileText;
  }
  return File;
}

function getUserAttachmentKindLabel(kind: MessageAttachment["kind"]) {
  if (kind === "image") {
    return "图片";
  }
  if (kind === "text") {
    return "文本";
  }
  return "文件";
}

function MessageAttachmentList({
  attachments,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  workspaceAgentId: workspaceAgentId,
}: {
  attachments: MessageAttachment[];
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  workspaceAgentId?: string | null;
}) {
  if (attachments.length === 0) {
    return null;
  }

  return (
    <div className="mt-2 flex flex-wrap justify-end gap-1.5">
      {attachments.map((attachment, index) => {
        const Icon = getUserAttachmentIcon(attachment.kind);
        const canOpen =
          Boolean(onOpenWorkspaceFile) &&
          Boolean(attachment.workspace_path) &&
          Boolean(workspaceAgentId) &&
          attachment.workspace_agent_id === workspaceAgentId;
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
              {getUserAttachmentKindLabel(attachment.kind)}
            </span>
          </>
        );

        if (!canOpen) {
          return (
            <span
              key={`${attachment.workspace_path}-${index}`}
              className={className}
              title={title}
            >
              {content}
            </span>
          );
        }

        return (
          <button
            key={`${attachment.workspace_path}-${index}`}
            type="button"
            className={className}
            title={title}
            onClick={() => onOpenWorkspaceFile?.(attachment.workspace_path, attachment.workspace_agent_id ?? workspaceAgentId)}
          >
            {content}
          </button>
        );
      })}
    </div>
  );
}

export function MessageUserSection({
  compact,
  userMessage: userMessage,
  userContent: userContent,
  userAttachments: userAttachments,
  currentUserAvatar: currentUserAvatar,
  copiedUser: copiedUser,
  onCopyUser: onCopyUser,
  onEditUserMessage: onEditUserMessage,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  workspaceAgentId: workspaceAgentId,
}: MessageUserSectionProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [draftContent, setDraftContent] = useState(userContent);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const normalizedDraftContent = draftContent.trim();
  const canSubmitEdit =
    Boolean(normalizedDraftContent) && normalizedDraftContent !== userContent;

  useEffect(() => {
    if (!isEditing) {
      setDraftContent(userContent);
    }
  }, [isEditing, userContent]);

  useEffect(() => {
    if (!isEditing) {
      return;
    }
    const textarea = textareaRef.current;
    textarea?.focus();
    textarea?.setSelectionRange(textarea.value.length, textarea.value.length);
  }, [isEditing]);
  useTextareaHeight(textareaRef, draftContent, {
    minHeight: compact ? 60 : 64,
    maxHeight: 120,
    lineHeight: 24,
    paddingY: compact ? 12 : 16,
  });

  if (!userMessage) {
    return null;
  }
  const isGuidedUserMessage =
    userMessage.role === "user" && userMessage.delivery_policy === "guide";

  const cancelEdit = () => {
    setDraftContent(userContent);
    setIsEditing(false);
  };

  const submitEdit = () => {
    if (!onEditUserMessage || !canSubmitEdit) {
      cancelEdit();
      return;
    }
    onEditUserMessage(userMessage.round_id, normalizedDraftContent);
    setIsEditing(false);
  };

  return (
    <div
      className={cn("nexus-chat-message-section w-full", compact ? "px-0" : "px-2 sm:px-3")}
      data-conversation-round-user-anchor="true"
    >
      <div className="w-full">
        <div
          className={cn(
            "group flex min-w-0 justify-end",
            compact ? "" : "gap-3",
          )}
        >
          <div
            className={cn(
              "relative ml-auto max-w-[min(100%,720px)]",
              isEditing ? "w-full" : "w-fit",
            )}
          >
            {!isEditing ? (
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
                      onClick={() => setIsEditing(true)}
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
                  className="nexus-chat-avatar shrink-0"
                  size={compact ? "compact" : "full"}
                  avatarUrl={currentUserAvatar}
                >
                  {!currentUserAvatar && (
                    <User className={compact ? "h-3 w-3" : "h-4 w-4"} />
                  )}
                </MessageAvatar>
              </div>
            ) : null}

            {isEditing ? (
              <div
                className="input-shell ml-auto flex w-full max-w-full flex-col overflow-hidden rounded-[18px]"
              >
                <textarea
                  ref={textareaRef}
                  aria-label="编辑消息内容"
                  className={cn(
                    "soft-scrollbar min-h-0 resize-none appearance-none border-0 bg-transparent px-3 text-left text-[14px] leading-6 text-(--text-strong)",
                    compact ? "py-1.5" : "py-2",
                    "outline-none shadow-none ring-0 transition-none placeholder:text-(--text-faint)",
                    "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
                  )}
                  rows={2}
                  value={draftContent}
                  onChange={(event) => setDraftContent(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Escape") {
                      event.preventDefault();
                      cancelEdit();
                      return;
                    }
                    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                      event.preventDefault();
                      submitEdit();
                    }
                  }}
                />
                <div className="flex items-center justify-end gap-1.5 border-t border-(--divider-subtle-color) px-2 py-0.5">
                  <button
                    type="button"
                    className={getUiButtonClassName({ size: "xs", variant: "surface" })}
                    onClick={cancelEdit}
                  >
                    取消
                  </button>
                  <button
                    type="button"
                    className={getUiButtonClassName({ size: "xs", variant: "solid" })}
                    disabled={!canSubmitEdit}
                    onClick={submitEdit}
                  >
                    发送
                  </button>
                </div>
              </div>
            ) : (
              <div className="nexus-chat-user-content-shell ml-auto flex w-fit max-w-full flex-col items-end rounded-2xl px-4 py-3">
                {userContent.trim() ? (
                  <ContentRenderer
                    content={userContent}
                    onOpenWorkspaceFile={onOpenWorkspaceFile}
                    workspaceAgentId={workspaceAgentId}
                    className={cn(
                      "nexus-chat-user-content w-fit max-w-[min(100%,760px)] self-end break-words text-left text-(--text-strong)",
                      compact
                        ? "text-[15px] leading-6 [&_.katex-display]:my-2"
                        : "text-[16px] leading-7 [&_.katex-display]:my-3",
                    )}
                  />
                ) : null}
                <MessageAttachmentList
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
