import type { ContentBlock } from "@/types/conversation/message/content";

import type { AssistantContentMode } from "../../message-item-projection";

const ACTIVE_STREAM_STATUSES = new Set([
  "pending",
  "streaming",
  "cancelled",
  "error",
]);
const STOPPABLE_STREAM_STATUSES = new Set(["pending", "streaming"]);

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
  const processVisible = assistantContentMode === "dm_archived" && (
    projection.processProjection.content.length > 0 ||
    projection.unmatchedPendingPermissions.length > 0
  );
  const finalVisible = hasDisplayContent(projection.finalAssistantContent);
  const showCursor = isLastRound && isLoading && [
    projection.streamingBlockIndexes.size > 0,
    projection.assistantMessages.length > 0,
    pendingPermissionCount > 0,
    STOPPABLE_STREAM_STATUSES.has(projection.streamStatus ?? ""),
  ].some(Boolean);
  const canCopy = Boolean(projection.finalAssistantText.trim());

  return {
    canCopy,
    canStop:
      hasStopHandler &&
      STOPPABLE_STREAM_STATUSES.has(projection.streamStatus ?? ""),
    directVisible,
    emptyStreamStatus: resolveEmptyStreamStatus(
      projection.mergedContent.length,
      projection.streamStatus,
    ),
    finalStreaming: Boolean(
      showCursor &&
      typeof projection.finalAssistantContent !== "string" &&
      projection.finalAssistantStreamingIndexes.size > 0
    ),
    finalVisible,
    footerVisible:
      (assistantContentMode === "dm_archived" ||
        assistantContentMode === "room_result") &&
      (Boolean(projection.stats) || (!isLoading && canCopy)),
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
    standaloneActivity: Boolean(
      projection.liveActivityState &&
      !directVisible &&
      !processVisible &&
      !finalVisible
    ),
  };
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
  return streamStatus === "cancelled" || streamStatus === "error"
    ? streamStatus
    : null;
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
    ACTIVE_STREAM_STATUSES.has(streamStatus ?? ""),
  ].some(Boolean);
}
