import type { ContentBlock } from "@/types/conversation/message/content";

import type { AssistantContentMode } from "../../message-item-projection";

const ACTIVE_STREAM_STATUSES = new Set([
  "pending",
  "streaming",
  "cancelled",
  "error",
]);
const STOPPABLE_STREAM_STATUSES = new Set(["pending", "streaming"]);
const PROCESS_CONTENT_MODES = new Set<AssistantContentMode>(["dm_archived"]);
const FOOTER_MODES = new Set<AssistantContentMode>([
  "dm_archived",
  "room_result",
]);

interface AssistantDisplayProjection {
  assistantMessages: readonly unknown[];
  directOrderedProjection: {
    content: readonly ContentBlock[];
  };
  finalAssistantContent: string | readonly ContentBlock[] | null;
  finalAssistantStreamingIndexes: ReadonlySet<number>;
  finalAssistantText: string;
  liveActivityState: unknown | null;
  mergedContent: readonly ContentBlock[];
  processProjection: {
    content: readonly ContentBlock[];
  };
  resultSummary: unknown;
  stats: unknown;
  streamStatus: string | null;
  streamingBlockIndexes: ReadonlySet<number>;
  unmatchedPendingPermissions: readonly unknown[];
}

interface ResolveAssistantDisplayStateOptions {
  assistantContentMode: AssistantContentMode;
  hasStopHandler: boolean;
  isLastRound: boolean;
  isLoading: boolean;
  pendingPermissionCount: number;
  projection: AssistantDisplayProjection;
}

export function resolveAssistantDisplayState({
  assistantContentMode,
  hasStopHandler,
  isLastRound,
  isLoading,
  pendingPermissionCount,
  projection,
}: ResolveAssistantDisplayStateOptions) {
  const directVisible = projection.directOrderedProjection.content.length > 0;
  const processVisible = hasVisibleProcessContent(
    assistantContentMode,
    projection.processProjection.content.length,
  );
  const finalVisible = hasDisplayContent(projection.finalAssistantContent);
  const showCursor = resolveShowCursor({
    hasAssistantMessages: projection.assistantMessages.length > 0,
    hasPendingPermissions: pendingPermissionCount > 0,
    hasStreamingBlocks: projection.streamingBlockIndexes.size > 0,
    isLastRound,
    isLoading,
    streamStatus: projection.streamStatus,
  });
  const canCopy = Boolean(projection.finalAssistantText.trim());

  return {
    canCopy,
    canStop: resolveCanStop(hasStopHandler, projection.streamStatus),
    directVisible,
    emptyStreamStatus: resolveEmptyStreamStatus(
      projection.mergedContent.length,
      projection.streamStatus,
    ),
    finalStreaming: resolveFinalStreaming(
      projection.finalAssistantContent,
      projection.finalAssistantStreamingIndexes.size,
      showCursor,
    ),
    finalVisible,
    footerVisible: resolveFooterVisible({
      canCopy,
      hasStats: Boolean(projection.stats),
      isLoading,
      mode: assistantContentMode,
    }),
    hidden: !hasAssistantSurfaceContent({
      hasDirectContent: directVisible,
      hasFinalContent: finalVisible,
      hasLiveActivity: Boolean(projection.liveActivityState),
      hasPendingPermission: projection.unmatchedPendingPermissions.length > 0,
      hasProcessContent: projection.processProjection.content.length > 0,
      hasResultSummary: Boolean(projection.resultSummary),
      streamStatus: projection.streamStatus,
    }),
    processVisible,
    showCursor,
    standaloneActivity: resolveStandaloneActivity(
      Boolean(projection.liveActivityState),
      directVisible,
      processVisible,
      finalVisible,
    ),
  };
}

// 未匹配权限拥有独立内容段，过程可见性只表达真实过程内容。
function hasVisibleProcessContent(
  mode: AssistantContentMode,
  contentLength: number,
): boolean {
  return [PROCESS_CONTENT_MODES.has(mode), contentLength > 0].every(Boolean);
}

