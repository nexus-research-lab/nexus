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
  is_room_thread_mode: isRoomThreadMode,
  can_respond_to_permissions: canRespondToPermissions,
  permission_read_only_reason: permissionReadOnlyReason,
  on_permission_response: onPermissionResponse,
  workspace_agent_id: workspaceAgentId,
}: PendingPermissionListProps) {
  if (permissions.length === 0) {
    return null;
  }

  return (
    <div
      className={cn(
        "mt-3 flex flex-col gap-3",
        isRoomThreadMode
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
            on_allow: (updatedPermissions) =>
              onPermissionResponse?.({
                request_id: permission.request_id,
                decision: "allow",
                updated_permissions: updatedPermissions,
              }),
            on_deny: (updatedPermissions) =>
              onPermissionResponse?.({
                request_id: permission.request_id,
                decision: "deny",
                updated_permissions: updatedPermissions,
              }),
          }}
          interaction_disabled={!canRespondToPermissions}
          interaction_disabled_reason={permissionReadOnlyReason}
          workspace_agent_id={workspaceAgentId}
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
  on_open_agent_contact?: (agentId: string) => void;
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
  current_agent_name: currentAgentName,
  current_agent_avatar: currentAgentAvatar,
  can_respond_to_permissions: canRespondToPermissions,
  permission_read_only_reason: permissionReadOnlyReason,
  on_permission_response: onPermissionResponse,
  on_open_agent_contact: onOpenAgentContact,
  on_open_workspace_file: onOpenWorkspaceFile,
  workspace_agent_id: workspaceAgentId,
  hidden_tool_names: hiddenToolNames = ["TodoWrite"],
  assistant_header_action: assistantHeaderAction,
  assistant_content_mode: assistantContentMode,
  state,
}: MessageAssistantSectionProps) {
  const isRoomThreadMode = assistantContentMode === "room_thread";
  const contentWorkspaceAgentId = state.assistant_agent_id ?? workspaceAgentId;
  const avatarAgentId = state.assistant_agent_id ?? workspaceAgentId ?? null;
  const collapsedProcessFileArtifacts = useWorkspaceFileArtifactsFromContent(
    state.should_render_process_callchain && !state.is_process_expanded
      ? state.process_projection.content
      : EMPTY_CONTENT_BLOCKS,
  );
  const handleOpenAgentContact = useCallback(() => {
    if (!avatarAgentId) {
      return;
    }
    onOpenAgentContact?.(avatarAgentId);
  }, [avatarAgentId, onOpenAgentContact]);

  if (state.should_hide_assistant_content) {
    return null;
  }

  const pendingPermissionBlock = (
    <PendingPermissionList
      permissions={state.unmatched_pending_permissions}
      is_room_thread_mode={isRoomThreadMode}
      can_respond_to_permissions={canRespondToPermissions}
      permission_read_only_reason={permissionReadOnlyReason}
      on_permission_response={onPermissionResponse}
      workspace_agent_id={contentWorkspaceAgentId}
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
              aria_label={`打开 ${currentAgentName || "协作成员"} 的联络`}
              class_name="nexus-chat-avatar"
              avatar_url={currentAgentAvatar}
              on_click={
                avatarAgentId && onOpenAgentContact
                  ? handleOpenAgentContact
                  : undefined
              }
              title={`打开 ${currentAgentName || "协作成员"} 的联络`}
            >
              {!currentAgentAvatar && <Bot className="h-4 w-4" />}
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
                  aria_label={`打开 ${currentAgentName || "协作成员"} 的联络`}
                  class_name="nexus-chat-avatar shrink-0"
                  size="compact"
                  avatar_url={currentAgentAvatar}
                  on_click={
                    avatarAgentId && onOpenAgentContact
                      ? handleOpenAgentContact
                      : undefined
                  }
                  title={`打开 ${currentAgentName || "协作成员"} 的联络`}
                >
                  {!currentAgentAvatar && <Bot className="h-3 w-3" />}
                </MessageAvatar>
              ) : null}
              <span className="nexus-chat-author shrink-0 text-sm font-bold text-(--text-strong)">
                {currentAgentName || "协作成员"}
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

              {assistantHeaderAction ? (
                <div className="shrink-0">{assistantHeaderAction}</div>
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
                    on_permission_response={onPermissionResponse}
                    can_respond_to_permissions={canRespondToPermissions}
                    permission_read_only_reason={permissionReadOnlyReason}
                    on_open_workspace_file={onOpenWorkspaceFile}
                    workspace_agent_id={contentWorkspaceAgentId}
                    hidden_tool_names={hiddenToolNames}
                    show_timeline_dots
                  />
                  {pendingPermissionBlock}
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
                      artifacts={collapsedProcessFileArtifacts}
                      class_name="ml-5 pb-1"
                      label="生成文件"
                      on_open_workspace_file={onOpenWorkspaceFile}
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
                        on_permission_response={onPermissionResponse}
                        can_respond_to_permissions={canRespondToPermissions}
                        permission_read_only_reason={
                          permissionReadOnlyReason
                        }
                        on_open_workspace_file={onOpenWorkspaceFile}
                        workspace_agent_id={contentWorkspaceAgentId}
                        hidden_tool_names={hiddenToolNames}
                        class_name="ml-1"
                        show_timeline_dots
                      />

                      {pendingPermissionBlock}
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
                    on_open_workspace_file={onOpenWorkspaceFile}
                    workspace_agent_id={contentWorkspaceAgentId}
                  />
                </div>
              ) : null}

              {!state.should_render_direct_assistant_content &&
              !state.should_render_process_callchain ? (
                <div className="pt-2">{pendingPermissionBlock}</div>
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
