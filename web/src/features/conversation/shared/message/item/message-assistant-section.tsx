"use client";

import { type ReactNode, useCallback } from "react";
import {
  Bot,
  ChevronDown,
  ChevronRight,
  Square,
  Wrench,
} from "lucide-react";

import { cn } from "@/lib/utils";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";
import { ToolBlock } from "../blocks/tool-block";
import { useWorkspaceFileArtifactsFromContent } from "../blocks/workspace-file-artifact-utils";
import { WorkspaceFileArtifactList } from "../blocks/workspace-file-artifacts";
import { MessageStats } from "../ui/message-stats";
import {
  MessageActionButton,
  MessageActivityStatus,
  MessageAvatar,
} from "../ui/message-primitives";
import { ContentRenderer } from "./content-renderer";
import { format_message_time } from "./message-item-support";
import type { MessageItemState } from "./message-item-types";
import type { ContentBlock } from "@/types/conversation/message";

const EMPTY_CONTENT_BLOCKS: ContentBlock[] = [];

interface PendingPermissionListProps {
  permissions: PendingPermission[];
  is_room_thread_mode: boolean;
  can_respond_to_permissions: boolean;
  permission_read_only_reason?: string;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  workspace_agent_id?: string | null;
}

function PendingPermissionList({
  permissions,
  is_room_thread_mode,
  can_respond_to_permissions,
  permission_read_only_reason,
  on_permission_response,
  workspace_agent_id,
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
          : "rounded-2xl bg-transparent p-3",
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
          workspace_agent_id={workspace_agent_id}
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
  on_open_agent_contact?: (agent_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
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
  on_open_agent_contact,
  on_open_workspace_file,
  workspace_agent_id,
  hidden_tool_names = ["TodoWrite"],
  assistant_header_action,
  assistant_content_mode,
  state,
}: MessageAssistantSectionProps) {
  const is_room_thread_mode = assistant_content_mode === "room_thread";
  const content_workspace_agent_id = state.assistant_agent_id ?? workspace_agent_id;
  const avatar_agent_id = state.assistant_agent_id ?? workspace_agent_id ?? null;
  const collapsed_process_file_artifacts = useWorkspaceFileArtifactsFromContent(
    state.should_render_process_callchain && !state.is_process_expanded
      ? state.process_projection.content
      : EMPTY_CONTENT_BLOCKS,
  );
  const handle_open_agent_contact = useCallback(() => {
    if (!avatar_agent_id) {
      return;
    }
    on_open_agent_contact?.(avatar_agent_id);
  }, [avatar_agent_id, on_open_agent_contact]);

  if (state.should_hide_assistant_content) {
    return null;
  }

  const pending_permission_block = (
    <PendingPermissionList
      permissions={state.unmatched_pending_permissions}
      is_room_thread_mode={is_room_thread_mode}
      can_respond_to_permissions={can_respond_to_permissions}
      permission_read_only_reason={permission_read_only_reason}
      on_permission_response={on_permission_response}
      workspace_agent_id={content_workspace_agent_id}
    />
  );

  return (
    <div className={cn("nexus-chat-message-section w-full", compact ? "px-0" : "px-2 sm:px-3")}>
      <div className={cn("w-full", compact ? "max-w-full" : "max-w-[980px]")}>
        <div
          className={cn(
            "nexus-chat-assistant-grid group grid min-w-0",
            compact
              ? "grid-cols-[minmax(0,1fr)]"
              : "nexus-chat-assistant-grid-expanded grid-cols-[40px_minmax(0,1fr)] gap-3",
          )}
        >
          {!compact ? (
            <MessageAvatar
              aria_label={`打开 ${current_agent_name || "协作成员"} 的联络`}
              class_name="nexus-chat-avatar"
              avatar_url={current_agent_avatar}
              on_click={
                avatar_agent_id && on_open_agent_contact
                  ? handle_open_agent_contact
                  : undefined
              }
              title={`打开 ${current_agent_name || "协作成员"} 的联络`}
            >
              {!current_agent_avatar && <Bot className="h-4 w-4" />}
            </MessageAvatar>
          ) : null}

          <div className="relative min-w-0">
            <div
              className={cn(
                "nexus-chat-message-header flex min-w-0 items-center gap-2",
                compact ? "min-h-6 pb-0" : "h-7 pb-0.5",
              )}
            >
              {compact ? (
                <MessageAvatar
                  aria_label={`打开 ${current_agent_name || "协作成员"} 的联络`}
                  class_name="nexus-chat-avatar shrink-0"
                  size="compact"
                  avatar_url={current_agent_avatar}
                  on_click={
                    avatar_agent_id && on_open_agent_contact
                      ? handle_open_agent_contact
                      : undefined
                  }
                  title={`打开 ${current_agent_name || "协作成员"} 的联络`}
                >
                  {!current_agent_avatar && <Bot className="h-3 w-3" />}
                </MessageAvatar>
              ) : null}
              <span className="nexus-chat-author shrink-0 text-sm font-bold text-(--text-strong)">
                {current_agent_name || "协作成员"}
              </span>

              {state.timestamp ? (
                <span className="nexus-chat-meta hidden shrink-0 text-xs text-(--text-muted) sm:inline">
                  {format_message_time(state.timestamp)}
                </span>
              ) : null}

              {state.model ? (
                <span className="nexus-chat-meta min-w-0 truncate text-xs text-(--text-soft)">
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
                "nexus-chat-message-content min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-left",
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
                    workspace_agent_id={content_workspace_agent_id}
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

                  {!state.is_process_expanded ? (
                    <WorkspaceFileArtifactList
                      artifacts={collapsed_process_file_artifacts}
                      class_name="ml-5 pb-1"
                      label="生成文件"
                      on_open_workspace_file={on_open_workspace_file}
                    />
                  ) : null}

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
                        workspace_agent_id={content_workspace_agent_id}
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
                    workspace_agent_id={content_workspace_agent_id}
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
