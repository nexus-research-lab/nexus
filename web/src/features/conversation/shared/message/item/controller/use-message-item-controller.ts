"use client";

import { useCallback, useEffect, useRef } from "react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";

import { hasTimedOutAskUserQuestion } from "../message-item-support";
import type { MessageItemProps, MessageItemState } from "../message-item-types";
import { useMessageItemStreamingLayout } from "../view/message-item-streaming-layout";
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
}: MessageItemControllerOptions): MessageItemState {
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

  const shouldRenderDirectAssistantContent =
    projection.directOrderedProjection.content.length > 0;
  const shouldRenderProcessCallchain =
    assistantContentMode === "dm_archived" &&
    (
      projection.processProjection.content.length > 0 ||
      projection.unmatchedPendingPermissions.length > 0
    );
  const shouldRenderAssistantText = hasDisplayContent(
    projection.finalAssistantContent,
  );
  const shouldRenderStandaloneActivityStatus = Boolean(
    projection.liveActivityState &&
    !shouldRenderDirectAssistantContent &&
    !shouldRenderProcessCallchain &&
    !shouldRenderAssistantText,
  );
  const shouldHideAssistantContent = !hasAssistantSurfaceContent({
    hasDirectContent: shouldRenderDirectAssistantContent,
    hasFinalContent: shouldRenderAssistantText,
    hasLiveActivity: Boolean(projection.liveActivityState),
    hasPendingPermission: projection.unmatchedPendingPermissions.length > 0,
    hasProcessContent: projection.processProjection.content.length > 0,
    hasResultSummary: Boolean(projection.resultSummary),
    streamStatus: projection.streamStatus,
  });
  const showCursor = Boolean(
    isLastRound &&
    isLoading &&
    (
      projection.streamingBlockIndexes.size > 0 ||
      projection.assistantMessages.length > 0 ||
      pendingPermissions.length > 0 ||
      STOPPABLE_STREAM_STATUSES.has(projection.streamStatus ?? "")
    ),
  );
  const finalAssistantIsStreaming = Boolean(
    showCursor &&
    typeof projection.finalAssistantContent !== "string" &&
    projection.finalAssistantStreamingIndexes.size > 0,
  );
  const canCopyAssistant = Boolean(projection.finalAssistantText.trim());
  const shouldShowAssistantFooter =
    (assistantContentMode === "dm_archived" || assistantContentMode === "room_result") &&
    (Boolean(projection.stats) || (!isLoading && canCopyAssistant));
  const canStopMessage = Boolean(
    onStopMessage && STOPPABLE_STREAM_STATUSES.has(projection.streamStatus ?? ""),
  );
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
    showCursor,
  });

  return {
    copiedUser,
    copiedAssistant,
    userMessage: projection.userMessage,
    userContent: projection.userContent,
    userAttachments: projection.userAttachments,
    assistantAgentId: projection.assistantAgentId,
    model: projection.model,
    timestamp: projection.timestamp,
    streamStatus: projection.streamStatus,
    stopReason: projection.stopReason,
    stats: projection.stats,
    matchedPendingPermissionsByToolUseId:
      projection.matchedPendingPermissionsByToolUseId,
    unmatchedPendingPermissions: projection.unmatchedPendingPermissions,
    directOrderedProjection: projection.directOrderedProjection,
    processProjection: projection.processProjection,
    finalAssistantContent: projection.finalAssistantContent,
    finalAssistantStreamingIndexes: projection.finalAssistantStreamingIndexes,
    finalAssistantText: projection.finalAssistantText,
    shouldRenderDirectAssistantContent,
    shouldRenderProcessCallchain,
    shouldRenderAssistantText,
    shouldRenderStandaloneActivityStatus,
    shouldShowAssistantFooter,
    showCursor,
    finalAssistantIsStreaming,
    shouldHideAssistantContent,
    processSummary: projection.processSummary,
    liveActivityState: projection.liveActivityState,
    isProcessExpanded,
    toggleProcessExpanded,
    processAnchorRef,
    canCopyAssistant,
    canStopMessage,
    handleCopyUser,
    handleCopyAssistant,
    handleStopMessage,
    contentAreaRef,
    contentAreaStyle,
    mergedContentLength: projection.mergedContent.length,
  };
}

function hasDisplayContent(
  content: MessageItemState["finalAssistantContent"],
): boolean {
  return typeof content === "string"
    ? Boolean(content.trim())
    : Boolean(content?.length);
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
