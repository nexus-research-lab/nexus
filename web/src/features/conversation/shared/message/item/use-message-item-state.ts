/**
 * =====================================================
 * @File   ：use-message-item-state.ts
 * @Date   ：2026-04-16 15:54
 * @Author ：leemysw
 * 2026-04-16 15:54   Create
 * =====================================================
 */

"use client";

import { useCallback, useEffect, useMemo } from "react";

import { useAssistantContentMerge } from "@/hooks/conversation/use-assistant-content-merge";
import { useScrollAnchoredState } from "@/hooks/conversation/use-scroll-anchored-state";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import {
  getSystemMessageDisplayMeta,
  type AssistantMessage,
  type SystemEventContent,
  type SystemMessage,
} from "@/types/conversation/message";

import type {
  MessageItemProps,
  MessageItemState,
} from "./message-item-types";
import {
  buildProcessSummary,
  resolveLiveActivityState,
} from "./message-item-activity";
import {
  buildVisibleAssistantTurns,
  buildVisibleOrderedAssistantEntries,
} from "./message-item-ordering";
import { resolveMessageItemPermissions } from "./message-item-permissions";
import { buildMessageStats } from "./message-item-stats";
import { resolveMessageItemFinalProjection } from "./message-item-final-projection";
import {
  hasTimedOutAskUserQuestion,
  type AssistantTurnEntry,
  type ContentProjection,
  type OrderedAssistantEntry,
} from "./message-item-support";
import { useMessageItemStreamingLayout } from "./message-item-streaming-layout";

