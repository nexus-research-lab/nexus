/**
 * =====================================================
 * @File   ：message-item-sections.tsx
 * @Date   ：2026-04-16 15:54
 * @Author ：leemysw
 * 2026-04-16 15:54   Create
 * =====================================================
 */

"use client";

import { type ReactNode, useState } from "react";
import {
  Bot,
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
  Edit2,
  Square,
  User,
  Wrench,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { PromptDialog } from "@/shared/ui/dialog/confirm-dialog";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

import { ToolBlock } from "../blocks/tool-block";
import { MessageStats } from "../ui/message-stats";
import {
  MessageActionButton,
  MessageActivityStatus,
  MessageAvatar,
} from "../ui/message-primitives";
import { ContentRenderer } from "./content-renderer";
import { format_message_time } from "./message-item-support";
import type { MessageItemState } from "./message-item-types";

interface MessageUserSectionProps {
  compact: boolean;
  user_message: MessageItemState["user_message"];
  user_content: string;
  copied_user: boolean;
  on_copy_user: () => Promise<void>;
  on_edit_user_message?: (message_id: string, new_content: string) => void;
  on_open_workspace_file?: (path: string) => void;
}

export function MessageUserSection({
  compact,
  user_message,
  user_content,
  copied_user,
  on_copy_user,
  on_edit_user_message,
  on_open_workspace_file,
}: MessageUserSectionProps) {
  const [is_edit_dialog_open, set_is_edit_dialog_open] = useState(false);

  if (!user_message) {
    return null;
  }

  return (
    <div className={cn("w-full", compact ? "px-0" : "px-2 sm:px-3")}>
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
                "flex items-center justify-end gap-2",
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

              <span className="hidden shrink-0 text-xs text-(--text-muted) sm:inline">
                {format_message_time(user_message.timestamp)}
              </span>
              <span className="shrink-0 text-sm font-bold text-(--text-strong)">
                你
              </span>
              <MessageAvatar
                class_name="shrink-0"
                size={compact ? "compact" : "full"}
              >
                <User className={compact ? "h-3 w-3" : "h-4 w-4"} />
              </MessageAvatar>
            </div>

            <div className="ml-auto w-fit max-w-full rounded-2xl bg-[color-mix(in_srgb,var(--primary)_6%,var(--material-card-background))] px-4 py-3">
              <ContentRenderer
                content={user_content}
                on_open_workspace_file={on_open_workspace_file}
                class_name={cn(
                  "text-left text-(--text-strong)",
                  compact
                    ? "text-[15px] leading-6 [&_.katex-display]:my-2"
                    : "text-[16px] leading-7 [&_.katex-display]:my-3",
                )}
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

interface PendingPermissionListProps {
  permissions: PendingPermission[];
  is_room_thread_mode: boolean;
  can_respond_to_permissions: boolean;
  permission_read_only_reason?: string;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
}

function PendingPermissionList({
  permissions,
  is_room_thread_mode,
  can_respond_to_permissions,
  permission_read_only_reason,
  on_permission_response,
}: PendingPermissionListProps) {
  if (permissions.length === 0) {
    return null;
  }

  return (
    <div
      className={cn(
        "mt-3 flex flex-col gap-3",
        is_room_thread_mode
          ? "border-t border-(--divider-subtle-color) pt-3"
          : "rounded-2xl bg-(--surface-inset-background) p-3",
      )}
    >
      {permissions.map((permission) => (
        <ToolBlock
          key={permission.request_id}
          tool_use={{
            type: "tool_use",
            id: `pending_${permission.request_id}`,
            name: permission.tool_name,
            input: permission.tool_input,
          }}
          status="waiting_permission"
          permission_request={{
            request_id: permission.request_id,
            tool_input: permission.tool_input,
            risk_level: permission.risk_level,
            risk_label: permission.risk_label,
            summary: permission.summary,
            suggestions: permission.suggestions,
            expires_at: permission.expires_at,
            on_allow: (updated_permissions) =>
              on_permission_response?.({
                request_id: permission.request_id,
                decision: "allow",
                updated_permissions,
              }),
            on_deny: (updated_permissions) =>
              on_permission_response?.({
                request_id: permission.request_id,
                decision: "deny",
                updated_permissions,
              }),
          }}
          interaction_disabled={!can_respond_to_permissions}
          interaction_disabled_reason={permission_read_only_reason}
        />
      ))}
    </div>
  );
}

interface MessageAssistantSectionProps {
  compact: boolean;
  current_agent_name?: string | null;
  current_agent_avatar?: string | null;
  can_respond_to_permissions: boolean;
  permission_read_only_reason?: string;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  on_open_workspace_file?: (path: string) => void;
  hidden_tool_names?: string[];
  assistant_header_action?: ReactNode;
  assistant_content_mode:
    | "dm_live"
    | "dm_archived"
    | "room_thread"
    | "room_result";
  state: MessageItemState;
}

export function MessageAssistantSection({
  compact,
  current_agent_name,
  current_agent_avatar,
  can_respond_to_permissions,
  permission_read_only_reason,
  on_permission_response,
  on_open_workspace_file,
  hidden_tool_names = ["TodoWrite"],
  assistant_header_action,
  assistant_content_mode,
  state,
}: MessageAssistantSectionProps) {
  if (state.should_hide_assistant_content) {
    return null;
  }

  const is_room_thread_mode = assistant_content_mode === "room_thread";
  const pending_permission_block = (
    <PendingPermissionList
      permissions={state.unmatched_pending_permissions}
      is_room_thread_mode={is_room_thread_mode}
      can_respond_to_permissions={can_respond_to_permissions}
      permission_read_only_reason={permission_read_only_reason}
      on_permission_response={on_permission_response}
    />
  );

  return (
    <div className={cn("w-full", compact ? "px-0" : "px-2 sm:px-3")}>
      <div className={cn("w-full", compact ? "max-w-full" : "max-w-[980px]")}>
        <div
          className={cn(
            "group grid min-w-0",
            compact
              ? "grid-cols-[minmax(0,1fr)]"
              : "grid-cols-[40px_minmax(0,1fr)] gap-3",
          )}
        >
          {!compact ? (
            <MessageAvatar avatar_url={current_agent_avatar}>
              {!current_agent_avatar && <Bot className="h-4 w-4" />}
            </MessageAvatar>
          ) : null}

          <div className="relative min-w-0">
            <div
              className={cn(
                "flex min-w-0 items-center gap-2",
                compact ? "min-h-6 pb-0" : "h-7 pb-0.5",
              )}
            >
              {compact ? (
                <MessageAvatar
                  class_name="shrink-0"
                  size="compact"
                  avatar_url={current_agent_avatar}
                >
                  {!current_agent_avatar && <Bot className="h-3 w-3" />}
                </MessageAvatar>
              ) : null}
              <span className="shrink-0 text-sm font-bold text-(--text-strong)">
                {current_agent_name || "协作成员"}
              </span>

              <span className="hidden shrink-0 text-xs text-(--text-muted) sm:inline">
                {format_message_time(state.timestamp)}
              </span>

              {state.model ? (
                <span className="min-w-0 truncate text-xs text-(--text-soft)">
                  {state.model}
                </span>
              ) : null}

              <div className="flex-1" />

              {assistant_header_action ? (
                <div className="shrink-0">{assistant_header_action}</div>
              ) : null}

              {state.can_stop_message ? (
                <MessageActionButton
                  type="button"
                  aria-label="停止生成"
                  onClick={state.handle_stop_message}
                  class_name="flex items-center gap-1 px-1.5 py-0.5 text-xs"
                  tone="default"
                >
                  <Square className="h-3 w-3 fill-current" />
                  <span>停止</span>
                </MessageActionButton>
              ) : null}
            </div>

            <div
              ref={state.content_area_ref}
              className={cn(
                "min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-left",
                compact ? "text-[15px] leading-6" : "text-[16px] leading-7",
              )}
              style={state.content_area_style}
            >
              {state.should_render_standalone_activity_status ? (
                <MessageActivityStatus
                  class_name="py-1"
                  state={state.live_activity_state!}
                />
              ) : null}

              {state.stream_status === "cancelled" &&
              state.merged_content_length === 0 ? (
                <span className="text-xs italic text-(--text-soft)">
                  已停止
                </span>
              ) : null}

              {state.stream_status === "error" &&
              state.merged_content_length === 0 ? (
                <span className="text-xs italic text-rose-500">执行失败</span>
              ) : null}

              {state.should_render_direct_assistant_content ? (
                <div>
                  <ContentRenderer
                    content={state.direct_ordered_projection.content}
                    is_streaming={state.show_cursor}
                    streaming_block_indexes={
                      state.direct_ordered_projection.streaming_indexes
                    }
                    fallback_activity_state={state.live_activity_state}
                    pending_permissions_by_tool_use_id={
                      state.matched_pending_permissions_by_tool_use_id
                    }
                    on_permission_response={on_permission_response}
                    can_respond_to_permissions={can_respond_to_permissions}
                    permission_read_only_reason={permission_read_only_reason}
                    on_open_workspace_file={on_open_workspace_file}
                    hidden_tool_names={hidden_tool_names}
                    show_timeline_dots
                  />
                  {pending_permission_block}
                </div>
              ) : null}

              {state.should_render_process_callchain ? (
                <div
                  ref={
                    state.process_anchor_ref as React.RefObject<HTMLDivElement>
                  }
                >
                  <button
                    className="flex w-full items-center gap-2 py-1.5 text-left text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)"
                    onClick={state.toggle_process_expanded}
                    type="button"
                  >
                    <Wrench className="h-3 w-3 shrink-0 text-(--icon-muted)" />
                    <div className="min-w-0 flex-1 truncate text-[12px] font-medium text-(--text-muted)">
                      {state.process_summary}
                    </div>
                    <div className="text-(--icon-muted)">
                      {state.is_process_expanded ? (
                        <ChevronDown className="h-3.5 w-3.5" />
                      ) : (
                        <ChevronRight className="h-3.5 w-3.5" />
                      )}
                    </div>
                  </button>

                  {state.is_process_expanded ? (
                    <div className="pt-1">
                      <ContentRenderer
                        content={state.process_projection.content}
                        is_streaming={state.show_cursor}
                        streaming_block_indexes={
                          state.process_projection.streaming_indexes
                        }
                        fallback_activity_state={state.live_activity_state}
                        pending_permissions_by_tool_use_id={
                          state.matched_pending_permissions_by_tool_use_id
                        }
                        on_permission_response={on_permission_response}
                        can_respond_to_permissions={can_respond_to_permissions}
                        permission_read_only_reason={
                          permission_read_only_reason
                        }
                        on_open_workspace_file={on_open_workspace_file}
                        hidden_tool_names={hidden_tool_names}
                        class_name="ml-1"
                        show_timeline_dots
                      />

                      {pending_permission_block}
                    </div>
                  ) : null}
                </div>
              ) : null}

              {state.should_render_assistant_text ? (
                <div className={cn(state.should_render_process_callchain)}>
                  <ContentRenderer
                    content={state.final_assistant_content ?? []}
                    is_streaming={state.final_assistant_is_streaming}
                    streaming_block_indexes={
                      state.final_assistant_streaming_indexes
                    }
                    fallback_activity_state={state.live_activity_state}
                    on_open_workspace_file={on_open_workspace_file}
                  />
                </div>
              ) : null}

              {!state.should_render_direct_assistant_content &&
              !state.should_render_process_callchain ? (
                <div className="pt-2">{pending_permission_block}</div>
              ) : null}
            </div>

            {state.should_show_assistant_footer ? (
              <MessageStats
                stats={state.stats || undefined}
                show_cursor={state.show_cursor}
                compact={compact}
                copied_assistant={state.copied_assistant}
                on_copy_assistant={
                  state.can_copy_assistant
                    ? state.handle_copy_assistant
                    : undefined
                }
              />
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}
