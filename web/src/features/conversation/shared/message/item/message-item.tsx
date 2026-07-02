/**
 * =====================================================
 * @File   ：message-item.tsx
 * @Date   ：2026-04-16 16:02
 * @Author ：leemysw
 * 2026-04-16 16:02   Create
 * =====================================================
 */

"use client";

import { memo } from "react";

import { cn } from "@/lib/utils";

import { MessageShell } from "../ui/message-primitives";
import { MessageAssistantSection } from "./message-assistant-section";
import type { MessageItemProps } from "./message-item-types";
import { MessageUserSection } from "./message-user-section";
import { useMessageItemState } from "./use-message-item-state";

function MessageItemInner({
  compact = false,
  current_agent_name: currentAgentName,
  current_agent_avatar: currentAgentAvatar,
  workspace_agent_id: workspaceAgentId,
  current_user_avatar: currentUserAvatar,
  on_edit_user_message: onEditUserMessage,
  on_open_agent_contact: onOpenAgentContact,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_permission_response: onPermissionResponse,
  can_respond_to_permissions: canRespondToPermissions = true,
  permission_read_only_reason: permissionReadOnlyReason,
  hidden_tool_names: hiddenToolNames = ["TodoWrite"],
  assistant_header_action: assistantHeaderAction,
  assistant_content_mode: assistantContentMode = "dm_archived",
  class_name: className,
  ...restProps
}: MessageItemProps) {
  const state = useMessageItemState({
    compact,
    current_agent_name: currentAgentName,
    current_agent_avatar: currentAgentAvatar,
    on_edit_user_message: onEditUserMessage,
    on_open_workspace_file: onOpenWorkspaceFile,
    on_permission_response: onPermissionResponse,
    can_respond_to_permissions: canRespondToPermissions,
    permission_read_only_reason: permissionReadOnlyReason,
    hidden_tool_names: hiddenToolNames,
    assistant_header_action: assistantHeaderAction,
    assistant_content_mode: assistantContentMode,
    class_name: className,
    ...restProps,
  });

  return (
    <MessageShell
      class_name={cn(
        "nexus-chat-message-round animate-in fade-in slide-in-from-bottom-2 space-y-2 py-3 duration-300",
        compact ? "nexus-chat-message-round-compact" : "nexus-chat-message-round-expanded",
        className,
      )}
      separated={!compact}
    >
      <MessageUserSection
        compact={compact}
        user_message={state.user_message}
        user_content={state.user_content}
        user_attachments={state.user_attachments}
        current_user_avatar={currentUserAvatar}
        copied_user={state.copied_user}
        on_copy_user={state.handle_copy_user}
        on_edit_user_message={onEditUserMessage}
        on_open_workspace_file={onOpenWorkspaceFile}
        workspace_agent_id={workspaceAgentId}
      />

      <MessageAssistantSection
        compact={compact}
        current_agent_name={currentAgentName}
        current_agent_avatar={currentAgentAvatar}
        can_respond_to_permissions={canRespondToPermissions}
        permission_read_only_reason={permissionReadOnlyReason}
        on_permission_response={onPermissionResponse}
        on_open_agent_contact={onOpenAgentContact}
        on_open_workspace_file={onOpenWorkspaceFile}
        workspace_agent_id={workspaceAgentId}
        hidden_tool_names={hiddenToolNames}
        assistant_header_action={assistantHeaderAction}
        assistant_content_mode={assistantContentMode}
        state={state}
      />
    </MessageShell>
  );
}

// 仅在影响视觉输出的关键属性变化时重新渲染，避免流式阶段产生无效更新。
const MessageItem = memo(MessageItemInner, (prev, next) => {
  if (prev.round_id !== next.round_id) return false;
  if (prev.is_last_round !== next.is_last_round) return false;
  if (prev.is_loading !== next.is_loading) return false;
  if (prev.runtime_phase !== next.runtime_phase) return false;
  if (prev.compact !== next.compact) return false;
  if (prev.current_agent_name !== next.current_agent_name) return false;
  if (prev.current_agent_avatar !== next.current_agent_avatar) return false;
  if (prev.workspace_agent_id !== next.workspace_agent_id) return false;
  if (prev.current_user_avatar !== next.current_user_avatar) return false;
  if (prev.pending_permissions !== next.pending_permissions) return false;
  if (prev.can_respond_to_permissions !== next.can_respond_to_permissions) return false;
  if (prev.permission_read_only_reason !== next.permission_read_only_reason) return false;
  if (prev.on_open_agent_contact !== next.on_open_agent_contact) return false;
  if (prev.assistant_header_action !== next.assistant_header_action) return false;
  if (prev.assistant_content_mode !== next.assistant_content_mode) return false;
  if (prev.class_name !== next.class_name) return false;
  if (prev.messages !== next.messages) return false;
  return true;
});

export default MessageItem;
