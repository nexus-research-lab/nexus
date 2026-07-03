"use client";

import { useState } from "react";
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

import { cn } from "@/lib/utils";
import { PromptDialog } from "@/shared/ui/dialog/confirm-dialog";
import { MessageActionButton, MessageAvatar } from "../ui/message-primitives";
import { ContentRenderer } from "./content-renderer";
import { formatMessageTime } from "./message-item-support";
import type { MessageItemState } from "./message-item-types";
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
  onOpenWorkspaceFile?: (path: string) => void;
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
  onOpenWorkspaceFile?: (path: string) => void;
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
            onClick={() => onOpenWorkspaceFile?.(attachment.workspace_path)}
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
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);

  if (!userMessage) {
    return null;
  }
  const isGuidedUserMessage =
    userMessage.role === "user" && userMessage.delivery_policy === "guide";

  return (
    <div className={cn("nexus-chat-message-section w-full", compact ? "px-0" : "px-2 sm:px-3")}>
      <div className="w-full">
        <div
          className={cn(
            "group flex min-w-0 justify-end",
            compact ? "" : "gap-3",
          )}
        >
          <div className="relative ml-auto w-fit max-w-[min(100%,720px)]">
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
                    onClick={() => setIsEditDialogOpen(true)}
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
          </div>
        </div>
      </div>

      {onEditUserMessage ? (
        <PromptDialog
          isOpen={isEditDialogOpen}
          title="编辑消息"
          message="修改后的内容会直接替换当前这条用户消息。"
          placeholder="输入新的消息内容"
          defaultValue={userContent}
          multiline
          onCancel={() => setIsEditDialogOpen(false)}
          onConfirm={(nextContent) => {
            const normalizedContent = nextContent.trim();
            if (!normalizedContent || normalizedContent === userContent) {
              setIsEditDialogOpen(false);
              return;
            }
            onEditUserMessage(userMessage.message_id, normalizedContent);
            setIsEditDialogOpen(false);
          }}
        />
      ) : null}
    </div>
  );
}
