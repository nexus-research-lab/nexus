import { useCallback } from "react";

import { useScrollAnchoredState } from "@/features/conversation/shared/timeline/scroll/use-scroll-anchored-state";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";

import type { MessageItemProps } from "../message-item-types";
import { useMessageItemStreamingLayout } from "../view/message-item-streaming-layout";
import { resolveAssistantDisplayState } from "./display/message-item-display-model";
import { useProcessExpansionLifecycle } from "./display/use-process-expansion-lifecycle";
import { hasTimedOutAskUserQuestion } from "./message-item-activity";
import { useMessageItemProjection } from "./use-message-item-projection";

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
