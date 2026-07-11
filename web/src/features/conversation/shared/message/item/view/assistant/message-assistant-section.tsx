"use client";

import { useCallback, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ContentBlock } from "@/types/conversation/message";
import type { PermissionDecisionPayload } from "@/types/conversation/permission";

import { useWorkspaceFileArtifactsFromContent } from "../../../blocks/artifact/workspace-file-artifact-utils";
import { MessageStats } from "../../../ui/message-stats";
import { MessageActivityStatus } from "../../../ui/message-primitives";
import type { MessageItemState } from "../../message-item-types";
import { ContentRenderer } from "../content/content-renderer";
import {
  AssistantMessageAvatar,
  AssistantMessageHeader,
} from "./assistant-message-header";
import { AssistantProcessCallchain } from "./assistant-process-callchain";
import { PendingPermissionList } from "./pending-permission-list";

const EMPTY_CONTENT_BLOCKS: ContentBlock[] = [];

interface MessageAssistantSectionProps {
  assistantContentMode:
    | "dm_live"
    | "dm_archived"
    | "room_thread"
    | "room_result";
  assistantHeaderAction?: ReactNode;
  canRespondToPermissions: boolean;
  compact: boolean;
  currentAgentAvatar?: string | null;
  currentAgentName?: string | null;
  hiddenToolNames?: string[];
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissionReadOnlyReason?: string;
  state: MessageItemState;
  workspaceAgentId?: string | null;
}

export function MessageAssistantSection({
  assistantContentMode,
  assistantHeaderAction,
  canRespondToPermissions,
  compact,
  currentAgentAvatar,
  currentAgentName,
  hiddenToolNames = ["TodoWrite"],
  onOpenAgentContact,
  onOpenWorkspaceFile,
  onPermissionResponse,
  permissionReadOnlyReason,
  state,
  workspaceAgentId,
}: MessageAssistantSectionProps) {
  const { t } = useI18n();
  const contentWorkspaceAgentId = state.assistantAgentId ?? workspaceAgentId;
  const avatarAgentId = state.assistantAgentId ?? workspaceAgentId ?? null;
  const collapsedProcessFileArtifacts = useWorkspaceFileArtifactsFromContent(
    state.shouldRenderProcessCallchain && !state.isProcessExpanded
      ? state.processProjection.content
      : EMPTY_CONTENT_BLOCKS,
  );
  const handleOpenAgentContact = useCallback(() => {
    if (avatarAgentId) {
      onOpenAgentContact?.(avatarAgentId);
    }
  }, [avatarAgentId, onOpenAgentContact]);

  if (state.shouldHideAssistantContent) {
    return null;
  }

  const pendingPermissionBlock = (
    <PendingPermissionList
      canRespond={canRespondToPermissions}
      isRoomThreadMode={assistantContentMode === "room_thread"}
      onResponse={onPermissionResponse}
      permissions={state.unmatchedPendingPermissions}
      readOnlyReason={permissionReadOnlyReason}
      workspaceAgentId={contentWorkspaceAgentId}
    />
  );
  const canOpenContact = Boolean(avatarAgentId && onOpenAgentContact);

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
            <AssistantMessageAvatar
              avatarUrl={currentAgentAvatar}
              displayName={currentAgentName || "协作成员"}
              onOpenContact={canOpenContact ? handleOpenAgentContact : undefined}
            />
          ) : null}

          <div className="relative min-w-0">
            <AssistantMessageHeader
              avatarUrl={currentAgentAvatar}
              canOpenContact={canOpenContact}
              canStop={state.canStopMessage}
              compact={compact}
              headerAction={assistantHeaderAction}
              model={state.model}
              name={currentAgentName}
              onOpenContact={handleOpenAgentContact}
              onStop={state.handleStopMessage}
              timestamp={state.timestamp}
            />

            <div
              className={cn(
                "nexus-chat-message-content min-w-0 max-w-full overflow-x-hidden pb-2 pt-1 text-left",
                compact ? "text-[15px] leading-6" : "text-[16px] leading-7",
              )}
              ref={state.contentAreaRef}
              style={state.contentAreaStyle}
            >
              {state.shouldRenderStandaloneActivityStatus ? (
                <MessageActivityStatus
                  className="py-1"
                  state={state.liveActivityState!}
                />
              ) : null}
              <EmptyStreamStatus
                contentLength={state.mergedContentLength}
                streamStatus={state.streamStatus}
              />

              {state.shouldRenderDirectAssistantContent ? (
                <div>
                  <ContentRenderer
                    canRespondToPermissions={canRespondToPermissions}
                    content={state.directOrderedProjection.content}
                    fallbackActivityState={state.liveActivityState}
                    hiddenToolNames={hiddenToolNames}
                    isStreaming={state.showCursor}
                    onOpenWorkspaceFile={onOpenWorkspaceFile}
                    onPermissionResponse={onPermissionResponse}
                    pendingPermissionsByToolUseId={state.matchedPendingPermissionsByToolUseId}
                    permissionReadOnlyReason={permissionReadOnlyReason}
                    showTimelineDots
                    streamingBlockIndexes={state.directOrderedProjection.streamingIndexes}
                    workspaceAgentId={contentWorkspaceAgentId}
                  />
                  {pendingPermissionBlock}
                </div>
              ) : null}

              {state.shouldRenderProcessCallchain ? (
                <AssistantProcessCallchain
                  anchorRef={state.processAnchorRef}
                  canRespondToPermissions={canRespondToPermissions}
                  collapsedFileArtifacts={collapsedProcessFileArtifacts}
                  fallbackActivityState={state.liveActivityState}
                  hiddenToolNames={hiddenToolNames}
                  isExpanded={state.isProcessExpanded}
                  isStreaming={state.showCursor}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  onPermissionResponse={onPermissionResponse}
                  pendingPermissionBlock={pendingPermissionBlock}
                  pendingPermissionsByToolUseId={state.matchedPendingPermissionsByToolUseId}
                  permissionReadOnlyReason={permissionReadOnlyReason}
                  processProjection={state.processProjection}
                  summary={state.processSummary}
                  toggleExpanded={state.toggleProcessExpanded}
                  workspaceAgentId={contentWorkspaceAgentId}
                />
              ) : null}

              {state.shouldRenderAssistantText ? (
                <ContentRenderer
                  content={state.finalAssistantContent ?? []}
                  fallbackActivityState={state.liveActivityState}
                  isStreaming={state.finalAssistantIsStreaming}
                  onOpenWorkspaceFile={onOpenWorkspaceFile}
                  streamingBlockIndexes={state.finalAssistantStreamingIndexes}
                  workspaceAgentId={contentWorkspaceAgentId}
                />
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
                compact={compact}
                copiedAssistant={state.copiedAssistant}
                onCopyAssistant={state.canCopyAssistant
                  ? state.handleCopyAssistant
                  : undefined}
                showCursor={state.showCursor}
                stats={state.stats || undefined}
              />
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

function EmptyStreamStatus({
  contentLength,
  streamStatus,
}: {
  contentLength: number;
  streamStatus: MessageItemState["streamStatus"];
}) {
  if (contentLength !== 0) {
    return null;
  }
  const labels = {
    cancelled: <span className="text-xs italic text-(--text-soft)">已停止</span>,
    error: <span className="text-xs italic text-rose-500">执行失败</span>,
  };
  return streamStatus === "cancelled" || streamStatus === "error"
    ? labels[streamStatus]
    : null;
}
