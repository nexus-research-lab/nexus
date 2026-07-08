"use client";

import { type ReactNode, useCallback } from "react";
import {
  AlertTriangle,
  Bot,
  ChevronDown,
  ChevronRight,
  Square,
  Wrench,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
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
import { formatMessageTime } from "./message-item-support";
import type { MessageItemState } from "./message-item-types";
import type { ContentBlock } from "@/types/conversation/message";

const EMPTY_CONTENT_BLOCKS: ContentBlock[] = [];

interface PendingPermissionListProps {
  permissions: PendingPermission[];
  isRoomThreadMode: boolean;
  canRespondToPermissions: boolean;
  permissionReadOnlyReason?: string;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  workspaceAgentId?: string | null;
}

function PendingPermissionList({
  permissions,
  isRoomThreadMode: isRoomThreadMode,
  canRespondToPermissions: canRespondToPermissions,
  permissionReadOnlyReason: permissionReadOnlyReason,
  onPermissionResponse: onPermissionResponse,
  workspaceAgentId: workspaceAgentId,
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
          toolUse={{
            type: "tool_use",
            id: `pending_${permission.request_id}`,
            name: permission.tool_name,
            input: permission.tool_input,
          }}
          status="waiting_permission"
          permissionRequest={{
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
          interactionDisabled={!canRespondToPermissions}
          interactionDisabledReason={permissionReadOnlyReason}
          workspaceAgentId={workspaceAgentId}
        />
      ))}
    </div>
  );
}

interface MessageAssistantSectionProps {
  compact: boolean;
  currentAgentName?: string | null;
  currentAgentAvatar?: string | null;
  canRespondToPermissions: boolean;
  permissionReadOnlyReason?: string;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  workspaceAgentId?: string | null;
  hiddenToolNames?: string[];
  assistantHeaderAction?: ReactNode;
  assistantContentMode:
    | "dm_live"
    | "dm_archived"
    | "room_thread"
    | "room_result";
  state: MessageItemState;
}

