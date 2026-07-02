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
  get_system_message_display_meta,
  type AssistantMessage,
  type SystemEventContent,
  type SystemMessage,
} from "@/types/conversation/message";

import type {
  MessageItemProps,
  MessageItemState,
} from "./message-item-types";
import {
  build_process_summary,
  resolve_live_activity_state,
} from "./message-item-activity";
import {
  build_visible_assistant_turns,
  build_visible_ordered_assistant_entries,
} from "./message-item-ordering";
import { resolve_message_item_permissions } from "./message-item-permissions";
import { build_message_stats } from "./message-item-stats";
import { resolve_message_item_final_projection } from "./message-item-final-projection";
import {
  has_timed_out_ask_user_question,
  type AssistantTurnEntry,
  type ContentProjection,
  type OrderedAssistantEntry,
} from "./message-item-support";
import { useMessageItemStreamingLayout } from "./message-item-streaming-layout";

export function useMessageItemState({
  is_last_round: isLastRound,
  is_loading: isLoading,
  runtime_phase: runtimePhase,
  messages,
  pending_permissions: pendingPermissions = [],
  hidden_tool_names: hiddenToolNames = ["TodoWrite"],
  on_stop_message: onStopMessage,
  round_id: roundId,
  default_process_expanded: defaultProcessExpanded = false,
  assistant_content_mode: assistantContentMode = "dm_archived",
}: MessageItemProps): MessageItemState {
  const { copied: copiedUser, copy: copyUser } = useCopyToClipboard();
  const { copied: copiedAssistant, copy: copyAssistant } = useCopyToClipboard();
  const {
    is_open: isProcessExpanded,
    toggle: toggleProcessExpanded,
    set_open: setIsProcessExpanded,
    anchor_ref: processAnchorRef,
  } = useScrollAnchoredState(defaultProcessExpanded);

  const {
    user_message: userMessage,
    assistant_messages: assistantMessages,
    result_summary: resultSummary,
    merged_content: mergedContent,
    merged_content_source_message_ids: mergedContentSourceMessageIds,
    streaming_block_indexes: streamingBlockIndexes,
  } = useAssistantContentMerge({
    messages,
    is_last_round: isLastRound,
    is_loading: isLoading,
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
        const displayMeta = get_system_message_display_meta(message);
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

  const stats = useMemo(
    () => build_message_stats(resultSummary),
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
    matched_pending_permissions_by_tool_use_id: matchedPendingPermissionsByToolUseId,
    unmatched_pending_permissions: unmatchedPendingPermissions,
  } = useMemo(
    () => resolve_message_item_permissions(messages, pendingPermissions),
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
    () => build_visible_ordered_assistant_entries({
      hidden_tool_names: hiddenToolNames,
      hidden_tool_use_ids: hiddenToolUseIds,
      is_loading: isLoading,
      merged_content: mergedContent,
      merged_content_source_message_ids: mergedContentSourceMessageIds,
      source_message_order_by_id: sourceMessageOrderById,
      system_event_blocks: systemEventBlocks,
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
      if (streamingBlockIndexes.has(entry.merged_index)) {
        nextIndexes.add(visibleIndex);
      }
    });

    return nextIndexes;
  }, [streamingBlockIndexes, visibleOrderedAssistantEntries]);

  const visibleAssistantTurns = useMemo<AssistantTurnEntry[]>(
    () => build_visible_assistant_turns({
      assistant_messages: assistantMessages,
      streaming_block_indexes: streamingBlockIndexes,
      visible_ordered_assistant_entries: visibleOrderedAssistantEntries,
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
      streaming_indexes: orderedAssistantStreamingIndexes,
    }),
    [orderedAssistantStreamingIndexes, visibleOrderedAssistantContent],
  );

  const {
    direct_ordered_projection: directOrderedProjection,
    process_projection: processProjection,
    final_assistant_content: finalAssistantContent,
    final_assistant_streaming_indexes: finalAssistantStreamingIndexes,
    final_assistant_text: finalAssistantText,
  } = useMemo(
    () =>
      resolve_message_item_final_projection({
        assistant_content_mode: assistantContentMode,
        assistant_messages: assistantMessages,
        ordered_projection: orderedProjection,
        result_summary: resultSummary,
        round_id: roundId,
        streaming_block_indexes: streamingBlockIndexes,
        visible_assistant_turns: visibleAssistantTurns,
        visible_ordered_assistant_entries: visibleOrderedAssistantEntries,
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
    () => has_timed_out_ask_user_question(processProjection.content),
    [processProjection.content],
  );

  const processSummary = useMemo(
    () => build_process_summary({
      pending_permission_count: pendingPermissions.length,
      process_content: processProjection.content,
    }),
    [pendingPermissions.length, processProjection.content],
  );

  const liveActivityState = useMemo(
    () => resolve_live_activity_state({
      is_last_round: isLastRound,
      is_loading: isLoading,
      merged_content: mergedContent,
      pending_permissions: pendingPermissions,
      runtime_phase: runtimePhase,
      stream_status: streamStatus,
      streaming_block_indexes: streamingBlockIndexes,
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

  const { content_area_ref: contentAreaRef, content_area_style: contentAreaStyle } =
    useMessageItemStreamingLayout({
      assistant_content_mode: assistantContentMode,
      direct_content: directOrderedProjection.content,
      final_assistant_text: finalAssistantText,
      show_cursor: showCursor,
    });

  return {
    copied_user: copiedUser,
    copied_assistant: copiedAssistant,
    user_message: userMessage,
    user_content: userContent,
    user_attachments: userAttachments,
    assistant_agent_id: assistantAgentId,
    model,
    timestamp,
    stream_status: streamStatus,
    stats,
    matched_pending_permissions_by_tool_use_id: matchedPendingPermissionsByToolUseId,
    unmatched_pending_permissions: unmatchedPendingPermissions,
    direct_ordered_projection: directOrderedProjection,
    process_projection: processProjection,
    final_assistant_content: finalAssistantContent,
    final_assistant_streaming_indexes: finalAssistantStreamingIndexes,
    final_assistant_text: finalAssistantText,
    should_render_direct_assistant_content: shouldRenderDirectAssistantContent,
    should_render_process_callchain: shouldRenderProcessCallchain,
    should_render_assistant_text: shouldRenderAssistantText,
    should_render_standalone_activity_status: shouldRenderStandaloneActivityStatus,
    should_show_assistant_footer: shouldShowAssistantFooter,
    show_cursor: showCursor,
    final_assistant_is_streaming: finalAssistantIsStreaming,
    should_hide_assistant_content: shouldHideAssistantContent,
    process_summary: processSummary,
    live_activity_state: liveActivityState,
    is_process_expanded: isProcessExpanded,
    toggle_process_expanded: toggleProcessExpanded,
    process_anchor_ref: processAnchorRef,
    can_copy_assistant: canCopyAssistant,
    can_stop_message: canStopMessage,
    handle_copy_user: handleCopyUser,
    handle_copy_assistant: handleCopyAssistant,
    handle_stop_message: handleStopMessage,
    content_area_ref: contentAreaRef,
    content_area_style: contentAreaStyle,
    merged_content_length: mergedContent.length,
  };
}
