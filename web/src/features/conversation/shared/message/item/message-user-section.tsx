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
import { format_message_time } from "./message-item-support";
import type { MessageItemState } from "./message-item-types";
import type { MessageAttachment } from "@/types/conversation/message";

interface MessageUserSectionProps {
  compact: boolean;
  user_message: MessageItemState["user_message"];
  user_content: string;
  user_attachments: MessageAttachment[];
  current_user_avatar?: string | null;
  copied_user: boolean;
  on_copy_user: () => Promise<void>;
  on_edit_user_message?: (message_id: string, new_content: string) => void;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
}

function get_user_attachment_icon(kind: MessageAttachment["kind"]) {
  if (kind === "image") {
    return ImageIcon;
  }
  if (kind === "text") {
    return FileText;
  }
  return File;
}

function get_user_attachment_kind_label(kind: MessageAttachment["kind"]) {
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
  on_open_workspace_file,
  workspace_agent_id,
}: {
  attachments: MessageAttachment[];
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
}) {
  if (attachments.length === 0) {
    return null;
  }

  return (
    <div className="mt-2 flex flex-wrap justify-end gap-1.5">
      {attachments.map((attachment, index) => {
        const Icon = get_user_attachment_icon(attachment.kind);
        const can_open =
          Boolean(on_open_workspace_file) &&
          Boolean(attachment.workspace_path) &&
          Boolean(workspace_agent_id) &&
          attachment.workspace_agent_id === workspace_agent_id;
        const title = `${attachment.file_name || attachment.workspace_path} · ${attachment.workspace_path}`;
        const class_name = cn(
          "inline-flex max-w-[260px] items-center gap-1.5 rounded-[7px] border px-2.5 py-1 text-xs font-medium",
          "border-(--divider-subtle-color) bg-transparent text-(--text-muted)",
          can_open
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
              {get_user_attachment_kind_label(attachment.kind)}
            </span>
          </>
        );

        if (!can_open) {
          return (
            <span
              key={`${attachment.workspace_path}-${index}`}
              className={class_name}
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
            className={class_name}
            title={title}
            onClick={() => on_open_workspace_file?.(attachment.workspace_path)}
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
  user_message,
  user_content,
  user_attachments,
  current_user_avatar,
  copied_user,
  on_copy_user,
  on_edit_user_message,
  on_open_workspace_file,
  workspace_agent_id,
}: MessageUserSectionProps) {
  const [is_edit_dialog_open, set_is_edit_dialog_open] = useState(false);

  if (!user_message) {
    return null;
  }
  const is_guided_user_message =
    user_message.role === "user" && user_message.delivery_policy === "guide";

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
                {on_edit_user_message ? (
                  <MessageActionButton
                    aria-label="编辑消息"
                    onClick={() => set_is_edit_dialog_open(true)}
                    tone="default"
                  >
                    <Edit2 className="h-3 w-3" />
                  </MessageActionButton>
                ) : null}
                <MessageActionButton
                  aria-label="复制消息"
                  onClick={on_copy_user}
                  tone={copied_user ? "success" : "default"}
                >
                  {copied_user ? (
                    <Check className="h-3 w-3" />
                  ) : (
                    <Copy className="h-3 w-3" />
                  )}
                </MessageActionButton>
              </div>

              <span className="nexus-chat-meta hidden shrink-0 text-xs text-(--text-muted) sm:inline">
                {format_message_time(user_message.timestamp)}
              </span>
              {is_guided_user_message ? (
                <span className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-(--text-muted)">
                  <CornerDownRight className="h-3.5 w-3.5" />
                  已引导对话
                </span>
              ) : null}
              <span className="nexus-chat-author shrink-0 text-sm font-bold text-(--text-strong)">
                你
              </span>
              <MessageAvatar
                class_name="nexus-chat-avatar shrink-0"
                size={compact ? "compact" : "full"}
                avatar_url={current_user_avatar}
              >
                {!current_user_avatar && (
                  <User className={compact ? "h-3 w-3" : "h-4 w-4"} />
                )}
              </MessageAvatar>
            </div>

            <div className="nexus-chat-user-content-shell ml-auto flex w-fit max-w-full flex-col items-end rounded-2xl px-4 py-3">
              {user_content.trim() ? (
                <ContentRenderer
                  content={user_content}
                  on_open_workspace_file={on_open_workspace_file}
                  workspace_agent_id={workspace_agent_id}
                  class_name={cn(
                    "nexus-chat-user-content w-fit max-w-[min(100%,760px)] self-end break-words text-left text-(--text-strong)",
                    compact
                      ? "text-[15px] leading-6 [&_.katex-display]:my-2"
                      : "text-[16px] leading-7 [&_.katex-display]:my-3",
                  )}
                />
              ) : null}
              <MessageAttachmentList
                attachments={user_attachments}
                on_open_workspace_file={on_open_workspace_file}
                workspace_agent_id={workspace_agent_id}
              />
            </div>
          </div>
        </div>
      </div>

      {on_edit_user_message ? (
        <PromptDialog
          is_open={is_edit_dialog_open}
          title="编辑消息"
          message="修改后的内容会直接替换当前这条用户消息。"
          placeholder="输入新的消息内容"
          default_value={user_content}
          multiline
          on_cancel={() => set_is_edit_dialog_open(false)}
          on_confirm={(next_content) => {
            const normalized_content = next_content.trim();
            if (!normalized_content || normalized_content === user_content) {
              set_is_edit_dialog_open(false);
              return;
            }
            on_edit_user_message(user_message.message_id, normalized_content);
            set_is_edit_dialog_open(false);
          }}
        />
      ) : null}
    </div>
  );
}