export function MessageAssistantSection({
  compact,
  currentAgentName: currentAgentName,
  currentAgentAvatar: currentAgentAvatar,
  canRespondToPermissions: canRespondToPermissions,
  permissionReadOnlyReason: permissionReadOnlyReason,
  onPermissionResponse: onPermissionResponse,
  onOpenAgentContact: onOpenAgentContact,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  workspaceAgentId: workspaceAgentId,
  hiddenToolNames: hiddenToolNames = ["TodoWrite"],
  assistantHeaderAction: assistantHeaderAction,
  assistantContentMode: assistantContentMode,
  state,
}: MessageAssistantSectionProps) {
  const { t } = useI18n();
  const isRoomThreadMode = assistantContentMode === "room_thread";
  const contentWorkspaceAgentId = state.assistantAgentId ?? workspaceAgentId;
  const avatarAgentId = state.assistantAgentId ?? workspaceAgentId ?? null;
  const collapsedProcessFileArtifacts = useWorkspaceFileArtifactsFromContent(
    state.shouldRenderProcessCallchain && !state.isProcessExpanded
      ? state.processProjection.content
      : EMPTY_CONTENT_BLOCKS,
  );
  const handleOpenAgentContact = useCallback(() => {
    if (!avatarAgentId) {
      return;
    }
    onOpenAgentContact?.(avatarAgentId);
  }, [avatarAgentId, onOpenAgentContact]);

  if (state.shouldHideAssistantContent) {
    return null;
  }

  const pendingPermissionBlock = (
    <PendingPermissionList
      permissions={state.unmatchedPendingPermissions}
      isRoomThreadMode={isRoomThreadMode}
      canRespondToPermissions={canRespondToPermissions}
      permissionReadOnlyReason={permissionReadOnlyReason}
      onPermissionResponse={onPermissionResponse}
      workspaceAgentId={contentWorkspaceAgentId}
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
              ariaLabel={`打开 ${currentAgentName || "协作成员"} 的联络`}
              className="nexus-chat-avatar"
              avatarUrl={currentAgentAvatar}
              onClick={
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
                  ariaLabel={`打开 ${currentAgentName || "协作成员"} 的联络`}
                  className="nexus-chat-avatar shrink-0"
                  size="compact"
                  avatarUrl={currentAgentAvatar}
                  onClick={
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
                  {formatMessageTime(state.timestamp)}
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

              {state.canStopMessage ? (
                <MessageActionButton
                  type="button"
                  aria-label="停止生成"
                  onClick={state.handleStopMessage}
                  className="flex items-center gap-1 px-1.5 py-0.5 text-xs"
                  tone="default"
                >
                  <Square className="h-3 w-3 fill-current" />
                  <span>停止</span>
                </MessageActionButton>
              ) : null}
            </div>

            <div
              ref={state.contentAreaRef}
              className={cn(
                "nexus-chat-message-content min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-left",
                compact ? "text-[15px] leading-6" : "text-[16px] leading-7",
              )}
              style={state.contentAreaStyle}
            >
              {state.shouldRenderStandaloneActivityStatus ? (
                <MessageActivityStatus
                  className="py-1"
                  state={state.liveActivityState!}
                />
              ) : null}

              {state.streamStatus === "cancelled" &&
              state.mergedContentLength === 0 ? (
                <span className="text-xs italic text-(--text-soft)">
                  已停止
                </span>
              ) : null}

              {state.streamStatus === "error" &&
              state.mergedContentLength === 0 ? (
                <span className="text-xs italic text-rose-500">执行失败</span>
              ) : null}

              {state.shouldRenderDirectAssistantContent ? (
                <div>
                  <ContentRenderer
                    content={state.directOrderedProjection.content}
                    isStreaming={state.showCursor}
                    streamingBlockIndexes={
                      state.directOrderedProjection.streamingIndexes
                    }
                    fallbackActivityState={state.liveActivityState}
                    pendingPermissionsByToolUseId={
                      state.matchedPendingPermissionsByToolUseId
                    }
                    onPermissionResponse={onPermissionResponse}
                    canRespondToPermissions={canRespondToPermissions}
                    permissionReadOnlyReason={permissionReadOnlyReason}
                    onOpenWorkspaceFile={onOpenWorkspaceFile}
                    workspaceAgentId={contentWorkspaceAgentId}
                    hiddenToolNames={hiddenToolNames}
                    showTimelineDots
                  />
                  {pendingPermissionBlock}
                </div>
              ) : null}

              {state.shouldRenderProcessCallchain ? (
                <div
                  ref={
                    state.processAnchorRef as React.RefObject<HTMLDivElement>
                  }
                >
                  <button
                    className="flex w-full items-center gap-2 py-1.5 text-left text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)"
                    onClick={state.toggleProcessExpanded}
                    type="button"
                  >
                    <Wrench className="h-3 w-3 shrink-0 text-(--icon-muted)" />
                    <div className="min-w-0 flex-1 truncate text-[12px] font-medium text-(--text-muted)">
                      {state.processSummary}
                    </div>
                    <div className="text-(--icon-muted)">
                      {state.isProcessExpanded ? (
                        <ChevronDown className="h-3.5 w-3.5" />
                      ) : (
                        <ChevronRight className="h-3.5 w-3.5" />
                      )}
                    </div>
                  </button>

                  {!state.isProcessExpanded ? (
                    <WorkspaceFileArtifactList
                      artifacts={collapsedProcessFileArtifacts}
                      className="ml-5 pb-1"
                      label="生成文件"
                      onOpenWorkspaceFile={onOpenWorkspaceFile}
                    />
                  ) : null}

                  {state.isProcessExpanded ? (
                    <div className="pt-1">
                      <ContentRenderer
                        content={state.processProjection.content}
                        isStreaming={state.showCursor}
                        streamingBlockIndexes={
                          state.processProjection.streamingIndexes
                        }
                        fallbackActivityState={state.liveActivityState}
                        pendingPermissionsByToolUseId={
                          state.matchedPendingPermissionsByToolUseId
                        }
                        onPermissionResponse={onPermissionResponse}
                        canRespondToPermissions={canRespondToPermissions}
                        permissionReadOnlyReason={
                          permissionReadOnlyReason
                        }
                        onOpenWorkspaceFile={onOpenWorkspaceFile}
                        workspaceAgentId={contentWorkspaceAgentId}
                        hiddenToolNames={hiddenToolNames}
                        className="ml-1"
                        showTimelineDots
                      />

                      {pendingPermissionBlock}
                    </div>
                  ) : null}
                </div>
              ) : null}

              {state.shouldRenderAssistantText ? (
                <div className={cn(state.shouldRenderProcessCallchain)}>
                  <ContentRenderer
                    content={state.finalAssistantContent ?? []}
                    isStreaming={state.finalAssistantIsStreaming}
                    streamingBlockIndexes={
                      state.finalAssistantStreamingIndexes
                    }
                    fallbackActivityState={state.liveActivityState}
                    onOpenWorkspaceFile={onOpenWorkspaceFile}
                    workspaceAgentId={contentWorkspaceAgentId}
                  />
                </div>
              ) : null}

              {state.stopReason === "max_tokens" ? (
                <div className="mt-2 flex items-center gap-1.5 rounded-[8px] border border-[color:color-mix(in_srgb,var(--warning)_18%,transparent)] px-3 py-2 text-xs leading-5 text-(--warning)">
                  <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
                  <span>{t("message.max_tokens_warning")}</span>
                </div>
              ) : null}

              {!state.shouldRenderDirectAssistantContent &&
              !state.shouldRenderProcessCallchain ? (
                <div className="pt-2">{pendingPermissionBlock}</div>
              ) : null}
            </div>

            {state.shouldShowAssistantFooter ? (
              <MessageStats
                stats={state.stats || undefined}
                showCursor={state.showCursor}
                compact={compact}
                copiedAssistant={state.copiedAssistant}
                onCopyAssistant={
                  state.canCopyAssistant
                    ? state.handleCopyAssistant
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