interface ShowCursorOptions {
  hasAssistantMessages: boolean;
  hasPendingPermissions: boolean;
  hasStreamingBlocks: boolean;
  isLastRound: boolean;
  isLoading: boolean;
  streamStatus: string | null;
}

function resolveShowCursor({
  hasAssistantMessages,
  hasPendingPermissions,
  hasStreamingBlocks,
  isLastRound,
  isLoading,
  streamStatus,
}: ShowCursorOptions): boolean {
  const isActiveRound = [isLastRound, isLoading].every(Boolean);
  const hasLiveContent = [
    hasStreamingBlocks,
    hasAssistantMessages,
    hasPendingPermissions,
    hasStreamStatus(STOPPABLE_STREAM_STATUSES, streamStatus),
  ].some(Boolean);
  return [isActiveRound, hasLiveContent].every(Boolean);
}

function resolveCanStop(
  hasStopHandler: boolean,
  streamStatus: string | null,
): boolean {
  return [
    hasStopHandler,
    hasStreamStatus(STOPPABLE_STREAM_STATUSES, streamStatus),
  ].every(Boolean);
}

function resolveFinalStreaming(
  content: string | readonly ContentBlock[] | null,
  streamingBlockCount: number,
  showCursor: boolean,
): boolean {
  return [
    showCursor,
    typeof content !== "string",
    streamingBlockCount > 0,
  ].every(Boolean);
}

function resolveFooterVisible({
  canCopy,
  hasStats,
  isLoading,
  mode,
}: {
  canCopy: boolean;
  hasStats: boolean;
  isLoading: boolean;
  mode: AssistantContentMode;
}): boolean {
  const hasCompletedCopy = [!isLoading, canCopy].every(Boolean);
  const hasFooterContent = [hasStats, hasCompletedCopy].some(Boolean);
  return [FOOTER_MODES.has(mode), hasFooterContent].every(Boolean);
}

function resolveStandaloneActivity(
  hasLiveActivity: boolean,
  directVisible: boolean,
  processVisible: boolean,
  finalVisible: boolean,
): boolean {
  return [
    hasLiveActivity,
    !directVisible,
    !processVisible,
    !finalVisible,
  ].every(Boolean);
}

function hasDisplayContent(
  content: string | readonly ContentBlock[] | null,
): boolean {
  return typeof content === "string"
    ? Boolean(content.trim())
    : Boolean(content?.length);
}

function resolveEmptyStreamStatus(
  contentLength: number,
  streamStatus: string | null,
): "cancelled" | "error" | null {
  if (contentLength !== 0) {
    return null;
  }
  return isEmptyStreamStatus(streamStatus) ? streamStatus : null;
}

function isEmptyStreamStatus(
  streamStatus: string | null,
): streamStatus is "cancelled" | "error" {
  return streamStatus === "cancelled" || streamStatus === "error";
}

function hasAssistantSurfaceContent({
  hasDirectContent,
  hasFinalContent,
  hasLiveActivity,
  hasPendingPermission,
  hasProcessContent,
  hasResultSummary,
  streamStatus,
}: {
  hasDirectContent: boolean;
  hasFinalContent: boolean;
  hasLiveActivity: boolean;
  hasPendingPermission: boolean;
  hasProcessContent: boolean;
  hasResultSummary: boolean;
  streamStatus: string | null;
}): boolean {
  return [
    hasDirectContent,
    hasFinalContent,
    hasLiveActivity,
    hasPendingPermission,
    hasProcessContent,
    hasResultSummary,
    hasStreamStatus(ACTIVE_STREAM_STATUSES, streamStatus),
  ].some(Boolean);
}

function hasStreamStatus(
  statuses: ReadonlySet<string>,
  streamStatus: string | null,
): boolean {
  return statuses.has(streamStatus ?? "");
}