export function useMessageItemState({
  isLastRound: isLastRound,
  isLoading: isLoading,
  runtimePhase: runtimePhase,
  messages,
  pendingPermissions: pendingPermissions = [],
  hiddenToolNames: hiddenToolNames = ["TodoWrite"],
  onStopMessage: onStopMessage,
  roundId: roundId,
  defaultProcessExpanded: defaultProcessExpanded = false,
  assistantContentMode: assistantContentMode = "dm_archived",
}: MessageItemProps): MessageItemState {
  const { copied: copiedUser, copy: copyUser } = useCopyToClipboard();
  const { copied: copiedAssistant, copy: copyAssistant } = useCopyToClipboard();
  const {
    isOpen: isProcessExpanded,
    toggle: toggleProcessExpanded,
    setOpen: setIsProcessExpanded,
    anchorRef: processAnchorRef,
  } = useScrollAnchoredState(defaultProcessExpanded);

  const {
    userMessage: userMessage,
    assistantMessages: assistantMessages,
    resultSummary: resultSummary,
    mergedContent: mergedContent,
    mergedContentSourceMessageIds: mergedContentSourceMessageIds,
    streamingBlockIndexes: streamingBlockIndexes,
  } = useAssistantContentMerge({
    messages,
    isLastRound,
    isLoading,
  });

  const systemMessages = useMemo(() => {
    return messages.filter(
      (message): message is SystemMessage =>
        message.role === "system" &&
        typeof message.content === "string" &&
        Boolean(message.content.trim()) &&
        (
          (isLastRound && isLoading) ||
          message.metadata?.subtype === "guided_input"
        ),
    );
  }, [isLastRound, isLoading, messages]);
  const systemEventBlocks = useMemo<SystemEventContent[]>(
    () =>
      systemMessages.map((message) => {
        const displayMeta = getSystemMessageDisplayMeta(message);
        return {
          type: "system_event",
          content: message.content,
          label: displayMeta.label,
          tone: displayMeta.tone,
          icon: displayMeta.icon,
          source_message_id: message.message_id,
          timestamp: message.timestamp,
          subtype: message.metadata?.subtype,
          tool_use_id:
            typeof message.metadata?.tool_use_id === "string"
              ? message.metadata.tool_use_id
              : null,
        };
      }),
    [systemMessages],
  );
  const sourceMessageOrderById = useMemo(() => {
    const nextOrder = new Map<string, number>();
    messages.forEach((message, index) => {
      nextOrder.set(message.message_id, index);
    });
    return nextOrder;
  }, [messages]);

  const firstAssistant = assistantMessages[0] as AssistantMessage | undefined;
  const assistantAgentId = firstAssistant?.agent_id ?? null;
  const model = firstAssistant?.model;
  const timestamp =
    firstAssistant?.timestamp ||
    systemEventBlocks[0]?.timestamp ||
    resultSummary?.timestamp;

  const streamStatus = useMemo(() => {
    return firstAssistant?.stream_status ?? null;
  }, [firstAssistant]);

  const stopReason = useMemo(() => {
    return firstAssistant?.stop_reason ?? null;
  }, [firstAssistant]);

  const stats = useMemo(
    () => buildMessageStats(resultSummary),
    [resultSummary],
  );

  const userContent = useMemo(() => {
    if (!userMessage || userMessage.role !== "user") {
      return "";
    }
    return typeof userMessage.content === "string" ? userMessage.content : "";
  }, [userMessage]);
  const userAttachments = useMemo(() => {
    if (!userMessage || userMessage.role !== "user") {
      return [];
    }
    return userMessage.attachments ?? [];
  }, [userMessage]);

  const {
    matchedPendingPermissionsByToolUseId,
    unmatchedPendingPermissions,
  } = useMemo(
    () => resolveMessageItemPermissions(messages, pendingPermissions),
    [messages, pendingPermissions],
  );

  const hiddenToolUseIds = useMemo(() => {
    const nextIds = new Set<string>();
    for (const block of mergedContent) {
      if (block.type === "tool_use" && hiddenToolNames.includes(block.name)) {
        nextIds.add(block.id);
      }
    }
    return nextIds;
  }, [hiddenToolNames, mergedContent]);

  const visibleOrderedAssistantEntries = useMemo<
    OrderedAssistantEntry[]
  >(
    () => buildVisibleOrderedAssistantEntries({
      hiddenToolNames,
      hiddenToolUseIds,
      isLoading,
      mergedContent,
      mergedContentSourceMessageIds,
      sourceMessageOrderById,
      systemEventBlocks,
    }),
    [
      hiddenToolNames,
      hiddenToolUseIds,
      isLoading,
      mergedContent,
      mergedContentSourceMessageIds,
      sourceMessageOrderById,
      systemEventBlocks,
    ],
  );

  const visibleOrderedAssistantContent = useMemo(() => {
    return visibleOrderedAssistantEntries.map((entry) => entry.block);
  }, [visibleOrderedAssistantEntries]);

  const orderedAssistantStreamingIndexes = useMemo(() => {
    const nextIndexes = new Set<number>();

    visibleOrderedAssistantEntries.forEach((entry, visibleIndex) => {
      if (streamingBlockIndexes.has(entry.mergedIndex)) {
        nextIndexes.add(visibleIndex);
      }
    });

    return nextIndexes;
  }, [streamingBlockIndexes, visibleOrderedAssistantEntries]);

  const visibleAssistantTurns = useMemo<AssistantTurnEntry[]>(
    () => buildVisibleAssistantTurns({
      assistantMessages,
      streamingBlockIndexes,
      visibleOrderedAssistantEntries,
    }),
    [
      assistantMessages,
      streamingBlockIndexes,
      visibleOrderedAssistantEntries,
    ],
  );

  const orderedProjection = useMemo<ContentProjection>(
    () => ({
      content: visibleOrderedAssistantContent,
      streamingIndexes: orderedAssistantStreamingIndexes,
    }),
    [orderedAssistantStreamingIndexes, visibleOrderedAssistantContent],
  );

  const {
    directOrderedProjection,
    processProjection,
    finalAssistantContent,
    finalAssistantStreamingIndexes,
    finalAssistantText,
  } = useMemo(
    () =>
      resolveMessageItemFinalProjection({
        assistantContentMode,
        assistantMessages,
        orderedProjection,
        resultSummary,
        roundId,
        streamingBlockIndexes,
        visibleAssistantTurns,
        visibleOrderedAssistantEntries,
      }),
    [
      assistantContentMode,
      assistantMessages,
      orderedProjection,
      resultSummary,
      roundId,
      streamingBlockIndexes,
      visibleAssistantTurns,
      visibleOrderedAssistantEntries,
    ],
  );

  const shouldRenderDirectAssistantContent =
    directOrderedProjection.content.length > 0;
  const hasVisibleProcess =
    processProjection.content.length > 0 ||
    unmatchedPendingPermissions.length > 0;
  const shouldRenderProcessCallchain =
    assistantContentMode === "dm_archived" && hasVisibleProcess;

  const hasTimedOutQuestionInProcess = useMemo(
    () => hasTimedOutAskUserQuestion(processProjection.content),
    [processProjection.content],
  );

  const processSummary = useMemo(
    () => buildProcessSummary({
      pendingPermissionCount: pendingPermissions.length,
      processContent: processProjection.content,
    }),
    [pendingPermissions.length, processProjection.content],
  );

  const liveActivityState = useMemo(
    () => resolveLiveActivityState({
      isLastRound,
      isLoading,
      mergedContent,
      pendingPermissions,
      runtimePhase,
      streamStatus,
      streamingBlockIndexes,
    }),
    [
      isLastRound,
      isLoading,
      mergedContent,
      pendingPermissions,
      runtimePhase,
      streamStatus,
      streamingBlockIndexes,
    ],
  );

  const shouldHideAssistantContent = useMemo(() => {
    if (liveActivityState) {
      return false;
    }
    if (unmatchedPendingPermissions.length > 0) {
      return false;
    }
    if (
      streamStatus === "pending" ||
      streamStatus === "streaming" ||
      streamStatus === "cancelled" ||
      streamStatus === "error"
    ) {
      return false;
    }
    if (directOrderedProjection.content.length > 0) {
      return false;
    }
    if (processProjection.content.length > 0) {
      return false;
    }
    if (typeof finalAssistantContent === "string") {
      return !finalAssistantContent.trim();
    }
    if (finalAssistantContent && finalAssistantContent.length > 0) {
      return false;
    }
    return !resultSummary;
  }, [
    directOrderedProjection.content.length,
    finalAssistantContent,
    liveActivityState,
    processProjection.content.length,
    resultSummary,
    streamStatus,
    unmatchedPendingPermissions.length,
  ]);

  const shouldRenderAssistantText = Boolean(
    typeof finalAssistantContent === "string"
      ? finalAssistantContent.trim()
      : finalAssistantContent?.length,
  );

  const shouldRenderStandaloneActivityStatus = Boolean(
    liveActivityState &&
    !shouldRenderDirectAssistantContent &&
    !shouldRenderProcessCallchain &&
    !shouldRenderAssistantText,
  );

  useEffect(() => {
    if (pendingPermissions.length > 0) {
      setIsProcessExpanded(true);
    }
  }, [pendingPermissions.length, setIsProcessExpanded]);

  useEffect(() => {
    if (hasTimedOutQuestionInProcess) {
      setIsProcessExpanded(true);
    }
  }, [hasTimedOutQuestionInProcess, setIsProcessExpanded]);

  const handleCopyUser = useCallback(async () => {
    if (!userContent) {
      return;
    }
    await copyUser(userContent);
  }, [copyUser, userContent]);

  const handleCopyAssistant = useCallback(async () => {
    if (!finalAssistantText) {
      return;
    }
    await copyAssistant(finalAssistantText);
  }, [copyAssistant, finalAssistantText]);

  const showCursor = Boolean(
    isLastRound &&
    isLoading &&
    (streamingBlockIndexes.size > 0 ||
      assistantMessages.length > 0 ||
      pendingPermissions.length > 0 ||
      streamStatus === "pending" ||
      streamStatus === "streaming"),
  );

  const finalAssistantIsStreaming = Boolean(
    showCursor &&
    typeof finalAssistantContent !== "string" &&
    finalAssistantStreamingIndexes.size > 0,
  );

  const canCopyAssistant = Boolean(finalAssistantText.trim());
  const shouldShowAssistantFooter =
    (assistantContentMode === "dm_archived" ||
      assistantContentMode === "room_result") &&
    (Boolean(stats) || (!isLoading && canCopyAssistant));

  const canStopMessage = Boolean(
    onStopMessage &&
    (streamStatus === "pending" || streamStatus === "streaming"),
  );
  const handleStopMessage = useCallback(() => {
    if (!onStopMessage || !firstAssistant) {
      return;
    }
    onStopMessage(firstAssistant.message_id);
  }, [firstAssistant, onStopMessage]);

  const { contentAreaRef, contentAreaStyle } =
    useMessageItemStreamingLayout({
      assistantContentMode,
      directContent: directOrderedProjection.content,
      finalAssistantText,
      showCursor,
    });

  return {
    copiedUser,
    copiedAssistant,
    userMessage,
    userContent,
    userAttachments,
    assistantAgentId,
    model,
    timestamp,
    streamStatus,
    stopReason,
    stats,
    matchedPendingPermissionsByToolUseId,
    unmatchedPendingPermissions,
    directOrderedProjection,
    processProjection,
    finalAssistantContent,
    finalAssistantStreamingIndexes,
    finalAssistantText,
    shouldRenderDirectAssistantContent,
    shouldRenderProcessCallchain,
    shouldRenderAssistantText,
    shouldRenderStandaloneActivityStatus,
    shouldShowAssistantFooter,
    showCursor,
    finalAssistantIsStreaming,
    shouldHideAssistantContent,
    processSummary,
    liveActivityState,
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
    mergedContentLength: mergedContent.length,
  };
}
