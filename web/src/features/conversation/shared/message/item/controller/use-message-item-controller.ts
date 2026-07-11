import { useCallback, useEffect, useRef } from "react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import type { ContentBlock } from "@/types/conversation/message";

import type { MessageItemProps } from "../message-item-types";
import { useMessageItemStreamingLayout } from "../view/message-item-streaming-layout";
import { hasTimedOutAskUserQuestion } from "./message-item-activity";
import { useMessageItemProjection } from "./use-message-item-projection";

const ACTIVE_STREAM_STATUSES = new Set(["pending", "streaming", "cancelled", "error"]);
const STOPPABLE_STREAM_STATUSES = new Set(["pending", "streaming"]);

type MessageItemControllerOptions = Pick<
  MessageItemProps,
  | "assistantContentMode"
  | "defaultProcessExpanded"
  | "hiddenToolNames"
  | "isLastRound"
  | "isLoading"
  | "messages"
  | "onStopMessage"
  | "pendingPermissions"
  | "roundId"
  | "runtimePhase"
>;

export function useMessageItemController({
  assistantContentMode = "dm_archived",
  defaultProcessExpanded = false,
  hiddenToolNames = [],
  isLastRound,
  isLoading,
  messages,
  onStopMessage,
  pendingPermissions = [],
  roundId,
  runtimePhase,
}: MessageItemControllerOptions) {
  const { copied: copiedUser, copy: copyUser } = useCopyToClipboard();
  const { copied: copiedAssistant, copy: copyAssistant } = useCopyToClipboard();
  const {
    isOpen: isProcessExpanded,
    toggle: toggleProcessExpanded,
    setOpen: setIsProcessExpanded,
    anchorRef: processAnchorRef,
  } = useScrollAnchoredState(defaultProcessExpanded);
  const projection = useMessageItemProjection({
    assistantContentMode,
    hiddenToolNames,
    isLastRound,
    isLoading,
    messages,
    pendingPermissions,
    roundId,
    runtimePhase,
  });
  const display = resolveAssistantDisplayState({
    assistantContentMode,
    hasStopHandler: Boolean(onStopMessage),
    isLastRound: Boolean(isLastRound),
    isLoading: Boolean(isLoading),
    pendingPermissionCount: pendingPermissions.length,
    projection,
  });
  const hasTimedOutQuestion = hasTimedOutAskUserQuestion(
    projection.processProjection.content,
  );

  useProcessExpansionLifecycle({
    assistantContentMode,
    hasPendingPermissions: pendingPermissions.length > 0,
    hasTimedOutQuestion,
    setIsProcessExpanded,
  });

  const handleCopyUser = useCallback(async () => {
    if (projection.userContent) {
      await copyUser(projection.userContent);
    }
  }, [copyUser, projection.userContent]);
  const handleCopyAssistant = useCallback(async () => {
    if (projection.finalAssistantText) {
      await copyAssistant(projection.finalAssistantText);
    }
  }, [copyAssistant, projection.finalAssistantText]);
  const handleStopMessage = useCallback(() => {
    if (onStopMessage && projection.firstAssistantMessageId) {
      onStopMessage(projection.firstAssistantMessageId);
    }
  }, [onStopMessage, projection.firstAssistantMessageId]);
  const { contentAreaRef, contentAreaStyle } = useMessageItemStreamingLayout({
    assistantContentMode,
    directContent: projection.directOrderedProjection.content,
    finalAssistantText: projection.finalAssistantText,
    showCursor: display.showCursor,
  });

  return {
    user: {
      attachments: projection.userAttachments,
      content: projection.userContent,
      copied: copiedUser,
      copy: handleCopyUser,
      message: projection.userMessage,
    },
    assistant: {
      hidden: display.hidden,
      header: {
        agentId: projection.assistantAgentId,
        canStop: display.canStop,
        model: projection.model,
        stop: handleStopMessage,
        timestamp: projection.timestamp,
      },
      permissions: {
        matchedByToolUseId: projection.matchedPendingPermissionsByToolUseId,
        unmatched: projection.unmatchedPendingPermissions,
      },
      direct: {
        projection: projection.directOrderedProjection,
        visible: display.directVisible,
      },
      process: {
        anchorRef: processAnchorRef,
        expanded: isProcessExpanded,
        projection: projection.processProjection,
        summary: projection.processSummary,
        toggle: toggleProcessExpanded,
        visible: display.processVisible,
      },
      final: {
        content: projection.finalAssistantContent,
        isStreaming: display.finalStreaming,
        streamingIndexes: projection.finalAssistantStreamingIndexes,
        visible: display.finalVisible,
      },
      activity: {
        emptyStreamStatus: display.emptyStreamStatus,
        showCursor: display.showCursor,
        standalone: display.standaloneActivity,
        state: projection.liveActivityState,
      },
      footer: {
        copied: copiedAssistant,
        onCopy: display.canCopy ? handleCopyAssistant : undefined,
        stats: projection.stats,
        visible: display.footerVisible,
      },
      layout: {
        contentAreaRef,
        contentAreaStyle,
      },
      showMaxTokensWarning: projection.stopReason === "max_tokens",
    },
  };
}

function resolveAssistantDisplayState({
  assistantContentMode,
  hasStopHandler,
  isLastRound,
  isLoading,
  pendingPermissionCount,
  projection,
}: {
  assistantContentMode: NonNullable<MessageItemProps["assistantContentMode"]>;
  hasStopHandler: boolean;
  isLastRound: boolean;
  isLoading: boolean;
  pendingPermissionCount: number;
  projection: ReturnType<typeof useMessageItemProjection>;
}) {
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
      projection.finalAssistantStreamingIndexes.size > 0,
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
      !finalVisible,
    ),
  };
}

function hasDisplayContent(
  content: string | ContentBlock[] | null,
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

function useProcessExpansionLifecycle({
  assistantContentMode,
  hasPendingPermissions,
  hasTimedOutQuestion,
  setIsProcessExpanded,
}: {
  assistantContentMode: NonNullable<MessageItemProps["assistantContentMode"]>;
  hasPendingPermissions: boolean;
  hasTimedOutQuestion: boolean;
  setIsProcessExpanded: (isOpen: boolean) => void;
}) {
  const wasLiveModeRef = useRef(assistantContentMode === "dm_live");

  useEffect(() => {
    if (hasPendingPermissions || hasTimedOutQuestion) {
      setIsProcessExpanded(true);
    }
  }, [hasPendingPermissions, hasTimedOutQuestion, setIsProcessExpanded]);

  useEffect(() => {
    const isLiveMode = assistantContentMode === "dm_live";
    if (wasLiveModeRef.current && !isLiveMode) {
      setIsProcessExpanded(false);
    }
    wasLiveModeRef.current = isLiveMode;
  }, [assistantContentMode, setIsProcessExpanded]);
}
