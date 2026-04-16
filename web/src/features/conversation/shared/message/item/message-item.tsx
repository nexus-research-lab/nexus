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
import { MessageAssistantSection, MessageUserSection } from "./message-item-view";
import type { MessageItemProps } from "./message-item-types";
import { useMessageItemState } from "./message-item-model";

function MessageItemInner({
  compact = false,
  current_agent_name,
  current_agent_avatar,
  on_edit_user_message,
  on_open_workspace_file,
  on_permission_response,
  can_respond_to_permissions = true,
  permission_read_only_reason,
  hidden_tool_names = ["TodoWrite"],
  assistant_header_action,
  assistant_content_mode = "dm_archived",
  class_name,
  ...rest_props
}: MessageItemProps) {
  const state = useMessageItemState({
    compact,
    current_agent_name,
    current_agent_avatar,
    on_edit_user_message,
    on_open_workspace_file,
    on_permission_response,
    can_respond_to_permissions,
    permission_read_only_reason,
    hidden_tool_names,
    assistant_header_action,
    assistant_content_mode,
    class_name,
    ...rest_props,
  });

  return (
    <MessageShell
      class_name={cn(
        "animate-in fade-in slide-in-from-bottom-2 space-y-2 py-3 duration-300",
        class_name,
      )}
      separated={!compact}
    >
      <MessageUserSection
        compact={compact}
        user_message={state.user_message}
        user_content={state.user_content}
        copied_user={state.copied_user}
        on_copy_user={state.handle_copy_user}
        on_edit_user_message={on_edit_user_message}
      />

      <MessageAssistantSection
        compact={compact}
        current_agent_name={current_agent_name}
        current_agent_avatar={current_agent_avatar}
        can_respond_to_permissions={can_respond_to_permissions}
        permission_read_only_reason={permission_read_only_reason}
        on_permission_response={on_permission_response}
        on_open_workspace_file={on_open_workspace_file}
        hidden_tool_names={hidden_tool_names}
        assistant_header_action={assistant_header_action}
        assistant_content_mode={assistant_content_mode}
        state={state}
      />
    </MessageShell>
  );
}

// 仅在影响视觉输出的关键属性变化时重新渲染，避免流式阶段产生无效更新。
export const MessageItem = memo(MessageItemInner, (prev, next) => {
  if (prev.round_id !== next.round_id) return false;
  if (prev.is_last_round !== next.is_last_round) return false;
  if (prev.is_loading !== next.is_loading) return false;
  if (prev.runtime_phase !== next.runtime_phase) return false;
  if (prev.compact !== next.compact) return false;
  if (prev.current_agent_name !== next.current_agent_name) return false;
  if (prev.pending_permissions !== next.pending_permissions) return false;
  if (prev.can_respond_to_permissions !== next.can_respond_to_permissions) return false;
  if (prev.permission_read_only_reason !== next.permission_read_only_reason) return false;
  if (prev.assistant_header_action !== next.assistant_header_action) return false;
  if (prev.assistant_content_mode !== next.assistant_content_mode) return false;
  if (prev.class_name !== next.class_name) return false;
  if (prev.messages !== next.messages) return false;
  return true;
});

export default MessageItem;
